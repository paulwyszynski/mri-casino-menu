// Package state persists the little the bot must remember between runs.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// State holds only the latest value of each field.
type State struct {
	WeeklySent      string `json:"weekly_sent,omitempty"`
	FailureNotified string `json:"failure_notified,omitempty"`
}

// Store reads and writes State as JSON at Path.
type Store struct {
	Path     string
	ReadOnly bool
}

// New returns a store for dir/state.json. A read-only store never writes
// (used for --dry-run).
func New(dir string, readOnly bool) *Store {
	return &Store{Path: filepath.Join(dir, "state.json"), ReadOnly: readOnly}
}

// Load returns the stored state. A missing file is an empty state. A file
// that cannot be read or decoded also yields an empty state, together with
// an error the caller should log and otherwise ignore.
func (s *Store) Load() (State, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("state: read %s: %w", s.Path, err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("state: decode %s: %w", s.Path, err)
	}
	return st, nil
}

// Save writes st atomically: temp file, fsync, rename.
func (s *Store) Save(st State) error {
	if s.ReadOnly {
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("state: encode: %w", err)
	}
	tmp := s.Path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("state: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("state: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("state: close: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("state: rename: %w", err)
	}
	return nil
}
