package main

import (
	"fmt"
	"time"

	"github.com/vivaui/viva/ui"
	"hwmusic/native/internal/icons"
	"hwmusic/native/internal/visual"
)

var traySettingID = tree.ID(shell, "close-to-tray")
var historySettingID = tree.ID(shell, "sync-history")
var queueRemove = tree.Sub(shell, tree.Name("remove-from-queue"))
var clearQueueID = tree.ID(shell, "clear-queue")
var customAccentID = tree.ID(shell, "custom-accent")
var refreshCollectionsID = tree.ID(shell, "refresh-collections")
var thirdPartyLyricsID = tree.ID(shell, "third-party-lyrics")
var lyricsMenuID = tree.ID(shell, "lyrics-menu")
var dockFavoriteID = tree.ID(shell, "dock-favorite")
var dockCoverID = tree.ID(shell, "dock-cover")

func (m *model) paintCollections(c *ui.Canvas, b ui.Rect) {
	if m.detail != nil {
		if m.pill(c, backID, rect(b.X+b.Width-100, b.Y-40, 100, 30), "返回歌单", false) {
			m.detail = nil
			return
		}
		side := float32(150)
		if b.Width < 800 {
			side = 112
		}
		extra := float32(0)
		if m.detail.Kind == "owned" {
			extra = 44
		}
		b.Height = min(b.Height, side+53+float32(len(m.detailTracks))*58+extra)

		kind := map[string]string{"owned": "自建歌单", "collected": "收藏歌单", "chart": "榜单"}[m.detail.Kind]
		listY := m.detailHero(c, b, kind, m.detail.Title, m.detail.CoverURL, fmt.Sprintf("%d 首", len(m.detailTracks)), m.detailTracks)
		if m.pill(c, refreshCollectionsID, rect(b.X+b.Width-110, listY-50, 90, 32), "刷新详情", false) {
			m.loadCollections("detail", true)
		}
		if m.detail.Kind == "owned" {
			m.text(c, m.detail.Description, rect(b.X+23, listY, b.Width-265, 32), 12, 400, m.secondary())
			if m.pill(c, editPlaylistID, rect(b.X+b.Width-240, listY, 100, 32), "修改简介", false) {
				m.dialog = "description"
				m.dialogID = m.detail.ID
				m.writeField.SetText(m.detail.Description)
			}
			if m.pill(c, deletePlaylistID, rect(b.X+b.Width-130, listY, 100, 32), "删除歌单", false) {
				m.dialog = "delete"
				m.dialogID = m.detail.ID
			}
			listY += 44
		}
		m.paintTrackList(c, rect(b.X+21, listY, b.Width-42, max(0, b.Y+b.Height-listY-20)), m.detailTracks, true, true)
		return
	}
	refreshLabel := "刷新歌单"
	if m.collectionFailed {
		refreshLabel = "重试更新"
	}
	if m.pill(c, refreshCollectionsID, rect(b.X+b.Width-90, b.Y, 90, 32), refreshLabel, false) {
		m.loadCollections("playlists", true)
	}
	if len(m.playlists) == 0 {
		label := "暂无歌单。"
		if m.collectionLoading {
			label = "正在读取歌单…"
		} else if m.session == nil && !m.demo {
			label = "请先登录。"
		}
		m.text(c, label, rect(b.X+20, b.Y+30, b.Width-40, 28), 14, 400, m.muted())
	}
	c.Clip(rect(b.X, b.Y+40, b.Width, b.Height-40))
	side, rowHeight := collectionCardSize(b.Width)
	for i, p := range m.playlists {
		x := b.X + float32(i%3)*(b.Width/3)
		y := b.Y + 48 + float32(i/3)*rowHeight - m.scroll[1]
		if y+rowHeight-12 < b.Y+40 {
			continue
		}
		if y > b.Y+b.Height {
			break
		}
		r := rect(x, y, side, rowHeight-12)
		c.Rounded(rect(x, y, side, side), 16, color(0xcbb8b3, .65))
		c.DrawIcon(icons.ListMusic, rect(x+r.Width/2-20, y+side/2-20, 40, 40), color(0xffffff, .9))
		m.cover(c, p.CoverURL, rect(x, y, side, side))
		m.text(c, p.Title, rect(x, y+side+12, r.Width, 24), 15, 600, m.ink())
		kind := map[string]string{"owned": "自建歌单", "collected": "收藏歌单", "chart": "榜单"}[p.Kind]
		m.text(c, kind, rect(x, y+side+38, r.Width, 20), 11, 400, m.muted())
		if hit(c, playlistRows.At(i), r, p.Title) {
			value := p
			m.detail = &value
			m.detailScroll = 0
			m.detailTracks = nil
			m.loadCollections("detail")
		}
	}
	c.Unclip()
}

