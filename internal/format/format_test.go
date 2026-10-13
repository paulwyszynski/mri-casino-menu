package format

import (
	"testing"
	"time"

	"github.com/paulwyszynski/mri-casino-menu/internal/menu"
)

var monday = menu.Day{
	Date: menu.Date{Year: 2026, Month: time.October, Day: 12},
	Dishes: []menu.Dish{
		{Name: "Peperonata mit Bio Polenta-vegan"},
		{Name: "Hähnchenbruststreifen in Erdnuss-Curry Sauce mit Gemüse und Basmatireis"},
		{Name: "Risotto mit Pfifferlinge", Price: "9,00 €"},
	},
}

var tuesday = menu.Day{
	Date: menu.Date{Year: 2026, Month: time.October, Day: 13},
	Dishes: []menu.Dish{
		{Name: "Curry & Reis <scharf>"},
	},
}

func TestDay(t *testing.T) {
	want := "🍽 <b>Speiseplan Montag, 12.10.</b>\n" +
		"\n" +
		"• Peperonata mit Bio Polenta-vegan\n" +
		"• Hähnchenbruststreifen in Erdnuss-Curry Sauce mit Gemüse und Basmatireis\n" +
		"⭐ Risotto mit Pfifferlinge – 9,00 €"
	if got := Day(monday); got != want {
		t.Errorf("Day()\n got: %q\nwant: %q", got, want)
	}
}

func TestDayEscapesHTML(t *testing.T) {
	want := "🍽 <b>Speiseplan Dienstag, 13.10.</b>\n\n• Curry &amp; Reis &lt;scharf&gt;"
	if got := Day(tuesday); got != want {
		t.Errorf("Day()\n got: %q\nwant: %q", got, want)
	}
}

func TestWeek(t *testing.T) {
	want := "📅 <b>Speiseplan nächste Woche (12.–13.10.)</b>\n" +
		"\n" +
		"<b>Montag, 12.10.</b>\n" +
		"• Peperonata mit Bio Polenta-vegan\n" +
		"• Hähnchenbruststreifen in Erdnuss-Curry Sauce mit Gemüse und Basmatireis\n" +
		"⭐ Risotto mit Pfifferlinge – 9,00 €\n" +
		"\n" +
		"<b>Dienstag, 13.10.</b>\n" +
		"• Curry &amp; Reis &lt;scharf&gt;"
	if got := Week([]menu.Day{monday, tuesday}); got != want {
		t.Errorf("Week()\n got: %q\nwant: %q", got, want)
	}
}

func TestWeekAcrossMonthsAndYears(t *testing.T) {
	days := []menu.Day{
		{Date: menu.Date{Year: 2026, Month: time.December, Day: 28}, Dishes: []menu.Dish{{Name: "A"}}},
		{Date: menu.Date{Year: 2027, Month: time.January, Day: 1}, Dishes: []menu.Dish{{Name: "B"}}},
	}
	want := "📅 <b>Speiseplan nächste Woche (28.12.–01.01.)</b>\n\n" +
		"<b>Montag, 28.12.</b>\n• A\n\n" +
		"<b>Freitag, 01.01.</b>\n• B"
	if got := Week(days); got != want {
		t.Errorf("Week()\n got: %q\nwant: %q", got, want)
	}
}

func TestNoMenu(t *testing.T) {
	want := "Für heute (Mo, 19.10.) ist kein Speiseplan online."
	if got := NoMenu(menu.Date{Year: 2026, Month: time.October, Day: 19}); got != want {
		t.Errorf("NoMenu() = %q, want %q", got, want)
	}
}

func TestFailure(t *testing.T) {
	want := "⚠️ Speiseplan konnte nicht abgerufen werden (menubot-daily.service). " +
		"Details: <code>journalctl -u menubot-daily.service</code>"
	if got := Failure("menubot-daily.service"); got != want {
		t.Errorf("Failure() = %q, want %q", got, want)
	}
}
