// Package format builds the Telegram message texts (parse_mode=HTML).
package format

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/paulwyszynski/mri-casino-menu/internal/menu"
)

var weekdays = map[time.Weekday]string{
	time.Monday: "Montag", time.Tuesday: "Dienstag", time.Wednesday: "Mittwoch",
	time.Thursday: "Donnerstag", time.Friday: "Freitag",
	time.Saturday: "Samstag", time.Sunday: "Sonntag",
}

var weekdaysShort = map[time.Weekday]string{
	time.Monday: "Mo", time.Tuesday: "Di", time.Wednesday: "Mi",
	time.Thursday: "Do", time.Friday: "Fr", time.Saturday: "Sa", time.Sunday: "So",
}

// Day is the daily message.
func Day(d menu.Day) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🍽 <b>Speiseplan %s</b>\n\n", dayLabel(d.Date))
	writeDishes(&b, d.Dishes)
	return strings.TrimRight(b.String(), "\n")
}

// Week is the weekly message. days must be non-empty.
func Week(days []menu.Day) string {
	var b strings.Builder
	first, last := days[0].Date, days[len(days)-1].Date
	fmt.Fprintf(&b, "📅 <b>Speiseplan nächste Woche (%s)</b>\n", dateRange(first, last))
	for _, d := range days {
		fmt.Fprintf(&b, "\n<b>%s</b>\n", dayLabel(d.Date))
		writeDishes(&b, d.Dishes)
	}
	return strings.TrimRight(b.String(), "\n")
}

// NoMenu is the notice sent when today is not on the page.
func NoMenu(d menu.Date) string {
	return fmt.Sprintf("Für heute (%s, %s) ist kein Speiseplan online.",
		weekdaysShort[d.Weekday()], dayMonth(d))
}

// Failure is the notice sent when a job failed for good.
func Failure(unit string) string {
	u := html.EscapeString(unit)
	return fmt.Sprintf("⚠️ Speiseplan konnte nicht abgerufen werden (%s). Details: <code>journalctl -u %s</code>", u, u)
}

func writeDishes(b *strings.Builder, dishes []menu.Dish) {
	for _, d := range dishes {
		name := html.EscapeString(d.Name)
		if d.Price != "" {
			fmt.Fprintf(b, "⭐ %s – %s\n", name, html.EscapeString(d.Price))
		} else {
			fmt.Fprintf(b, "• %s\n", name)
		}
	}
}

func dayLabel(d menu.Date) string {
	return weekdays[d.Weekday()] + ", " + dayMonth(d)
}

func dayMonth(d menu.Date) string {
	return fmt.Sprintf("%02d.%02d.", d.Day, int(d.Month))
}

func dateRange(a, b menu.Date) string {
	switch {
	case a == b:
		return dayMonth(a)
	case a.Year == b.Year && a.Month == b.Month:
		return fmt.Sprintf("%02d.–%s", a.Day, dayMonth(b))
	default:
		return dayMonth(a) + "–" + dayMonth(b)
	}
}