func (m *model) paintHistory(c *ui.Canvas, b ui.Rect) {
	label := "查看华为历史"
	if m.remoteHistoryOpen {
		label = "查看本机记录"
	}
	if m.pill(c, historySourceID, rect(b.X, b.Y, 140, 34), label, false) {
		m.remoteHistoryOpen = !m.remoteHistoryOpen
		if m.remoteHistoryOpen {
			m.loadCollections("history")
		}
	}
	tracks := m.localHistory
	if m.remoteHistoryOpen {
		tracks = m.remoteHistory
	}
	m.paintLibrary(c, rect(b.X, b.Y+50, b.Width, b.Height-50), tracks, true)
}

func (m *model) paintDock(c *ui.Canvas, size ui.Size) {
	m.paintDockSurface(c, size, m.stageAmount(time.Now()))
	m.paintTransport(c, size, m.stageAmount(time.Now()))
}

func (m *model) dockButton(c *ui.Canvas, id ui.ID, b ui.Rect, icon ui.Icon, label string, active, enabled bool) bool {
	if enabled {
		return m.iconButton(c, id, b, icon, label, active)
	}
	c.DrawIcon(icons.RoundedStroke(icon), rect(b.X+7, b.Y+7, 20, 20), color(m.palette().muted, .4))
	c.HitWithOptions(id, b, label, ui.HitOptions{Disabled: true})
	return false
}

func (m *model) paintDrawer(c *ui.Canvas, size ui.Size) {
	c.PushLayer(1)
	defer c.PopLayer()
	b := rect(size.Width-378, 155, 338, size.Height-280)
	c.Block(b)
	m.glass(c, b, 16, .88)
	c.PushLayer(2)
	defer c.PopLayer()
	title := "播放队列"

	m.text(c, title, rect(b.X+20, b.Y+18, 240, 30), 18, 650, m.ink())
	if m.iconButton(c, closePanelID, rect(b.X+b.Width-46, b.Y+16, 30, 30), icons.X, "关闭面板", false) {
		m.queueOpen = false
		m.lyricsMenuOpen = false
	}
	if m.queueOpen && m.pill(c, clearQueueID, rect(b.X+165, b.Y+18, 90, 30), "清空队列", false) {
		m.cancelPlayback()
		m.queue.Replace(nil, 0)
		m.audioState.Position = 0
		m.audioState.Duration = 0
		m.saveQueue()
	}
	c.Clip(rect(b.X+12, b.Y+64, b.Width-24, b.Height-76))
	if m.queueOpen {
		for i, t := range m.queue.Tracks {
			y := b.Y + 66 + float32(i)*48 - m.queueScroll
			if y+48 < b.Y+64 {
				continue
			}
			if y > b.Y+b.Height {
				break
			}
			r := rect(b.X+12, y, b.Width-24, 46)
			if i == m.queue.Index {
				tint := m.accent()
				tint.Alpha = .09
				c.Rounded(r, 8, tint)
			}
			m.text(c, t.Title, rect(r.X+10, y+5, r.Width-60, 22), 13, 500, m.ink())
			m.text(c, t.ArtistText(), rect(r.X+10, y+27, r.Width-60, 18), 11, 400, m.muted())
			if hit(c, queueRows.At(i), r, "队列曲目 "+t.Title) {
				m.queue.Select(i)
				m.playCurrent()
				m.saveQueue()
			}
			if m.iconButton(c, queueRemove.At(i), rect(r.X+r.Width-36, y+8, 30, 30), icons.X, "移除 "+t.Title, false) {
				current := m.queue.Remove(i)
				if current {
					m.cancelPlayback()
					if m.queue.Current() != nil {
						m.playCurrent()
					}
				}
				m.saveQueue()
				break
			}
		}
	}
	c.Unclip()
}

func collectionCardSize(width float32) (float32, float32) {
	side := min(float32(240), width/3-18)
	return side, side + 78
}

func (m *model) paintDockSurface(c *ui.Canvas, size ui.Size, t float32) {
	b := rect(24, size.Height-114, size.Width-48, 98)
	if t < 1 {
		fade := 1 - t
		shadow := float32(.14)
		if m.palette().dark {
			shadow = .30
		}
		c.Shadow(b, 24, ui.ShadowStyle{OffsetY: 6, Blur: 24}, color(0, shadow*fade))
		c.Shadow(b, 24, ui.ShadowStyle{OffsetY: 1, Blur: 4}, color(0, .09*fade))
		visual.Border(c, b, 24, 1, m.line(.09*fade), m.surface(.98*fade))
	}
}
