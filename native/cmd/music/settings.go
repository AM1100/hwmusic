package main

import (
	"fmt"

	"github.com/vivaui/viva/ui"
	"hwmusic/native/internal/storage"
	"hwmusic/native/internal/typography"
	"hwmusic/native/internal/visual"
)

var settingOptions = tree.Sub(shell, tree.Name("setting-options"))
var clearLocalLyricsID = tree.ID(shell, "clear-local-lyrics")

func (m *model) settingsHeight() float32 {
	if m.settingsTab == 0 {
		return 710
	}
	if m.settingsTab == 1 {
		return 640
	}
	return 300
}

func (m *model) settingSliderBounds(size ui.Size, id ui.ID) ui.Rect {
	pad := float32(30)
	if size.Width <= 1100 {
		pad = 22
	}
	x := sidebarWidth(size.Width) + 20 + pad + 1 + 209
	y := float32(178.8+20+16) - m.settingsScroll

	return rect(x+148, y, max(60, size.Width-10-pad-9-23-x-148), 24)
}

func (m *model) settingToggle(c *ui.Canvas, id ui.ID, b ui.Rect, label string, value *bool) {
	m.text(c, label, rect(b.X+2, b.Y+15, b.Width-60, 24), 12, 400, m.ink())
	r := rect(b.X+b.Width-40, b.Y+18, 36, 21)
	tint := color(0x16181c, .16)
	if *value {
		tint = m.accent()
	}
	c.Rounded(r, 10.5, tint)
	dx := float32(3)
	if *value {
		dx += 15
	}
	c.Shadow(rect(r.X+dx, r.Y+3, 15, 15), 7.5, ui.ShadowStyle{OffsetY: 1, Blur: 3}, color(0, .24))
	c.Rounded(rect(r.X+dx, r.Y+3, 15, 15), 7.5, color(0xffffff, 1))
	if hit(c, id, b, label) {
		*value = !*value
		m.saveSettings()
	}
}

func (m *model) settingChoice(c *ui.Canvas, id ui.ID, b ui.Rect, key, label string, options []string, selected int, change func(int)) {
	m.text(c, label, rect(b.X+2, b.Y+17, b.Width-210, 24), 12, 400, m.ink())
	w := float32(132)
	if key == "mode" {
		w = 188
	}
	r := rect(b.X+b.Width-w-2, b.Y+13, w, 33)
	visual.Border(c, r, 8, 1, m.line(.085), m.surface(1))
	m.text(c, options[selected], rect(r.X+12, r.Y+5, w-38, 23), 12, 400, m.ink())
	m.text(c, "⌄", rect(r.X+w-24, r.Y+4, 15, 23), 15, 500, m.secondary())
	if hit(c, id, r, label) {
		if m.settingsSelect == key {
			m.settingsSelect = ""
		} else {
			m.settingsSelect = key
		}
	}
	if m.settingsSelect != key {
		return
	}
	c.PushLayer(2)
	defer c.PopLayer()
	popup := rect(r.X, r.Y+r.Height+3, w, float32(len(options))*34+8)
	c.Block(popup)
	m.panel(c, popup, 8, 1)
	for i, option := range options {
		item := rect(popup.X+4, popup.Y+4+float32(i)*34, w-8, 34)
		if i == selected {
			tint := m.accent()
			tint.Alpha = .08
			c.Rounded(item, 5, tint)
		}
		m.text(c, option, rect(item.X+8, item.Y+5, item.Width-16, 24), 12, 400, m.ink())
		if hit(c, settingOptions.At(i), item, option) {
			change(i)
			m.settingsSelect = ""
			m.saveSettings()
		}
	}
}

