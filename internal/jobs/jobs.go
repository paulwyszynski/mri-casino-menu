// Package jobs decides what to send and when. Scheduling itself is done by
// systemd; every job checks its own time window so late catch-up runs are
// harmless.
package jobs

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/paulwyszynski/mri-casino-menu/internal/format"
	"github.com/paulwyszynski/mri-casino-menu/internal/menu"
	"github.com/paulwyszynski/mri-casino-menu/internal/state"
)

type Fetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
}

type Sender interface {
	Send(ctx context.Context, text string) error
}

type Deps struct {
	Fetcher Fetcher
	Sender  Sender
	State   *state.Store
	Log     *slog.Logger
}

// Daily sends today's menu, or a notice if today is not on the page.
// It does nothing on weekends, before 10:00 (a catch-up run after boot would
// duplicate the regular 10:00 run) and from 14:00 on (lunch is over).
func Daily(ctx context.Context, now time.Time, d Deps) error {
	if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday || now.Hour() < 10 || now.Hour() >= 14 {
		d.Log.Info("outside daily window, skipping", "now", now.Format(time.DateTime))
		return nil
	}
	days, err := fetchMenu(ctx, d)
	if err != nil {
		return err
	}
	today := menu.DateOf(now)
	for _, day := range days {
		if day.Date == today {
			return d.Sender.Send(ctx, format.Day(day))
		}
	}
	d.Log.Info("no menu for today on page", "date", today)
	return d.Sender.Send(ctx, format.NoMenu(today))
}

// Weekly sends next week's menu once, on Friday 15:00–20:59, as soon as the
// page shows next week.
func Weekly(ctx context.Context, now time.Time, d Deps) error {
	if now.Weekday() != time.Friday || now.Hour() < 15 || now.Hour() > 20 {
		d.Log.Info("outside weekly window, skipping", "now", now.Format(time.DateTime))
		return nil
	}
	nextMon := mondayOf(menu.DateOf(now)).AddDays(7)
	nextFri := nextMon.AddDays(4)
	year, week := nextMon.Time().ISOWeek()
	key := fmt.Sprintf("%d-W%02d", year, week)

	st := load(d)
	if st.WeeklySent == key {
		d.Log.Info("weekly menu already sent", "week", key)
		return nil
	}

	days, err := fetchMenu(ctx, d)
	if err != nil {
		return err
	}
	for _, day := range days {
		if day.Date.Before(nextMon) || nextFri.Before(day.Date) {
			d.Log.Info("next week not published yet", "week", key, "page_day", day.Date)
			return nil
		}
	}

	if err := d.Sender.Send(ctx, format.Week(days)); err != nil {
		return err
	}
	st.WeeklySent = key
	if err := d.State.Save(st); err != nil {
		// Do not fail: systemd would retry and send the menu again.
		d.Log.Error("weekly menu sent but state not saved; it may be sent again", "err", err)
	}
	return nil
}

// NotifyFailure posts the failure notice for unit, at most once per day.
func NotifyFailure(ctx context.Context, now time.Time, unit string, d Deps) error {
	today := menu.DateOf(now).String()
	st := load(d)
	if st.FailureNotified == today {
		d.Log.Info("failure notice already sent today", "unit", unit)
		return nil
	}
	if err := d.Sender.Send(ctx, format.Failure(unit)); err != nil {
		return err
	}
	st.FailureNotified = today
	if err := d.State.Save(st); err != nil {
		d.Log.Error("failure notice sent but state not saved", "err", err)
	}
	return nil
}

func fetchMenu(ctx context.Context, d Deps) ([]menu.Day, error) {
	body, err := d.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return menu.Parse(bytes.NewReader(body))
}

func load(d Deps) state.State {
	st, err := d.State.Load()
	if err != nil {
		d.Log.Warn("ignoring unreadable state", "err", err)
	}
	return st
}

func mondayOf(d menu.Date) menu.Date {
	offset := (int(d.Weekday()) + 6) % 7 // Monday=0 … Sunday=6
	return d.AddDays(-offset)
}
