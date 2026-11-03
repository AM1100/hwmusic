package storage

import (
	"hwmusic/native/internal/huawei"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsQueueRestartAndClear(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	settings := Defaults()
	settings.Appearance.Accent = "blue"
	settings.Appearance.CornerRadius = 999
	if err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveQueue(SavedQueue{Tracks: []huawei.Track{{ID: "fixture", Title: "Synthetic", CoverURL: "file:///private"}}, Position: 13}); err != nil {
		t.Fatal(err)
	}
	restarted := New(dir)
	value, err := restarted.LoadSettings()
	if err != nil || value.Appearance.Accent != "blue" || value.Appearance.CornerRadius != 18 {
		t.Fatal("settings restore")
	}
	queue, err := restarted.LoadQueue()
	if err != nil || queue.Position != 13 || queue.Tracks[0].CoverURL != "" {
		t.Fatal("queue restore")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "queue.json"))
	if strings.Contains(string(data), "file:") {
		t.Fatal("unsafe URL persisted")
	}
	if err := s.ClearQueue(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadSettings(); err != nil {
		t.Fatal("logout cleared settings")
	}
}
func TestCorruptProfileIsPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte("broken"), 0600)
	s := New(dir)
	value, err := s.LoadSettings()
	if err == nil || value.Appearance.Accent != "red" {
		t.Fatal("corrupt settings not reported")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "broken" {
		t.Fatal("corrupt document was overwritten during read")
	}
}
