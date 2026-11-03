package main

import (
	"fmt"
	"github.com/vivaui/viva/ui"
	"hwmusic/native/internal/icons"
)

var transportOptions = tree.Sub(shell, tree.Name("transport-options"))

func volumePopoverBounds(size ui.Size) ui.Rect {
	return rect(size.Width-238, size.Height-172, 192, 84)
}
func volumeTrackBounds(size ui.Size) ui.Rect {
	b := volumePopoverBounds(size)
	return rect(b.X+18, b.Y+48, b.Width-36, 24)
}

func (m *model) paintVolumeMenu(c *ui.Canvas, size ui.Size) {
	b := volumePopoverBounds(size)
	c.PushLayer(4)
	defer c.PopLayer()
	c.Block(b)
	c.Shadow(b, 14, ui.ShadowStyle{OffsetY: 4, Blur: 20}, color(0, .15))
	c.Rounded(b, 14, m.surface(1))
	m.text(c, "音量", rect(b.X+18, b.Y+12, 80, 24), 12, 500, m.ink())
	m.centerText(c, fmt.Sprintf("%.0f%%", m.settings.Volume*100), rect(b.X+126, b.Y+12, 50, 24), 11, 500, m.secondary())
	r := volumeTrackBounds(size)
	x := r.X + r.Width*float32(m.settings.Volume)
	c.Rounded(rect(r.X, r.Y+10, r.Width, 4), 2, m.line(.15))
	c.Rounded(rect(r.X, r.Y+10, x-r.X, 4), 2, m.accent())
	c.Rounded(rect(x-6, r.Y+6, 12, 12), 6, m.accent())
	c.HitWithOptions(volumeSliderID, r, "音量", ui.HitOptions{Role: ui.RoleSlider})
}

func (m *model) toggleTransportMenu(kind string) {
	if m.transportMenu == kind {
		m.transportMenu = ""
	} else {
		m.transportMenu = kind
	}
	m.queueOpen = false
	m.lyricsMenuOpen = false
}
func (m *model) modeIcon() ui.Icon {
	if m.settings.Shuffle {
		return icons.Shuffle
	}
	if m.settings.RepeatMode == "one" {
		return icons.RepeatOne
	}
	if m.settings.RepeatMode == "all" {
		return icons.Repeat
	}
	return icons.Ordered
}
func (m *model) paintTransportMenu(c *ui.Canvas, size ui.Size) {
	if m.transportMenu == "" {
		return
	}
	if m.transportMenu == "volume" {
		m.paintVolumeMenu(c, size)
		return
	}
	labels := []string{"顺序播放", "列表循环", "单曲循环", "随机播放"}
	glyphs := []ui.Icon{icons.Ordered, icons.Repeat, icons.RepeatOne, icons.Shuffle}
	selected := 0
	if m.settings.Shuffle {
		selected = 3
	} else if m.settings.RepeatMode == "one" {
		selected = 2
	} else if m.settings.RepeatMode == "all" {
		selected = 1
	}
	if m.transportMenu == "quality" {
		labels = []string{"标准音质", "无损音质"}
		selected = 0
		if m.settings.Quality == "lossless" {
			selected = 1
		}
	}
	b := rect(size.Width-330, size.Height-96-float32(len(labels))*40-16, 192, float32(len(labels))*40+16)
	c.PushLayer(4)
	defer c.PopLayer()
	c.Block(b)
	c.Shadow(b, 14, ui.ShadowStyle{OffsetY: 4, Blur: 20}, color(0, .15))
	c.Rounded(b, 14, m.surface(1))
	for i, label := range labels {
		r := rect(b.X+8, b.Y+8+float32(i)*40, b.Width-16, 36)
		if selected == i {
			tint := m.accent()
			tint.Alpha = .12
			c.Rounded(r, 8, tint)
		}
		x := r.X + 12
		if m.transportMenu == "mode" {
			c.DrawIcon(glyphs[i], rect(x, r.Y+10, 16, 16), m.secondary())
			x += 28
		}
		m.text(c, label, rect(x, r.Y, r.Width-(x-r.X)-12, r.Height), 12, 500, m.ink())
		if hit(c, transportOptions.At(i), r, label) {
			if m.transportMenu == "mode" {
				m.settings.Shuffle = i == 3
				m.settings.RepeatMode = []string{"off", "all", "one", "off"}[i]
				m.queue.SetShuffle(i == 3)
			} else {
				m.settings.Quality = []string{"standard", "lossless"}[i]
			}
			m.saveSettings()
			m.transportMenu = ""
		}
	}
}
