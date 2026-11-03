package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"hwmusic/native/internal/audio"
	"hwmusic/native/internal/huawei"
	"hwmusic/native/internal/library"
	"hwmusic/native/internal/lyrics"
	"hwmusic/native/internal/media"
	"hwmusic/native/internal/player"
	"hwmusic/native/internal/storage"
)

type tickMsg time.Time
type sessionMsg struct {
	session *huawei.Session
	err     error
	login   bool
	serial  int
}
type pageMsg struct {
	revision  int
	page      huawei.Page
	playlists []huawei.Playlist
	err       error
	start     int
}
type openedMsg struct {
	generation int
	sound      sound
	err        error
	playback   huawei.Playback
}
type lyricMsg struct {
	generation int
	lines      []lyrics.Line
}
type queueMsg struct {
	serial int
	tracks []huawei.Track
	err    error
}
type model struct {
	b                        *backend
	width, height            int
	view, title, query       string
	input                    []rune
	editing, help, lyricView bool
	tracks                   []huawei.Track
	playlists                []huawei.Playlist
	playlist                 huawei.Playlist
	cursor, next, revision   int
	more, loading            bool
	browseCancel             context.CancelFunc
	session                  *huawei.Session
	loginCancel              context.CancelFunc
	authSerial               int
	status                   string
	settings                 storage.Settings
	queue                    player.Queue
	account                  *storage.Store
	sound                    sound
	state                    audio.State
	lines                    []lyrics.Line
	generation               int
	playCancel               context.CancelFunc
	playCtx                  context.Context
	opening                  bool
	endHandled               bool
	resume                   float64
	media                    *media.Controller
	queueSerial              int
	queueCancel              context.CancelFunc
}

func newModel(b *backend) *model {
	settings := storage.Defaults()
	if b.demo != "" {
		settings.Volume = 0
	} // Synthetic audio always starts muted.
	m := &model{b: b, width: 100, height: 30, view: "favorites", title: "我的收藏", settings: settings, status: "按 L 扫码登录 · / 搜索歌曲 · ? 查看按键"}
	if b.demo != "" {
		m.tracks = demoTracks()
		m.status = "离线演示 · 合成数据 · 不读取账号，不请求音乐服务"
	}
	return m
}
func tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m *model) Init() tea.Cmd {
	if m.b.demo != "" {
		return tick()
	}
	b := m.b
	return tea.Batch(tick(), func() tea.Msg {
		s, err := b.store.LoadSession()
		if err == nil {
			err = b.client.ValidateSession(b.ctx, s)
		}
		return sessionMsg{session: s, err: err}
	})
}
func (m *model) fail(err error) {
	if huawei.SessionExpired(err) {
		m.queueSerial++
		if m.queueCancel != nil {
			m.queueCancel()
			m.queueCancel = nil
		}
		m.stopPlayback()
		m.session = nil
		m.account = nil
		m.queue.Replace(nil, 0)
		m.tracks = nil
		m.playlists = nil
		_ = m.b.store.DeleteSession()
		m.status = "登录已过期，按 L 重新扫码"
		return
	}
	if errors.Is(err, context.Canceled) {
		m.status = "操作已取消"
		return
	}
	m.status = "操作失败：" + err.Error() + " · 可按 r 重试 / L 登录"
}
func (m *model) save() {
	if m.b.store == nil {
		return
	}
	if err := m.b.store.SaveSettings(m.settings); err != nil {
		m.status = "设置保存失败，本次进程内仍有效"
	}
	if m.account != nil {
		position := m.state.Position
		if m.sound == nil {
			position = m.resume
		}
		if err := m.account.SaveQueue(storage.SavedQueue{Tracks: m.queue.Tracks, Index: m.queue.Index, Position: position}); err != nil {
			m.status = "播放队列保存失败"
		}
	}
}
func (m *model) stopPlayback() {
	m.generation++
	if m.playCancel != nil {
		m.playCancel()
		m.playCancel = nil
	}
	m.b.release(m.sound)
	m.sound = nil
	m.opening = false
	m.endHandled = false
	m.lines = nil
	m.state = audio.State{}
}