func (m *model) paintSettings(c *ui.Canvas, b ui.Rect) {
	c.Clip(b)
	defer c.Unclip()

	for i, label := range []string{"播放", "外观", "关于"} {
		r := rect(b.X+23, b.Y+21+float32(i)*38, 145, 36)
		weight := float32(500)
		if m.settingsTab == i {
			c.Shadow(r, 11, ui.ShadowStyle{OffsetY: 4, Blur: 12}, color(0x101216, .07))
			c.Rounded(r, 11, m.surface(1))
			weight = 650
		}
		m.text(c, label, rect(r.X+12, r.Y+7, 120, 22), 13, weight, m.secondary())
		if hit(c, tabs.At(i), r, label+"设置") {
			m.settingsTab = i
			m.settingsScroll = 0
			m.settingsSelect = ""
		}
	}
	x, y, w := b.X+207, b.Y+20-m.settingsScroll, b.Width-230
	switch m.settingsTab {
	case 0:
		closeChoice := 0
		if m.settings.CloseToTray {
			closeChoice = 1
		}
		m.settingChoice(c, traySettingID, rect(x, y, w, 60), "close", "关闭窗口时", []string{"退出程序", "最小化到托盘"}, closeChoice, func(i int) { m.settings.CloseToTray = i == 1 })
		quality := 0
		if m.settings.Quality == "lossless" {
			quality = 1
		}
		m.settingChoice(c, settingsQualityID, rect(x, y+62, w, 60), "quality", "默认音质", []string{"标准", "无损"}, quality, func(i int) { m.settings.Quality = []string{"standard", "lossless"}[i] })
		mode := map[string]int{"off": 0, "all": 1, "one": 2}[m.settings.RepeatMode]
		if m.settings.Shuffle {
			mode = 3
		}
		m.settingChoice(c, settingsRepeatID, rect(x, y+124, w, 60), "mode", "播放模式", []string{"顺序播放", "列表循环", "单曲循环", "随机播放（打乱队列顺序）"}, mode, func(i int) {
			m.settings.Shuffle = i == 3
			m.queue.SetShuffle(i == 3)
			m.settings.RepeatMode = []string{"off", "all", "one", "off"}[i]
		})
		m.settingToggle(c, historySettingID, rect(x, y+186, w, 54), "同步播放到华为历史", &m.settings.SyncHistory)
		m.settingHint(c, rect(x, y+242, w, 60), "开启后，实际听到一定时长才会上报；快速跳过的曲目不记入华为历史，", "不会上报未播放的秒数。尚未用真实账号验证，失败不影响播放。")
		m.settingToggle(c, thirdPartyLyricsID, rect(x, y+304, w, 54), "第三方歌词匹配", &m.settings.ThirdPartyLyrics)
		m.settingHint(c, rect(x, y+360, w, 60), "华为接口没有返回歌词时，按歌名、艺人、时长匹配酷狗音乐和 LRCLIB。", "只请求歌词文本，不涉及音源，不上传账号信息。")
		m.text(c, "歌词缓存", rect(x+2, y+432, w-4, 22), 12, 400, m.ink())
		stats := m.lyricStats
		m.text(c, fmt.Sprintf("网络 %d 条 · 本地 %d 条 · %.1f KB", stats.Network, stats.Local, float64(stats.Bytes)/1024), rect(x+2, y+460, w-4, 22), 11, 400, m.muted())
		if m.pill(c, clearLyricsID, rect(x, y+494, 120, 32), "清除网络缓存", false) && m.store != nil {
			if err := m.store.ClearLyrics(false); err != nil {
				m.status = "缓存清理失败"
			} else {
				m.status = "网络歌词缓存已清除"
			}
			m.lyricStats = m.store.LyricsInfo()
		}
		if m.pill(c, clearLocalLyricsID, rect(x+132, y+494, 120, 32), "清除本地导入", false) {
			m.dialog = "clear-local-lyrics"
		}
		if m.pill(c, importLyricsID, rect(x, y+544, 120, 32), "导入本地歌词", false) {
			m.importLyrics()
		}
		if m.pill(c, removeLyricsID, rect(x+132, y+544, 120, 32), "移除本曲导入", false) {
			m.removeLyrics()
		}
	case 1:
		for i, item := range []struct {
			id               ui.ID
			label            string
			value, low, high float32
		}{{fontScaleID, "字体大小", float32(m.settings.Appearance.FontScale), .85, 1.2}} {
			ry := y + 16 + float32(i)*53
			m.text(c, item.label, rect(x+2, ry, 128, 24), 12, 400, m.ink())
			value := fmt.Sprintf("%.0f%%", item.value*100)
			if item.id == radiusID {
				value = fmt.Sprintf("%.0fpx", item.value)
			}
			m.text(c, value, rect(x+102, ry, 36, 24), 11, 400, m.muted())
			slider := rect(x+150, ry, w-150, 24)
			c.Rounded(rect(slider.X, ry+11, slider.Width, 3), 1.5, m.line(.14))
			c.Rounded(rect(slider.X, ry+11, slider.Width*(item.value-item.low)/(item.high-item.low), 3), 1.5, m.accent())
			c.HitWithOptions(item.id, slider, item.label, ui.HitOptions{Role: ui.RoleSlider})
			if c.Hovered() == item.id || c.FocusVisible() == item.id {
				c.Rounded(rect(slider.X+slider.Width*(item.value-item.low)/(item.high-item.low)-4, ry+8, 8, 8), 4, m.accent())
			}
		}
		m.text(c, "当前字体", rect(x+2, y+86, 100, 24), 12, 400, m.ink())
		family := m.settings.Appearance.FontFamily
		if family == "" {
			family = typography.DefaultFamily()
		}
		m.text(c, family, rect(x+112, y+86, w-248, 24), 12, 400, m.secondary())
		if m.pill(c, fontChoiceID, rect(x+w-126, y+82, 126, 32), "选择系统字体", false) {
			m.fontFamilies = typography.Families()
			m.fontFilter.SetText("")
			m.fontFilterText = ""
			m.fontScroll = 0
			m.fontIndex = 0
			m.fontListOpen = true
		}

		m.text(c, "主题", rect(x, y+146, w, 24), 12, 500, m.ink())
		for i, name := range themeNames {
			r := rect(x+float32(i%3)*(w/3), y+182+float32(i/3)*44, w/3-8, 34)
			if m.pill(c, themes.At(i), r, themeLabels[i], m.settings.Appearance.Theme == name || (name == "light" && m.settings.Appearance.Theme == "")) {
				m.settings.Appearance.Theme = name
				size := m.backgroundSize
				m.backgroundSize = ui.Size{}
				m.prepareBackground(size)
				m.saveSettings()
			}
		}
		if m.pill(c, resetID, rect(x, y+376, 104, 34), "恢复默认外观", false) {
			m.settings.Appearance = storage.Defaults().Appearance
			size := m.backgroundSize
			m.backgroundSize = ui.Size{}
			m.prepareBackground(size)
			m.saveSettings()
		}

	case 2:
		m.text(c, appDisplayName, rect(x+2, y+16, w-4, 30), 15, 600, m.ink())
		m.text(c, "非官方 · AGPL-3.0 · Viva 原生迁移中", rect(x+2, y+68, w-4, 24), 12, 400, m.muted())
		m.text(c, "OAuth 身份不属于本项目。", rect(x+2, y+108, w-4, 24), 12, 400, m.muted())
		if m.session != nil && m.pill(c, logoutID, rect(x, y+158, 100, 34), "退出账号", false) {
			m.logout()
		}
	}
}

func (m *model) settingHint(c *ui.Canvas, b ui.Rect, first, second string) {
	m.text(c, first, rect(b.X+2, b.Y+12, b.Width-4, 18), 11, 400, m.muted())
	m.text(c, second, rect(b.X+2, b.Y+30, b.Width-4, 18), 11, 400, m.muted())
}
