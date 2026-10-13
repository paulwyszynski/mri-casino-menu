package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func menuServer(t *testing.T) *httptest.Server {
	t.Helper()
	page, err := os.ReadFile("../../internal/menu/testdata/speiseplan-2026-10-12.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(page)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDryRunDaily(t *testing.T) {
	srv := menuServer(t)
	t.Setenv("MENU_URL", srv.URL)
	t.Setenv("STATE_DIR", t.TempDir())
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	for _, args := range [][]string{
		{"--dry-run", "--now", "2026-10-12T10:00", "daily"},
		{"daily", "--dry-run", "--now", "2026-10-12T10:00"},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Fatalf("run(%v) = %d, stderr: %s", args, code, errb.String())
		}
		if !strings.Contains(out.String(), "Speiseplan Montag, 12.10.") {
			t.Errorf("run(%v) stdout = %q", args, out.String())
		}
	}
}

func TestDryRunWeeklyDoesNotWriteState(t *testing.T) {
	srv := menuServer(t)
	dir := t.TempDir()
	t.Setenv("MENU_URL", srv.URL)
	t.Setenv("STATE_DIR", dir)

	var out, errb bytes.Buffer
	if code := run([]string{"--dry-run", "--now", "2026-10-09T15:00", "weekly"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Speiseplan nächste Woche (12.–16.10.)") {
		t.Errorf("stdout = %q", out.String())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("dry-run wrote state: %v", entries)
	}
}

func TestUsageAndConfigErrors(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "usage"},
		{"unknown command", []string{"lunch"}, "unknown command"},
		{"notify-failure without unit", []string{"--dry-run", "notify-failure"}, "usage"},
		{"bad --now", []string{"--dry-run", "--now", "tomorrow", "daily"}, "--now"},
		{"missing token", []string{"daily"}, "TELEGRAM_BOT_TOKEN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(tt.args, &out, &errb); code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			if !strings.Contains(errb.String(), tt.want) {
				t.Errorf("stderr = %q, want it to mention %q", errb.String(), tt.want)
			}
		})
	}
}

func TestJobFailureExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()
	t.Setenv("MENU_URL", srv.URL)
	t.Setenv("STATE_DIR", t.TempDir())

	var out, errb bytes.Buffer
	if code := run([]string{"--dry-run", "--now", "2026-10-12T10:00", "daily"}, &out, &errb); code != 1 {
		t.Errorf("exit = %d, want 1; stderr: %s", code, errb.String())
	}
}