// Collection playback scans every page and deduplicates IDs before replacing
// the queue. Searching continues to enqueue only the explicitly chosen song.
func (m *model) playCollection() tea.Cmd {
	if m.view != "favorites" && m.view != "playlist" {
		m.status = "整表播放仅用于收藏和歌单；搜索结果回车加入队列"
		return nil
	}
	if m.b.demo == "" && m.session == nil {
		m.status = "请按 L 登录"
		return nil
	}
	if m.queueCancel != nil {
		m.queueCancel()
	}
	m.queueSerial++
	serial := m.queueSerial
	ctx, cancel := context.WithCancel(m.b.ctx)
	m.queueCancel = cancel
	b, s, v, p := m.b, m.session, m.view, m.playlist
	m.status = "正在读取完整收藏/歌单，完成后开始播放 · Esc 取消"
	return func() tea.Msg {
		if b.demo != "" {
			return queueMsg{serial, demoTracks(), nil}
		}
		catalog := library.New(func(ctx context.Context, start, limit int) (huawei.Page, error) {
			if v == "playlist" {
				return b.client.PlaylistDetail(ctx, s, p, start, limit)
			}
			return b.client.Favorites(ctx, s, start, limit)
		})
		err := catalog.LoadAll(ctx)
		tracks, _ := catalog.Snapshot()
		return queueMsg{serial, tracks, err}
	}
}
func (m *model) playCurrent() tea.Cmd {
	m.stopPlayback()
	t := m.queue.Current()
	if t == nil {
		return nil
	}
	if m.b.demo == "" && m.session == nil {
		m.status = "请按 L 登录后播放"
		return nil
	}
	m.opening = true
	m.status = "正在准备音频…"
	ctx, cancel := context.WithCancel(m.b.ctx)
	m.playCancel = cancel
	m.playCtx = ctx
	b, s, track, q, g := m.b, m.session, *t, m.settings.Quality, m.generation
	return func() tea.Msg {
		p, resolved, err := b.open(ctx, s, track, q)
		return openedMsg{generation: g, sound: p, err: err, playback: resolved}
	}
}
func (m *model) step(delta int, ended bool) tea.Cmd {
	if m.queue.Step(delta, ended) {
		m.resume = 0
		return m.playCurrent()
	}
	m.status = "已到播放队列末尾 · 回车加入歌曲 / m 切换循环"
	return nil
}
func (m *model) toggle() tea.Cmd {
	if m.opening {
		return nil
	}
	if m.sound == nil {
		return m.playCurrent()
	}
	if m.sound.Snapshot().Playing {
		m.sound.Pause()
	} else {
		m.sound.Play()
	}
	m.state = m.sound.Snapshot()
	m.save()
	return nil
}
func (m *model) load(reset bool) tea.Cmd {
	if m.view == "queue" {
		return nil
	}
	if m.loading && !reset {
		return nil
	}
	if !reset && !m.more {
		return nil
	}
	if m.view != "search" && m.b.demo == "" && m.session == nil {
		m.status = "请按 L 扫码登录"
		return nil
	}
	if reset {
		if m.browseCancel != nil {
			m.browseCancel()
		}
		m.revision++
		m.next = 0
		m.cursor = 0
		m.tracks = nil
		m.playlists = nil
		m.more = false
	}
	ctx, cancel := context.WithCancel(m.b.ctx)
	m.browseCancel = cancel
	b, s, v, q, p, start, rev := m.b, m.session, m.view, m.query, m.playlist, m.next, m.revision
	m.loading = true
	m.status = "正在加载…"
	return func() tea.Msg {
		r := pageMsg{revision: rev, start: start}
		if b.demo != "" {
			if v == "playlists" {
				r.playlists = []huawei.Playlist{{ID: "demo-list", Title: "离线测试歌单", Kind: "owned"}}
			} else {
				r.page.Tracks = demoTracks()
			}
			return r
		}
		switch v {
		case "favorites":
			r.page, r.err = b.client.Favorites(ctx, s, start, 50)
		case "search":
			r.page, r.err = b.client.Search(ctx, q, start, 30)
		case "playlists":
			r.playlists, r.err = b.client.Playlists(ctx, s)
		case "playlist":
			r.page, r.err = b.client.PlaylistDetail(ctx, s, p, start, 50)
		case "history":
			r.page.Tracks, r.err = b.client.History(ctx, s)
		}
		return r
	}
}
func (m *model) switchView(view, title string) tea.Cmd {
	m.view, m.title = view, title
	m.lyricView = false
	if view == "queue" {
		if m.browseCancel != nil {
			m.browseCancel()
		}
		m.revision++
		m.loading = false
		m.cursor = max(0, m.queue.Index)
		return nil
	}
	return m.load(true)
}
func (m *model) listTracks() []huawei.Track {
	if m.view == "queue" {
		return m.queue.Tracks
	}
	return m.tracks
}
func (m *model) count() int {
	if m.view == "playlists" {
		return len(m.playlists)
	}
	return len(m.listTracks())
}
func (m *model) login() tea.Cmd {
	if m.b.demo != "" {
		m.status = "离线演示不登录账号，请退出后正常启动"
		return nil
	}
	if m.loginCancel != nil {
		m.status = "登录窗口已打开，Esc 取消"
		return nil
	}
	ctx, cancel := context.WithTimeout(m.b.ctx, 10*time.Minute)
	m.loginCancel = cancel
	m.authSerial++
	serial := m.authSerial
	m.status = "请在浏览器中扫码，完成后会自动返回 · Esc 取消"
	b := m.b
	return func() tea.Msg { s, err := b.login(ctx); return sessionMsg{s, err, true, serial} }
}
func (m *model) handleSession(r sessionMsg) tea.Cmd {
	if r.serial != m.authSerial {
		return nil
	}
	if r.login {
		if m.loginCancel != nil {
			m.loginCancel()
			m.loginCancel = nil
		}
	}
	if r.err != nil {
		if !r.login && errors.Is(r.err, os.ErrNotExist) {
			return nil
		}
		m.fail(r.err)
		return nil
	}
	m.stopPlayback()
	m.queue.Replace(nil, 0)
	m.queueSerial++
	if m.queueCancel != nil {
		m.queueCancel()
		m.queueCancel = nil
	}
	m.resume = 0
	m.session = r.session
	if settings, err := m.b.store.LoadSettings(); err == nil {
		m.settings = settings
	}
	m.queue.Mode = m.settings.RepeatMode
	m.queue.SetShuffle(m.settings.Shuffle)
	m.account = m.b.store.Account(r.session.AccountKey())
	if q, err := m.account.LoadQueue(); err == nil {
		m.queue.Replace(q.Tracks, q.Index)
		m.resume = q.Position
	}
	if r.login {
		if err := m.b.store.SaveSession(r.session); err != nil {
			m.status = "登录成功，但无法保存会话，退出后需重新登录"
		}
	}
	return m.switchView("favorites", "我的收藏")
}
func (m *model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case sessionMsg:
		return m, m.handleSession(msg)
	case queueMsg:
		if msg.serial != m.queueSerial {
			return m, nil
		}
		if m.queueCancel != nil {
			m.queueCancel()
			m.queueCancel = nil
		}
		if msg.err != nil {
			m.fail(msg.err)
			return m, nil
		}
		if len(msg.tracks) == 0 {
			m.status = "列表里没有可播放歌曲"
			return m, nil
		}
		m.queue.Replace(msg.tracks, 0)
		m.resume = 0
		return m, m.playCurrent()
	case pageMsg:
		if msg.revision != m.revision {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.fail(msg.err)
			return m, nil
		}
		if msg.playlists != nil {
			m.playlists = msg.playlists
			m.status = "回车浏览歌单"
			return m, nil
		}
		seen := make(map[string]bool)
		for _, t := range m.tracks {
			seen[t.ID] = true
		}
		before := len(m.tracks)
		for _, t := range msg.page.Tracks {
			if t.ID != "" && !seen[t.ID] {
				m.tracks = append(m.tracks, t)
				seen[t.ID] = true
			}
		}
		m.more = msg.page.More
		if m.more && (msg.page.Next <= msg.start || len(m.tracks) == before) {
			m.more = false
			m.status = "分页没有新内容，已停止；r 重新加载"
			return m, nil
		}
		m.next = msg.page.Next
		m.status = "回车播放 · j/k 选择 · J 加载下一页"
	case openedMsg:
		if msg.generation != m.generation {
			m.b.release(msg.sound)
			return m, nil
		}
		m.opening = false
		if msg.err != nil {
			m.fail(msg.err)
			return m, nil
		}
		m.sound = msg.sound
		_ = m.sound.SetVolume(m.settings.Volume)
		if m.resume > 0 {
			_ = m.sound.Seek(m.resume)
			m.resume = 0
		}
		m.sound.Play()
		m.state = m.sound.Snapshot()
		m.status = "正在播放"
		m.save()
		b, g, ctx, resolved := m.b, m.generation, m.playCtx, msg.playback
		return m, func() tea.Msg { return lyricMsg{g, b.lyric(ctx, resolved)} }
	case lyricMsg:
		if msg.generation == m.generation {
			m.lines = msg.lines
		}
	case tickMsg:
		var cmd tea.Cmd
		if m.sound != nil {
			m.state = m.sound.Snapshot()
			if m.state.Failed {
				m.status = "音频解码失败 · 回车重新选择歌曲"
				m.stopPlayback()
			} else if m.state.Ended && !m.endHandled {
				m.endHandled = true
				m.save()
				cmd = m.step(1, true)
			}
		}
		if m.media != nil {
			if t := m.queue.Current(); t != nil {
				m.media.Update(t.Title, t.ArtistText(), m.state.Duration, m.state.Position, m.state.Playing)
			}
			m.media.Preferences(m.settings.Volume, m.settings.Shuffle, m.settings.RepeatMode)
		}
		return m, tea.Batch(tick(), cmd)
	case media.Event:
		switch msg.Command {
		case 0:
			return m, m.toggle()
		case 1:
			if !m.state.Playing {
				return m, m.toggle()
			}
		case 2:
			if m.state.Playing {
				return m, m.toggle()
			}
		case 3:
			return m, m.step(-1, false)
		case 4:
			return m, m.step(1, false)
		case 5:
			if m.sound != nil {
				_ = m.sound.Seek(msg.Position)
			}
		case 7:
			m.save()
			return m, tea.Quit
		case 8:
			m.settings.Volume = msg.Position
			if m.sound != nil {
				_ = m.sound.SetVolume(msg.Position)
			}
			m.save()
		case 9:
			m.settings.Shuffle = msg.Position != 0
			m.queue.SetShuffle(m.settings.Shuffle)
			m.save()
		case 10:
			if msg.Position >= 0 && msg.Position <= 2 {
				m.settings.RepeatMode = []string{"off", "all", "one"}[int(msg.Position)]
				m.queue.Mode = m.settings.RepeatMode
				m.save()
			}
		}
	case tea.KeyMsg:
		return m, m.key(msg)
	}
	return m, nil
}
func (m *model) key(msg tea.KeyMsg) tea.Cmd {
	m.cursor = max(0, min(m.cursor, m.count()-1))
	key := msg.String()
	if key == "ctrl+c" {
		m.save()
		return tea.Quit
	}
	if m.editing {
		switch key {
		case "esc":
			m.editing = false
			m.input = nil
		case "enter":
			m.editing = false
			m.query = strings.TrimSpace(string(m.input))
			if m.query != "" {
				return m.switchView("search", "搜索 · "+m.query)
			}
		case "backspace", "ctrl+h":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case "ctrl+u":
			m.input = nil
		default:
			if msg.Type == tea.KeyRunes && len(m.input) < 200 {
				m.input = append(m.input, msg.Runes...)
			}
			if key == " " && msg.Type != tea.KeyRunes && len(m.input) < 200 {
				m.input = append(m.input, ' ')
			}
		}
		return nil
	}
	if m.help {
		if key == "?" || key == "esc" {
			m.help = false
		}
		return nil
	}
	switch key {
	case "q":
		m.save()
		return tea.Quit
	case "?":
		m.help = true
	case "esc":
		if m.queueCancel != nil {
			m.queueCancel()
			m.queueCancel = nil
			m.queueSerial++
			m.status = "已取消整表播放"
		} else if m.loginCancel != nil {
			m.loginCancel()
			m.status = "已取消登录"
		} else if m.lyricView {
			m.lyricView = false
		} else {
			return m.switchView("favorites", "我的收藏")
		}
	case "L":
		return m.login()
	case "1":
		return m.switchView("favorites", "我的收藏")
	case "2":
		return m.switchView("playlists", "我的歌单")
	case "3":
		return m.switchView("history", "播放历史")
	case "4":
		return m.switchView("queue", "播放队列")
	case "/":
		m.editing = true
		m.input = []rune(m.query)
	case "r":
		return m.load(true)
	case "J":
		return m.load(false)
	case "j", "down":
		m.cursor = min(m.cursor+1, max(0, m.count()-1))
		if m.cursor >= m.count()-3 && m.more {
			return m.load(false)
		}
	case "k", "up":
		m.cursor = max(0, m.cursor-1)
	case "pgdown":
		m.cursor = min(m.cursor+max(1, m.height-13), max(0, m.count()-1))
		if m.more {
			return m.load(false)
		}
	case "pgup":
		m.cursor = max(0, m.cursor-max(1, m.height-13))
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = max(0, m.count()-1)
	case "enter":
		if m.count() == 0 {
			return nil
		}
		if m.view == "playlists" {
			m.playlist = m.playlists[m.cursor]
			return m.switchView("playlist", m.playlist.Title)
		}
		m.queueSerial++
		if m.queueCancel != nil {
			m.queueCancel()
			m.queueCancel = nil
		}
		if m.view == "queue" {
			m.queue.Select(m.cursor)
		} else if m.view == "favorites" || m.view == "playlist" {
			m.queue.Replace(m.tracks, m.cursor)
		} else {
			m.queue.Enqueue(m.tracks[m.cursor])
		}
		m.resume = 0
		return m.playCurrent()
	case "a":
		if m.view != "playlists" && m.count() > 0 {
			t := m.listTracks()[m.cursor]
			for _, existing := range m.queue.Tracks {
				if existing.ID == t.ID {
					m.status = "歌曲已在队列中"
					return nil
				}
			}
			current, pos := m.queue.Index, m.queue.Position
			m.queue.Replace(append(append([]huawei.Track(nil), m.queue.Tracks...), t), current)
			m.queue.Position = pos
			m.status = "已加入队尾"
			m.save()
		}
	case " ":
		return m.toggle()
	case "n":
		m.resume = 0
		return m.step(1, false)
	case "p":
		m.resume = 0
		return m.step(-1, false)
	case "h", "left":
		if m.sound != nil {
			_ = m.sound.Seek(max(0, m.state.Position-5))
		}
	case "l", "right":
		if m.sound != nil {
			_ = m.sound.Seek(min(m.state.Duration, m.state.Position+5))
		}
	case "+", "=", "-":
		delta := .05
		if key == "-" {
			delta = -delta
		}
		m.settings.Volume = max(0, min(1, m.settings.Volume+delta))
		if m.sound != nil {
			_ = m.sound.SetVolume(m.settings.Volume)
		}
		m.save()
	case "m":
		m.settings.Shuffle = false
		m.queue.SetShuffle(false)
		m.settings.RepeatMode = map[string]string{"off": "all", "all": "one", "one": "off"}[m.settings.RepeatMode]
		m.queue.Mode = m.settings.RepeatMode
		m.save()
	case "s":
		m.settings.Shuffle = !m.settings.Shuffle
		m.queue.SetShuffle(m.settings.Shuffle)
		m.save()
	case "t":
		if m.settings.Quality == "standard" {
			m.settings.Quality = "lossless"
		} else {
			m.settings.Quality = "standard"
		}
		m.status = "音质已切换，下首生效"
		m.save()
	case "v":
		m.lyricView = !m.lyricView
	case "P":
		return m.playCollection()
	case "U":
		if m.b.demo != "" {
			return nil
		}
		m.save()
		m.stopPlayback()
		m.session = nil
		m.account = nil
		m.queue.Replace(nil, 0)
		m.resume = 0
		m.authSerial++
		if m.loginCancel != nil {
			m.loginCancel()
			m.loginCancel = nil
		}
		m.queueSerial++
		if m.queueCancel != nil {
			m.queueCancel()
			m.queueCancel = nil
		}
		m.revision++
		if m.browseCancel != nil {
			m.browseCancel()
		}
		m.loading = false
		m.tracks = nil
		m.playlists = nil
		m.more = false
		if err := m.b.store.DeleteSession(); err != nil {
			m.status = "已退出，删除保存的会话失败"
		} else {
			m.status = "已退出账号，按 L 重新登录"
		}
	}
	return nil
}
func runProgram(b *backend) error {
	m := newModel(b)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(b.ctx))
	if b.demo == "" {
		m.media = newMedia(func(event media.Event) { p.Send(event) })
		if m.media != nil {
			defer m.media.Close()
		}
	}
	_, err := p.Run()
	m.save()
	if err != nil {
		return fmt.Errorf("终端播放器退出：%w", err)
	}
	return nil
}
