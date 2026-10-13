package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paulwyszynski/mri-casino-menu/internal/menu"
	"github.com/paulwyszynski/mri-casino-menu/internal/state"
)

type fakeFetcher struct {
	body  []byte
	err   error
	calls int
}

func (f *fakeFetcher) Fetch(context.Context) ([]byte, error) {
	f.calls++
	return f.body, f.err
}

type fakeSender struct {
	sent []string
	err  error
}

func (s *fakeSender) Send(_ context.Context, text string) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, text)
	return nil
}

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../menu/testdata/speiseplan-2026-10-12.html")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func at(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	tm, err := time.ParseInLocation("2006-01-02T15:04", s, loc)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func deps(t *testing.T, f *fakeFetcher) (Deps, *fakeSender) {
	t.Helper()
	s := &fakeSender{}
	return Deps{
		Fetcher: f,
		Sender:  s,
		State:   state.New(t.TempDir(), false),
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, s
}

// --- Daily ---

func TestDailySendsToday(t *testing.T) {
	d, s := deps(t, &fakeFetcher{body: fixture(t)})
	if err := Daily(context.Background(), at(t, "2026-10-12T10:00"), d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 || !strings.HasPrefix(s.sent[0], "🍽 <b>Speiseplan Montag, 12.10.</b>") {
		t.Fatalf("sent = %q", s.sent)
	}
	if !strings.Contains(s.sent[0], "⭐ Risotto mit Pfifferlinge – 9,00 €") {
		t.Errorf("special missing: %q", s.sent[0])
	}
}

func TestDailyNoMenuNotice(t *testing.T) {
	d, s := deps(t, &fakeFetcher{body: fixture(t)})
	if err := Daily(context.Background(), at(t, "2026-10-19T10:00"), d); err != nil {
		t.Fatal(err)
	}
	want := "Für heute (Mo, 19.10.) ist kein Speiseplan online."
	if len(s.sent) != 1 || s.sent[0] != want {
		t.Errorf("sent = %q, want %q", s.sent, want)
	}
}

func TestDailyOutsideWindowDoesNothing(t *testing.T) {
	// 08:30 = Persistent=true catch-up after boot; the regular 10:00 run
	// would otherwise send the same day's menu a second time.
	for _, now := range []string{"2026-10-17T10:00", "2026-10-18T10:00", "2026-10-12T14:00", "2026-10-12T23:30", "2026-10-12T08:30", "2026-10-12T09:59"} {
		t.Run(now, func(t *testing.T) {
			f := &fakeFetcher{err: errors.New("must not fetch")}
			d, s := deps(t, f)
			if err := Daily(context.Background(), at(t, now), d); err != nil {
				t.Fatal(err)
			}
			if f.calls != 0 || len(s.sent) != 0 {
				t.Errorf("fetch calls = %d, sent = %q", f.calls, s.sent)
			}
		})
	}
}

func TestDailyErrors(t *testing.T) {
	t.Run("fetch error", func(t *testing.T) {
		d, s := deps(t, &fakeFetcher{err: errors.New("boom")})
		if err := Daily(context.Background(), at(t, "2026-10-12T10:00"), d); err == nil {
			t.Error("want error")
		}
		if len(s.sent) != 0 {
			t.Errorf("sent = %q", s.sent)
		}
	})
	t.Run("parse error is not 'no menu'", func(t *testing.T) {
		d, s := deps(t, &fakeFetcher{body: []byte("<html><body>Wartung</body></html>")})
		err := Daily(context.Background(), at(t, "2026-10-12T10:00"), d)
		if !errors.Is(err, menu.ErrNoMenu) {
			t.Errorf("err = %v, want ErrNoMenu", err)
		}
		if len(s.sent) != 0 {
			t.Errorf("sent = %q", s.sent)
		}
	})
	t.Run("send error", func(t *testing.T) {
		d, s := deps(t, &fakeFetcher{body: fixture(t)})
		s.err = errors.New("telegram down")
		if err := Daily(context.Background(), at(t, "2026-10-12T10:00"), d); err == nil {
			t.Error("want error")
		}
	})
}

// --- Weekly ---

func TestWeeklySendsNextWeekOnce(t *testing.T) {
	d, s := deps(t, &fakeFetcher{body: fixture(t)})
	now := at(t, "2026-10-09T15:00")
	if err := Weekly(context.Background(), now, d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 || !strings.HasPrefix(s.sent[0], "📅 <b>Speiseplan nächste Woche (12.–16.10.)</b>") {
		t.Fatalf("sent = %q", s.sent)
	}
	st, _ := d.State.Load()
	if st.WeeklySent != "2026-W42" {
		t.Errorf("WeeklySent = %q, want 2026-W42", st.WeeklySent)
	}

	// The 16:00 run must not send again.
	if err := Weekly(context.Background(), at(t, "2026-10-09T16:00"), d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 {
		t.Errorf("sent twice: %q", s.sent)
	}
}

func TestWeeklyAlreadySentSkipsFetch(t *testing.T) {
	f := &fakeFetcher{err: errors.New("must not fetch")}
	d, s := deps(t, f)
	if err := d.State.Save(state.State{WeeklySent: "2026-W42"}); err != nil {
		t.Fatal(err)
	}
	if err := Weekly(context.Background(), at(t, "2026-10-09T17:00"), d); err != nil {
		t.Fatal(err)
	}
	if f.calls != 0 || len(s.sent) != 0 {
		t.Errorf("fetch calls = %d, sent = %q", f.calls, s.sent)
	}
}

func TestWeeklyNotPublishedYet(t *testing.T) {
	// Friday 16 Oct: next week is 19–23 Oct, the page still shows 12–16 Oct.
	d, s := deps(t, &fakeFetcher{body: fixture(t)})
	if err := Weekly(context.Background(), at(t, "2026-10-16T15:00"), d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 0 {
		t.Errorf("sent = %q", s.sent)
	}
	st, _ := d.State.Load()
	if st.WeeklySent != "" {
		t.Errorf("WeeklySent = %q, want empty", st.WeeklySent)
	}
}

func TestWeeklyOutsideWindowDoesNothing(t *testing.T) {
	for _, now := range []string{"2026-10-09T14:59", "2026-10-09T21:00", "2026-10-10T15:00", "2026-10-08T15:00"} {
		t.Run(now, func(t *testing.T) {
			f := &fakeFetcher{err: errors.New("must not fetch")}
			d, s := deps(t, f)
			if err := Weekly(context.Background(), at(t, now), d); err != nil {
				t.Fatal(err)
			}
			if f.calls != 0 || len(s.sent) != 0 {
				t.Errorf("fetch calls = %d, sent = %q", f.calls, s.sent)
			}
		})
	}
}

func TestWeeklyFetchError(t *testing.T) {
	d, _ := deps(t, &fakeFetcher{err: errors.New("boom")})
	if err := Weekly(context.Background(), at(t, "2026-10-09T15:00"), d); err == nil {
		t.Error("want error")
	}
}

func TestWeeklyYearBoundary(t *testing.T) {
	page := `<html><body>
<h2>Montag, 28, Dezember 2026</h2><a class="elementor-toggle-title">• Eintopf</a>
<h2>Freitag, 1, Januar 2027</h2><a class="elementor-toggle-title">• Fisch</a>
</body></html>`
	d, s := deps(t, &fakeFetcher{body: []byte(page)})
	if err := Weekly(context.Background(), at(t, "2026-12-25T15:00"), d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 || !strings.HasPrefix(s.sent[0], "📅 <b>Speiseplan nächste Woche (28.12.–01.01.)</b>") {
		t.Fatalf("sent = %q", s.sent)
	}
	st, _ := d.State.Load()
	if st.WeeklySent != "2026-W53" {
		t.Errorf("WeeklySent = %q, want 2026-W53", st.WeeklySent)
	}
}

func TestWeeklyStateSaveFailureStillSucceeds(t *testing.T) {
	d, s := deps(t, &fakeFetcher{body: fixture(t)})
	d.State = state.New(filepath.Join(t.TempDir(), "missing-dir"), false)
	if err := Weekly(context.Background(), at(t, "2026-10-09T15:00"), d); err != nil {
		t.Fatalf("err = %v, want nil (sent; a retry would duplicate)", err)
	}
	if len(s.sent) != 1 {
		t.Errorf("sent = %q", s.sent)
	}
}

// --- NotifyFailure ---

func TestNotifyFailureOncePerDay(t *testing.T) {
	d, s := deps(t, &fakeFetcher{})
	ctx := context.Background()

	if err := NotifyFailure(ctx, at(t, "2026-10-16T10:20"), "menubot-daily.service", d); err != nil {
		t.Fatal(err)
	}
	if err := NotifyFailure(ctx, at(t, "2026-10-16T15:20"), "menubot-weekly.service", d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 || !strings.Contains(s.sent[0], "menubot-daily.service") {
		t.Fatalf("sent = %q, want exactly the daily notice", s.sent)
	}
	if err := NotifyFailure(ctx, at(t, "2026-10-19T10:20"), "menubot-daily.service", d); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 2 {
		t.Errorf("next day: sent = %q, want 2 notices", s.sent)
	}
	st, _ := d.State.Load()
	if st.FailureNotified != "2026-10-19" {
		t.Errorf("FailureNotified = %q", st.FailureNotified)
	}
}

func TestNotifyFailureSendError(t *testing.T) {
	d, s := deps(t, &fakeFetcher{})
	s.err = errors.New("telegram down")
	if err := NotifyFailure(context.Background(), at(t, "2026-10-16T10:20"), "u", d); err == nil {
		t.Error("want error")
	}
	st, _ := d.State.Load()
	if st.FailureNotified != "" {
		t.Errorf("FailureNotified = %q, want empty after failed send", st.FailureNotified)
	}
}
