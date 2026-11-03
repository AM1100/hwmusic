//go:build linux

package media

import (
	"github.com/godbus/dbus/v5"
	"os"
	"testing"
	"time"
)

func TestIndependentFrontendNames(t *testing.T) {
	if os.Getenv("HWMUSIC_TEST_MPRIS") == "" {
		t.Skip("isolated session bus required")
	}
	desktop := New(func(Event) {})
	terminal := NewNamed(func(Event) {}, "org.mpris.MediaPlayer2.hwmusic", "hwmusic TUI", "hwmusic")
	if desktop == nil || terminal == nil {
		if desktop != nil {
			desktop.Close()
		}
		if terminal != nil {
			terminal.Close()
		}
		t.Fatal("independent frontend registration failed")
	}
	defer desktop.Close()
	defer terminal.Close()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for name, want := range map[string]string{busName: "hwmusic", "org.mpris.MediaPlayer2.hwmusic": "hwmusic TUI"} {
		value, err := conn.Object(name, objectPath).GetProperty("org.mpris.MediaPlayer2.Identity")
		if err != nil || value.Value() != want {
			t.Fatal("frontend identity mismatch")
		}
	}
}

func TestMPRISWithIsolatedBus(t *testing.T) {
	if os.Getenv("HWMUSIC_TEST_MPRIS") == "" {
		t.Skip("run with dbus-run-session and HWMUSIC_TEST_MPRIS=1")
	}
	events := make(chan Event, 16)
	c := New(func(e Event) { events <- e })
	if c == nil {
		t.Fatal("MPRIS registration failed")
	}
	defer c.Close()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	object := conn.Object(busName, objectPath)
	c.Update("synthetic", "fixture", 100, 12, true)
	v, err := object.GetProperty(playerInterface + ".PlaybackStatus")
	if err != nil || v.Value() != "Playing" {
		t.Fatal("playing state missing")
	}
	if err = object.Call(playerInterface+".Seek", 0, int64(3e6)).Err; err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e.Command != 5 || e.Position != 15 {
			t.Fatal("seek mapping incorrect")
		}
	case <-time.After(time.Second):
		t.Fatal("seek timed out")
	}
	c.Preferences(.6, true, "all")
	v, err = object.GetProperty(playerInterface + ".LoopStatus")
	if err != nil || v.Value() != "Playlist" {
		t.Fatal("loop state missing")
	}
	err = object.Call("org.freedesktop.DBus.Properties.Set", 0, playerInterface, "Volume", dbus.MakeVariant(.25)).Err
	if err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e.Command != 8 || e.Position != .25 {
			t.Fatal("volume mapping incorrect")
		}
	case <-time.After(time.Second):
		t.Fatal("volume timed out")
	}
	c.Close()
	if object.Call(playerInterface+".Play", 0).Err == nil {
		t.Fatal("closed bus name still callable")
	}
}
