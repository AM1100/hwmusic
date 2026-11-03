//go:build (linux || windows || portableaudio) && cgo

package audio

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func awaitPortable(t *testing.T, p *Player, condition func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		state := p.Snapshot()
		if state.Failed {
			t.Fatal("native decoding failed")
		}
		if condition(state) {
			return state
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("native playback timed out: %+v", p.Snapshot())
	return State{}
}

func TestPortableLocalPlayback(t *testing.T) {
	dir := os.Getenv("HWMUSIC_AUDIO_FIXTURES")
	if dir == "" {
		t.Skip("set HWMUSIC_AUDIO_FIXTURES to a synthetic tone.mp3/tone.flac directory")
	}
	for _, name := range []string{"tone.mp3", "tone.flac"} {
		t.Run(name, func(t *testing.T) {
			p, err := OpenLocalFixture(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			ready := awaitPortable(t, p, func(s State) bool { return s.Ready && s.Duration > 3 })
			if ready.Playing || ready.Position > .1 {
				t.Fatal("started automatically")
			}
			if err := p.SetVolume(0); err != nil {
				t.Fatal(err)
			}
			p.Play()
			awaitPortable(t, p, func(s State) bool { return s.Playing && s.Position > .2 })
			p.Pause()
			paused := awaitPortable(t, p, func(s State) bool { return !s.Playing })
			time.Sleep(100 * time.Millisecond)
			if math.Abs(p.Snapshot().Position-paused.Position) > .08 {
				t.Fatal("pause did not stop position")
			}
			if err := p.Seek(ready.Duration - .4); err != nil {
				t.Fatal(err)
			}
			awaitPortable(t, p, func(s State) bool { return s.Position >= ready.Duration-.5 })
			p.Play()
			awaitPortable(t, p, func(s State) bool { return s.Ended })
			p.Close()
			p.Close()
			if p.Snapshot().Playing {
				t.Fatal("closed player still playing")
			}
		})
	}
}
