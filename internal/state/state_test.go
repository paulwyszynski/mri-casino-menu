package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	s := New(t.TempDir(), false)
	want := State{WeeklySent: "2026-W42", FailureNotified: "2026-10-16"}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := os.Stat(s.Path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind: %v", err)
	}
}

func TestOverwriteKeepsSingleFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, false)
	for _, w := range []string{"2026-W42", "2026-W43", "2026-W44"} {
		if err := s.Save(State{WeeklySent: w}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Errorf("dir entries = %v, want only state.json", entries)
	}
	got, _ := s.Load()
	if got.WeeklySent != "2026-W44" {
		t.Errorf("WeeklySent = %q", got.WeeklySent)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := New(t.TempDir(), false).Load()
	if err != nil || got != (State{}) {
		t.Errorf("Load() = %+v, %v; want empty, nil", got, err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := New(dir, false).Load()
	if err == nil {
		t.Error("want error for corrupt file")
	}
	if got != (State{}) {
		t.Errorf("got %+v, want empty", got)
	}
}

func TestReadOnlyDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	if err := New(dir, true).Save(State{WeeklySent: "2026-W42"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(err) {
		t.Errorf("read-only store wrote a file: %v", err)
	}
}

func TestSaveIntoMissingDirFails(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "does-not-exist"), false)
	if err := s.Save(State{WeeklySent: "2026-W42"}); err == nil {
		t.Error("want error")
	}
}
