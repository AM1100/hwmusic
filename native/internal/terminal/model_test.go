package terminal

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"hwmusic/native/internal/audio"
	"hwmusic/native/internal/huawei"
)

type fakeSound struct {
	state  audio.State
	closed int
	volume float64
}

func (p *fakeSound) Play()                     { p.state.Playing = true }
func (p *fakeSound) Pause()                    { p.state.Playing = false }
func (p *fakeSound) Seek(v float64) error      { p.state.Position = v; return nil }
func (p *fakeSound) SetVolume(v float64) error { p.volume = v; return nil }
func (p *fakeSound) Snapshot() audio.State     { return p.state }
func (p *fakeSound) Close()                    { p.closed++ }
func demoModel(t *testing.T) *model {
	t.Helper()
	b := newBackend(t.TempDir(), "", true)
	t.Cleanup(b.close)
	return newModel(b)
}
func key(m *model, s string) tea.Cmd { return m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}) }
func TestStaleOpenDoesNotPlayOrReplaceCurrent(t *testing.T) {
	m := demoModel(t)
	m.queue.Replace(demoTracks(), 0)
	_ = m.playCurrent()
	old := m.generation
	m.queue.Select(1)
	_ = m.playCurrent()
	stale := &fakeSound{}
	m.b.keep(stale)
	m.Update(openedMsg{generation: old, sound: stale})
	if stale.closed != 1 || m.sound != nil {
		t.Fatal("stale audio retained")
	}
	current := &fakeSound{state: audio.State{Ready: true, Duration: 4}}
	m.b.keep(current)
	m.Update(openedMsg{generation: m.generation, sound: current})
	if m.sound != current || !current.state.Playing || current.volume != 0 {
		t.Fatal("current synthetic audio not adopted muted")
	}
	m.b.close()
	if current.closed != 1 {
		t.Fatal("audio was not closed exactly once")
	}
}
func TestNoLateSoundAfterShutdown(t *testing.T) {
	m := demoModel(t)
	m.b.close()
	p := &fakeSound{}
	if m.b.keep(p) || p.closed != 1 {
		t.Fatal("sound leaked after program shutdown")
	}
}
func TestPaginationDiscardsOldViewAndStopsRepeatedIDs(t *testing.T) {
	m := demoModel(t)
	_ = m.switchView("search", "搜索")
	old := m.revision
	_ = m.switchView("history", "历史")
	m.Update(pageMsg{revision: old, page: huawei.Page{Tracks: demoTracks()}})
	if len(m.tracks) != 0 || !m.loading {
		t.Fatal("stale page changed new view")
	}
	m.Update(pageMsg{revision: m.revision, page: huawei.Page{Tracks: demoTracks(), More: true, Next: 2}})
	m.Update(pageMsg{revision: m.revision, start: 2, page: huawei.Page{Tracks: demoTracks(), More: true, Next: 4}})
	if len(m.tracks) != 2 || m.more {
		t.Fatal("duplicate pages not stopped")
	}
}
func TestQueueKeysDoNotReplaceExistingSongs(t *testing.T) {
	m := demoModel(t)
	_ = key(m, "a")
	m.cursor = 1
	_ = key(m, "a")
	if len(m.queue.Tracks) != 2 || m.queue.Index != 0 {
		t.Fatal("enqueue changed current track")
	}
	_ = m.key(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.queue.Tracks) != 2 || m.queue.Index != 1 {
		t.Fatal("selection duplicated queue")
	}
	_ = key(m, "m")
	if m.queue.Mode != "all" {
		t.Fatal("loop not applied")
	}
	_ = key(m, "s")
	if !m.queue.Shuffle {
		t.Fatal("shuffle not applied")
	}
}
func TestSearchSupportsChineseAndSpaceWithoutPlaybackKeys(t *testing.T) {
	m := demoModel(t)
	_ = key(m, "/")
	_ = key(m, "周杰伦")
	_ = key(m, " ")
	_ = key(m, "晴天")
	cmd := m.key(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.query != "周杰伦 晴天" || m.view != "search" || len(m.queue.Tracks) != 0 {
		t.Fatalf("bad search: %q", m.query)
	}
}
func TestEndOfQueueIsHandledOnceAndNextTrackLoads(t *testing.T) {
	m := demoModel(t)
	m.queue.Replace(demoTracks(), 1)
	p := &fakeSound{state: audio.State{Ready: true, Duration: 4, Position: 4, Ended: true}}
	m.b.keep(p)
	m.sound = p
	m.Update(tickMsg(time.Now()))
	if !m.endHandled {
		t.Fatal("end not handled")
	}
	m.status = "unchanged"
	m.Update(tickMsg(time.Now()))
	if m.status != "unchanged" {
		t.Fatal("finished playback repeatedly handled")
	}
	m.queue.Select(0)
	m.endHandled = false
	m.Update(tickMsg(time.Now()))
	if m.queue.Index != 1 || !m.opening || p.closed != 1 {
		t.Fatal("automatic next track failed")
	}
}
func TestDemoDoesNotReadProfileOrLogin(t *testing.T) {
	dir := t.TempDir() + "/untouched"
	b := newBackend(dir, "", true)
	defer b.close()
	m := newModel(b)
	m.save()
	if m.login() != nil {
		t.Fatal("demo launched authorization")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("demo touched profile")
	}
}
func TestViewFitsTerminalAndNeutralizesMetadata(t *testing.T) {
	m := demoModel(t)
	m.tracks[0].Title = "恶意\x1b]52;c;clipboard\x07\n标题"
	for _, size := range [][2]int{{42, 16}, {80, 24}, {120, 35}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, variant := range []string{"list", "lyrics", "help", "search"} {
			m.lyricView = variant == "lyrics"
			m.help = variant == "help"
			m.editing = variant == "search"
			v := m.View()
			if strings.Contains(v, "\x1b]52") || strings.Count(v, "\n") >= size[1] {
				t.Fatal("unsafe or overflowing view")
			}
			for _, row := range strings.Split(v, "\n") {
				if ansi.StringWidth(row) > size[0] {
					t.Fatalf("%s overflows %d: %q", variant, size[0], row)
				}
			}
		}
	}
}
func TestCanceledRestoreDoesNotOverwriteNewLogin(t *testing.T) {
	m := demoModel(t)
	m.authSerial = 1
	m.status = "new login"
	m.handleSession(sessionMsg{err: context.Canceled, serial: 0})
	if m.status != "new login" {
		t.Fatal("old restore changed new login")
	}
}

func TestWholeCollectionCancellationAndSearchIsolation(t *testing.T) {
	m := demoModel(t)
	cmd := m.playCollection()
	if cmd == nil {
		t.Fatal("missing collection command")
	}
	result := cmd()
	m.key(tea.KeyMsg{Type: tea.KeyEsc})
	m.Update(result)
	if len(m.queue.Tracks) != 0 {
		t.Fatal("cancelled collection replaced queue")
	}
	cmd = m.playCollection()
	m.Update(cmd())
	if len(m.queue.Tracks) != 2 || !m.opening {
		t.Fatal("complete collection not queued")
	}
	m.view = "search"
	if m.playCollection() != nil {
		t.Fatal("search unexpectedly offers play all")
	}
}
