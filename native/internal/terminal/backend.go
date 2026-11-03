// Package terminal is the keyboard-driven Huawei Music client. It imports no
// Viva windowing code; playback works without DISPLAY or a desktop compositor.
package terminal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"hwmusic/native/internal/audio"
	"hwmusic/native/internal/huawei"
	"hwmusic/native/internal/lyrics"
	"hwmusic/native/internal/resource"
	"hwmusic/native/internal/storage"
	"hwmusic/native/internal/webauth"
)

type provider interface {
	ValidateSession(context.Context, *huawei.Session) error
	Favorites(context.Context, *huawei.Session, int, int) (huawei.Page, error)
	Search(context.Context, string, int, int) (huawei.Page, error)
	Playlists(context.Context, *huawei.Session) ([]huawei.Playlist, error)
	PlaylistDetail(context.Context, *huawei.Session, huawei.Playlist, int, int) (huawei.Page, error)
	History(context.Context, *huawei.Session) ([]huawei.Track, error)
	ResolvePlayback(context.Context, *huawei.Session, string, string) (huawei.Playback, error)
}
type sound interface {
	Play()
	Pause()
	Seek(float64) error
	SetVolume(float64) error
	Snapshot() audio.State
	Close()
}
type backend struct {
	ctx              context.Context
	cancel           context.CancelFunc
	client           provider
	store            *storage.Store
	demo, fixtureDir string
	mu               sync.Mutex
	closed           bool
	sounds           map[sound]bool
}

func newBackend(profile, fixtureDir string, demo bool) *backend {
	ctx, cancel := context.WithCancel(context.Background())
	b := &backend{ctx: ctx, cancel: cancel, client: huawei.NewClient(), fixtureDir: fixtureDir, sounds: make(map[sound]bool)}
	if demo {
		b.demo = "离线演示"
	} else {
		b.store = storage.New(profile)
	}
	return b
}
func (b *backend) keep(p sound) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		p.Close()
		return false
	}
	b.sounds[p] = true
	return true
}
func (b *backend) release(p sound) {
	if p == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sounds[p] {
		delete(b.sounds, p)
		p.Close()
	}
}
func (b *backend) close() {
	b.cancel()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for p := range b.sounds {
		p.Close()
		delete(b.sounds, p)
	}
}
func (b *backend) login(ctx context.Context) (*huawei.Session, error) {
	attempt := huawei.NewAttempt(time.Now())
	defer attempt.Cancel()
	raw, err := webauth.Authorize(ctx, attempt)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New(webauth.LoginErrorMessage(err))
	}
	if err = attempt.Accept(raw, time.Now()); err != nil {
		return nil, errors.New("扫码授权未完成，请重新登录")
	}
	c := huawei.NewClient()
	s, err := c.Exchange(ctx, attempt)
	if err == nil {
		err = c.ValidateSession(ctx, s)
	}
	return s, err
}
func demoTracks() []huawei.Track {
	d := 4.0
	return []huawei.Track{
		{ID: "demo-mp3", Title: "夜航 · 合成 MP3", Artists: []huawei.Artist{{Name: "hwmusic 离线测试"}}, Duration: &d},
		{ID: "demo-flac", Title: "潮汐 · 合成 FLAC", Artists: []huawei.Artist{{Name: "hwmusic 离线测试"}}, Duration: &d},
	}
}
func (b *backend) open(ctx context.Context, s *huawei.Session, t huawei.Track, quality string) (sound, huawei.Playback, error) {
	var p *audio.Player
	var err error
	var resolved huawei.Playback
	if b.demo != "" {
		if b.fixtureDir == "" {
			return nil, resolved, errors.New("演示播放需指定 --fixture-dir 合成音频目录")
		}
		name := "tone.mp3"
		if t.ID == "demo-flac" {
			name = "tone.flac"
		}
		p, err = audio.OpenLocalFixture(filepath.Join(b.fixtureDir, name))
	} else {
		resolved, err = b.client.ResolvePlayback(ctx, s, t.ID, quality)
		if err == nil {
			p, err = audio.OpenRemote(ctx, resolved.URL)
		}
	}
	if err != nil {
		return nil, resolved, err
	}
	if !b.keep(p) {
		return nil, resolved, context.Canceled
	}
	return p, resolved, nil
}
func (b *backend) lyric(ctx context.Context, resolved huawei.Playback) []lyrics.Line {
	if b.demo != "" {
		return lyrics.Parse("[00:00.00]这是终端里的 hwmusic\n[00:01.00]用方向键选择，用空格暂停\n[00:02.00]让音乐留在终端\n[00:03.00]下一首，继续听")
	}
	text := resolved.LyricText
	if text == "" && resolved.LyricURL != "" {
		if data, err := resource.New().Bytes(ctx, resolved.LyricURL, 512*1024); err == nil {
			text = string(data)
		}
	}
	return lyrics.Parse(text)
}

// Run's demo path never opens the normal profile or initiates provider requests.
func Run(profile, fixtureDir string, demo bool) error {
	if !demo && profile == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		profile = filepath.Join(base, "huawei-music-desktop", "terminal-v1")
	}
	b := newBackend(profile, fixtureDir, demo)
	defer b.close()
	return runProgram(b)
}
