package terminal

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"hwmusic/native/internal/lyrics"
)

var (
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selected = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("236"))
)

// Music metadata is untrusted terminal input. Never emit provider control codes.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, s)
}
func clip(s string, width int) string { return ansi.Truncate(clean(s), max(0, width), "…") }
func duration(seconds float64) string {
	if seconds < 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
		seconds = 0
	}
	return fmt.Sprintf("%02d:%02d", int(seconds)/60, int(seconds)%60)
}
func (m *model) View() string {
	if m.width < 42 || m.height < 16 {
		return "hwmusic\n请将终端扩大至至少 42 列 × 16 行。\n按 q 或 Ctrl+C 退出。"
	}
	w := m.width - 4
	badge := "未登录"
	if m.session != nil {
		badge = "已登录"
	}
	if m.b.demo != "" {
		badge = "离线演示"
	}
	rows := []string{accent.Render(clip("hwmusic  /  HUAWEI MUSIC  ·  "+badge, w)), "",
		muted.Render(clip("1 收藏   2 歌单   3 历史   4 队列   / 搜索   L 登录   ? 帮助", w)), "",
		accent.Render(clip(m.title, w)), ""}
	bodyHeight := m.height - 15
	if m.help {
		help := []string{"键盘操作", "↑/↓ 或 j/k   选择歌曲     Enter 立即播放", "a 加入队尾              4 查看队列", "P 播放完整收藏/歌单     空格 暂停/继续", "n/p 下一首/上一首       ←/→ 或 h/l 跳转 5 秒", "+/- 音量                m 顺序/列表循环/单曲循环", "s 随机                  t 标准/无损（下首生效）", "v 歌词                  / 搜索（支持中文）", "J 加载下一页            r 刷新当前列表", "L 扫码登录              U 退出账号", "Esc 取消登录/整表播放   ? 或 Esc 关闭帮助", "q 或 Ctrl+C 保存队列并退出"}
		for i := 0; i < bodyHeight; i++ {
			line := ""
			if i < len(help) {
				line = help[i]
			}
			rows = append(rows, clip(line, w))
		}
	} else if m.lyricView {
		index := lyrics.Active(m.lines, m.state.Position)
		start := max(0, index-bodyHeight/2)
		lyricRows := []string{}
		if len(m.lines) == 0 {
			lyricRows = append(lyricRows, muted.Render("暂无歌词 · 按 v 返回列表"))
		}
		for i := start; i < len(m.lines) && len(lyricRows) < bodyHeight; i++ {
			text := clip(m.lines[i].Text, w-4)
			if i == index {
				text = accent.Render("▸ " + text)
			} else {
				text = muted.Render("  " + text)
			}
			lyricRows = append(lyricRows, text)
			for _, subtitle := range m.lines[i].Subtitles() {
				if len(lyricRows) < bodyHeight {
					lyricRows = append(lyricRows, muted.Render("  "+clip(subtitle, w-4)))
				}
			}
		}
		for len(lyricRows) < bodyHeight {
			lyricRows = append(lyricRows, "")
		}
		rows = append(rows, lyricRows...)
	} else {
		total := m.count()
		start := max(0, m.cursor-bodyHeight+1)
		for row := 0; row < bodyHeight; row++ {
			i := start + row
			line := ""
			if i < total {
				text := ""
				if m.view == "playlists" {
					text = m.playlists[i].Title
				} else {
					t := m.listTracks()[i]
					text = t.Title + "  /  " + t.ArtistText()
					if playing := m.queue.Current(); playing != nil && playing.ID == t.ID {
						text = "♫ " + text
					}
				}
				line = fmt.Sprintf("%3d  %s", i+1, clip(text, w-7))
				if i == m.cursor {
					line = selected.Render(lipgloss.NewStyle().Width(w).Render(line))
				}
			} else if total == 0 && row == 1 {
				text := "暂无内容 · 按 L 登录，或 / 搜索歌曲"
				if m.loading {
					text = "加载中…"
				}
				if m.view == "queue" {
					text = "队列是空的 · 在列表中回车播放，或 a 加入队尾"
				}
				line = muted.Render(clip(text, w))
			}
			rows = append(rows, line)
		}
	}
	info := fmt.Sprintf("已载入 %d 项", m.count())
	if m.view == "search" {
		info = "搜索结果"
	}
	if m.more {
		info += " · 还有下一页 ↓ / J"
	}
	if m.view == "queue" {
		info = fmt.Sprintf("队列 %d 首", len(m.queue.Tracks))
	}
	rows = append(rows, muted.Render(clip(info, w)), muted.Render(strings.Repeat("─", w)))
	title := "尚未选择歌曲"
	if t := m.queue.Current(); t != nil {
		title = t.Title + "  /  " + t.ArtistText()
	}
	symbol := "Ⅱ"
	if m.state.Playing {
		symbol = "▶"
	}
	if m.opening {
		symbol = "…"
	}
	rows = append(rows, accent.Render(symbol+" ")+clip(title, w-3))
	progress := 0.0
	if m.state.Duration > 0 {
		progress = max(0, min(1, m.state.Position/m.state.Duration))
	}
	barWidth := max(3, w-17)
	fill := int(progress * float64(barWidth))
	rows = append(rows, duration(m.state.Position)+" "+accent.Render(strings.Repeat("━", fill))+muted.Render(strings.Repeat("─", barWidth-fill))+" "+duration(m.state.Duration))
	mode := map[string]string{"off": "顺序", "all": "列表循环", "one": "单曲循环"}[m.settings.RepeatMode]
	if m.settings.Shuffle {
		mode = "随机 · " + mode
	}
	quality := "标准"
	if m.settings.Quality == "lossless" {
		quality = "无损"
	}
	rows = append(rows, muted.Render(clip(fmt.Sprintf("音量 %.0f%% · %s · %s · 空格 播放/暂停 · v 歌词", m.settings.Volume*100, mode, quality), w)))
	status := m.status
	if m.editing {
		status = "搜索 > " + string(m.input) + "▏  [Enter 搜索 / Esc 取消]"
	}
	rows = append(rows, clip(status, w))
	if !m.editing && !m.lyricView {
		if i := lyrics.Active(m.lines, m.state.Position); i >= 0 {
			rows = append(rows, muted.Render(clip("♪ "+m.lines[i].Text, w)))
		} else {
			rows = append(rows, "")
		}
	}
	return "  " + strings.Join(rows, "\n  ")
}
