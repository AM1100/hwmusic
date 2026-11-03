package main

import (
	"context"
	"log"
	"math"
	"time"

	"hwmusic/native/internal/audio"
	"hwmusic/native/internal/huawei"
	"hwmusic/native/internal/lyrics"
	"hwmusic/native/internal/resource"
	"hwmusic/native/internal/storage"
)

type playbackResult struct {
	generation  uint64
	player      *audio.Player
	playback    huawei.Playback
	lyrics      []lyrics.Line
	lyricSource string
	err         error
	lyricsOnly  bool
}

func (m *model) resolveCurrent() {
	track := m.queue.Current()
	if track == nil {
		return
	}
	if m.playbackCancel != nil {
		m.playbackCancel()
	}
	m.playbackGeneration++
	generation := m.playbackGeneration
	ctx, cancel := context.WithCancel(context.Background())
	m.playbackCancel = cancel
	m.resolving = true
	id, quality, session := track.ID, m.settings.Quality, m.session
	store := m.store
	fallbackEnabled := m.settings.ThirdPartyLyrics
	query := lyrics.Query{Title: track.Title, Artist: track.ArtistText()}
	if track.Duration != nil {
		query.Duration = *track.Duration
	}
	m.status = "正在获取音频资源…"
	go func() {
		defer cancel()
		r := playbackResult{generation: generation}
		r.playback, r.err = huawei.NewClient().ResolvePlayback(ctx, session, id, quality)
		if r.err == nil {
			r.player, r.err = audio.OpenRemote(ctx, r.playback.URL)
		}
		m.mu.Lock()
		if !m.closed {
			m.playbackResults = append(m.playbackResults, r)
		} else if r.player != nil {
			r.player.Close()
		}
		m.mu.Unlock()
		m.requestUpdate()
		if r.err != nil {
			return
		}
		r.player = nil
		r.lyricsOnly = true
		if r.err == nil {
			text := r.playback.LyricText
			r.lyricSource = "华为音乐"
			cached := false
			if store != nil {
				if doc, err := store.LoadLyrics(id); err == nil {
					text, r.lyricSource, cached = doc.Text, doc.Source, true
				}
			}
			if !cached && text == "" && r.playback.LyricURL != "" {
				if data, err := resource.New().Bytes(ctx, r.playback.LyricURL, 512*1024); err == nil {
					text, _ = lyrics.Decode(data)
				}
			}
			r.lyrics = lyrics.Parse(text)
			if fallbackEnabled && len(r.lyrics) == 0 && ctx.Err() == nil {
				text, r.lyricSource = lyrics.Fallback(ctx, query)
				r.lyrics = lyrics.Parse(text)
			}
			if !cached && len(r.lyrics) > 0 && store != nil {
				_ = store.SaveLyrics(id, storage.LyricDocument{Text: text, Source: r.lyricSource}, false)
			}
		}
		m.mu.Lock()
		if !m.closed {
			m.playbackResults = append(m.playbackResults, r)
		} else if r.player != nil {
			r.player.Close()
		}
		m.mu.Unlock()
		m.requestUpdate()
	}()
}

func (m *model) applyPlaybackResults() {
	m.mu.Lock()
	results := m.playbackResults
	m.playbackResults = nil
	m.mu.Unlock()
	for _, r := range results {
		if r.generation != m.playbackGeneration {
			if r.player != nil {
				r.player.Close()
			}
			continue
		}
		if r.lyricsOnly {
			// A local import made while a provider request was in flight wins.
			if m.lyricSource != "本地导入" {
				m.lyricLines = r.lyrics
				m.lyricSource = r.lyricSource
			}
			m.playbackCancel = nil
			continue
		}
		m.resolving = false
		if r.err != nil {
			if m.expireSession(r.err) {
				continue
			}
			m.status = "播放失败，请重试或选择其他音质。"
			if r.player != nil {
				r.player.Close()
			}
			continue
		}
		m.player = r.player
		m.lyricLines = r.lyrics
		m.lyricSource = r.lyricSource
		m.lyricTrack = ""
		m.actualQuality = r.playback.Quality
		if r.playback.Duration != nil && *r.playback.Duration > 0 {
			m.resolvedDuration = *r.playback.Duration
		}
		_ = m.player.SetVolume(m.settings.Volume)
		m.pendingSeek = m.queue.Position
		m.player.Play()
		m.audioRunning = true
		m.startedAt = time.Now()
		m.status = ""
		m.pollAudio()
	}
}

func (m *model) cancelPlayback() {
	m.playbackGeneration++
	m.resolving = false
	if m.playbackCancel != nil {
		m.playbackCancel()
		m.playbackCancel = nil
	}
	if m.playTimer != nil {
		m.playTimer.Stop()
		m.playTimer = nil
	}
	if m.player != nil {
		m.player.Close()
		m.player = nil
	}
	m.audioRunning = false
}

func (m *model) seekPlayback(seconds float64) {
	if m.player == nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return
	}
	state := m.player.Snapshot()
	if !state.Ready || state.Duration <= 0 {
		return
	}
	seconds = max(0, min(seconds, state.Duration))
	if m.player.Seek(seconds) != nil {
		return
	}
	m.pendingSeek = 0
	m.audioState = state
	m.audioState.Position = seconds
	m.queue.Position = seconds
	m.lastPosition = seconds
	m.lastPoll = time.Now()
	m.updateMedia()
	m.requestUpdate()
}

// Only numeric durations are logged: never track identifiers or signed URLs.
func (m *model) checkDuration() {
	if m.durationMismatch || !m.audioState.Ready || m.audioState.Duration <= 0 {
		return
	}
	catalog := 0.0
	if t := m.queue.Current(); t != nil && t.Duration != nil {
		catalog = *t.Duration
	}
	media := m.audioState.Duration
	if (catalog > 0 && math.Abs(catalog-media) > 2) || (m.resolvedDuration > 0 && math.Abs(m.resolvedDuration-media) > 2) {
		m.durationMismatch = true
		log.Printf("playback duration mismatch: catalog=%.3fs resolved=%.3fs audio=%.3fs; using audio duration", catalog, m.resolvedDuration, media)
	}
}
