package main

import (
	"context"
	"fmt"
	"time"

	"github.com/vivaui/viva/ui"
	"hwmusic/native/internal/huawei"
	"hwmusic/native/internal/icons"
	"hwmusic/native/internal/library"
	"hwmusic/native/internal/visual"
)

var albumRows = tree.Sub(shell, tree.Name("album-links"))
var artistRows = tree.Sub(shell, tree.Name("artist-links"))
var entityAlbums = tree.Sub(shell, tree.Name("artist-albums"))
var entityAlbumsPrevious = tree.ID(shell, "artist-albums-previous")
var entityAlbumsNext = tree.ID(shell, "artist-albums-next")

type entityResult struct {
	generation uint64
	detail     huawei.Detail
	err        error
}

func (m *model) openEntity(kind, id, title string) {
	if id == "" {
		return
	}
	m.entityGeneration++
	gen := m.entityGeneration
	m.entity = &huawei.Detail{ID: id, Title: title}
	m.entityKind = kind
	m.entityScroll = 0
	m.entityAlbumPage = 0
	if m.demo {
		m.entity.Tracks = syntheticTracks(min(m.demoCount, 20))
		return
	}
	m.status = "正在读取详情…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client := huawei.NewClient()
		r := entityResult{generation: gen}
		if kind == "artist" {
			r.detail, r.err = client.ArtistDetail(ctx, id)
		} else {
			catalog := library.New(func(ctx context.Context, start, limit int) (huawei.Page, error) {
				d, err := client.AlbumDetail(ctx, id, start, limit)
				if start == 0 {
					r.detail = d
				}
				return huawei.Page{Tracks: d.Tracks, More: d.More, Next: d.Next, RawCount: d.Next - start}, err
			})
			r.err = catalog.LoadAll(ctx)
			r.detail.Tracks, _ = catalog.Snapshot()
		}
		m.mu.Lock()
		if !m.closed {
			m.entityResults = append(m.entityResults, r)
		}
		m.mu.Unlock()
		m.requestUpdate()
	}()
}
func (m *model) applyEntities() {
	m.mu.Lock()
	results := m.entityResults
	m.entityResults = nil
	m.mu.Unlock()
	for _, r := range results {
		if r.generation != m.entityGeneration || m.entity == nil {
			continue
		}
		if r.err != nil {
			if m.expireSession(r.err) {
				continue
			}
			m.status = "详情读取失败，请返回后重试"
			continue
		}
		m.entity = &r.detail
		m.status = "详情已更新"
	}
}
func (m *model) paintEntity(c *ui.Canvas, b ui.Rect) {
	if m.pill(c, backID, rect(b.X+b.Width-90, b.Y-40, 90, 30), "返回", false) {
		m.entity = nil
		m.entityGeneration++
		return
	}
	side := float32(150)
	if b.Width < 800 {
		side = 112
	}
	extra := float32(0)
	if len(m.entity.Albums) > 0 {
		extra = 44
	}
	b.Height = min(b.Height, side+53+float32(len(m.entity.Tracks))*58+extra)

	kind := "专辑"
	if m.entityKind == "artist" {
		kind = "歌手"
	}
	meta := fmt.Sprintf("%d 首", len(m.entity.Tracks))
	if m.entityKind == "album" && len(m.entity.Tracks) > 0 {
		meta = m.entity.Tracks[0].ArtistText() + " · " + meta
	}
	listY := m.detailHero(c, b, kind, m.entity.Title, m.entity.CoverURL, meta, m.entity.Tracks)
	if len(m.entity.Albums) > 0 {
		columns := max(1, int((b.Width-86)/180))
		start := min(m.entityAlbumPage*columns, len(m.entity.Albums)-1)
		for i := start; i < min(start+columns, len(m.entity.Albums)); i++ {
			a := m.entity.Albums[i]
			if m.pill(c, entityAlbums.At(i), rect(b.X+21+float32(i-start)*180, listY, 168, 32), a.Title, false) {
				m.openEntity("album", a.ID, a.Title)
				return
			}
		}
		if start > 0 && m.pill(c, entityAlbumsPrevious, rect(b.X+b.Width-80, listY, 36, 32), "<", false) {
			m.entityAlbumPage--
		}
		if start+columns < len(m.entity.Albums) && m.pill(c, entityAlbumsNext, rect(b.X+b.Width-38, listY, 36, 32), ">", false) {
			m.entityAlbumPage++
		}
		listY += 44
	}
	m.paintTrackList(c, rect(b.X+21, listY, b.Width-42, max(0, b.Y+b.Height-listY-20)), m.entity.Tracks, true, true)
}

func (m *model) detailHero(c *ui.Canvas, b ui.Rect, kind, title, cover, meta string, tracks []huawei.Track) float32 {
	side, gap := float32(150), float32(24)
	if b.Width < 800 {
		side = 112
		gap = 18
	}
	art := rect(b.X+23, b.Y+15, side, side)
	c.Shadow(art, 16, ui.ShadowStyle{OffsetY: 16, Blur: 36}, color(0x101216, .14))
	visual.RoundedGradient(c, art, 16, color(0xead9d6, 1), color(0xcbb8b3, 1))
	icon := icons.ListMusic
	if kind == "专辑" {
		icon = icons.Disc3
	}
	c.DrawIcon(icon, rect(art.X+side/2-17, art.Y+side/2-17, 34, 34), color(0xffffff, .92))
	m.cover(c, cover, art)
	x := art.X + side + gap
	w := b.X + b.Width - 23 - x
	y := art.Y + side - 126
	m.text(c, kind, rect(x, y, w, 16), 10, 700, m.muted())
	m.text(c, title, rect(x, y+23, w, 32), 26, 680, m.ink())
	m.text(c, meta, rect(x, y+62, w, 18), 11, 400, m.muted())
	play := rect(x, y+90, 98, 32)
	c.Shadow(play, 16, ui.ShadowStyle{OffsetY: 6, Blur: 14}, color(0xc8202e, .2))
	c.Rounded(play, 16, m.accent())
	c.DrawIcon(icons.PlayFilled, rect(x+16, y+100, 12, 12), color(0xffffff, 1))
	m.text(c, "播放全部", rect(x+35, y+90, 58, 32), 12, 600, color(0xffffff, 1))
	if hit(c, allID, play, "播放全部") {
		m.startTracks(tracks, 0)
	}
	return art.Y + side + 18
}
