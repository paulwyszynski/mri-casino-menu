package menu

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/speiseplan-2026-10-12.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Day{
		{Date{2026, time.October, 12}, []Dish{
			{Name: "Peperonata mit Bio Polenta-vegan"},
			{Name: "Hähnchenbruststreifen in Erdnuss-Curry Sauce mit Gemüse und Basmatireis"},
			{Name: "Risotto mit Pfifferlinge", Price: "9,00 €"},
		}},
		{Date{2026, time.October, 13}, []Dish{
			{Name: "Hausgemachte Käsespätzle"},
			{Name: "Schweinehaxenfleisch mit Sauerkraut und Kartoffelpüree"},
			{Name: "Sloppy Joe Burger", Price: "11,00 €"},
		}},
		{Date{2026, time.October, 14}, []Dish{
			{Name: "Grillgemüse in Tikka Masala Sauce mit Duftreis"},
			{Name: "Picata (Hähnchen) mit Spaghetti und Tomatensauce"},
			{Name: "Bowl mit Hirtenkäse", Price: "11,00 €"},
		}},
		{Date{2026, time.October, 15}, []Dish{
			{Name: "Couscous Taler mit Wirsing-Karotten Gemüse"},
			{Name: "Wraps mit Hackfleisch (Rind und Schwein), Gemüse und Salatgarnitur"},
		}},
		{Date{2026, time.October, 16}, []Dish{
			{Name: "Gemüse in Tempura mit Kokoscurry-Sauce und Basmatireis"},
			{Name: "Pfannengyros (Schwein) mit Pommes frites und Tzatziki"},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want Date
		ok   bool
	}{
		{"Montag ,12, Oktober 2026", Date{2026, time.October, 12}, true},
		{"Dienstag, 13, Oktober 2026 ", Date{2026, time.October, 13}, true},
		{"Mittwoch, 4. März 2026", Date{2026, time.March, 4}, true},
		{"Mittwoch,4.Maerz 2026", Date{2026, time.March, 4}, true},
		{"Freitag 16, Oktober 2026", Date{2026, time.October, 16}, true},
		{"FREITAG, 1, Januar 2027", Date{2027, time.January, 1}, true},
		{"Täglich wechselnde Suppen und frische Salate", Date{}, false},
		{"Montag, 12. Oktober", Date{}, false},       // no year
		{"Montag, 31, Februar 2026", Date{}, false},  // invalid date
		{"Montag, 12, Brumaire 2026", Date{}, false}, // unknown month
	}
	for _, tt := range tests {
		got, ok := parseDate(clean(tt.in))
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseDate(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestParseDish(t *testing.T) {
	tests := []struct {
		in   string
		want Dish
		ok   bool
	}{
		{"•\tPeperonata mit Bio Polenta-vegan", Dish{Name: "Peperonata mit Bio Polenta-vegan"}, true},
		{"•\tHausgemachte Käsespätzle (a1,c,g)", Dish{Name: "Hausgemachte Käsespätzle"}, true},
		{"•\tSchweinehaxenfleisch mit Sauerkraut (g,2)", Dish{Name: "Schweinehaxenfleisch mit Sauerkraut"}, true},
		{"Risotto mit Pfifferlinge 9,00€", Dish{Name: "Risotto mit Pfifferlinge", Price: "9,00 €"}, true},
		{" Bowl mit Hirtenkäse  11,00€ (a1,f,c,g)", Dish{Name: "Bowl mit Hirtenkäse", Price: "11,00 €"}, true},
		{"Picata (Hähnchen) mit Spaghetti (a1,c,g)", Dish{Name: "Picata (Hähnchen) mit Spaghetti"}, true},
		{"– Linsensuppe (e)", Dish{Name: "Linsensuppe"}, true},
		{"- Burger 9€", Dish{Name: "Burger", Price: "9,00 €"}, true},
		{"* Pasta 9.50 €", Dish{Name: "Pasta", Price: "9,50 €"}, true},
		{"Schnitzel 11,5€", Dish{Name: "Schnitzel", Price: "11,50 €"}, true},
		{"Curry & Reis <scharf>", Dish{Name: "Curry & Reis <scharf>"}, true},
		{"•\t ", Dish{}, false},
		{"(a1,c,g)", Dish{}, false},
	}
	for _, tt := range tests {
		got, ok := parseDish(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseDish(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func page(blocks ...string) string {
	return "<html><body>" + strings.Join(blocks, "") + "</body></html>"
}

func heading(s string) string { return "<h2>" + s + "</h2>" }

func item(s string) string {
	return `<div class="elementor-tab-title"><a class="elementor-toggle-title" tabindex="0">` + s + `</a></div>`
}

func TestParseStructure(t *testing.T) {
	t.Run("empty page is ErrNoMenu", func(t *testing.T) {
		_, err := Parse(strings.NewReader(page("<p>Wartungsmodus</p>")))
		if !errors.Is(err, ErrNoMenu) {
			t.Fatalf("err = %v, want ErrNoMenu", err)
		}
	})
	t.Run("day without dishes is an error", func(t *testing.T) {
		_, err := Parse(strings.NewReader(page(heading("Montag, 12, Oktober 2026"))))
		if err == nil || errors.Is(err, ErrNoMenu) {
			t.Fatalf("err = %v, want a no-dishes error", err)
		}
	})
	t.Run("dishes before first date and after non-date heading are ignored", func(t *testing.T) {
		got, err := Parse(strings.NewReader(page(
			item("Vorspeise"),
			heading("Montag, 12, Oktober 2026"),
			item("•\tSuppe"),
			heading("Kontakt"),
			item("Footer-Link"),
		)))
		if err != nil {
			t.Fatal(err)
		}
		want := []Day{{Date{2026, time.October, 12}, []Dish{{Name: "Suppe"}}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("HTML entities are decoded", func(t *testing.T) {
		got, err := Parse(strings.NewReader(page(
			heading("Montag,&nbsp;12, Oktober 2026"),
			item("Curry &amp; Reis"),
		)))
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Dishes[0].Name != "Curry & Reis" {
			t.Errorf("name = %q", got[0].Dishes[0].Name)
		}
	})
}

func TestDateHelpers(t *testing.T) {
	d := Date{2026, time.December, 31}
	if got := d.AddDays(1); got != (Date{2027, time.January, 1}) {
		t.Errorf("AddDays = %v", got)
	}
	if d.Weekday() != time.Thursday {
		t.Errorf("Weekday = %v", d.Weekday())
	}
	if d.String() != "2026-12-31" {
		t.Errorf("String = %q", d.String())
	}
	if !d.Before(d.AddDays(1)) || d.Before(d) {
		t.Error("Before broken")
	}
	berlin, _ := time.LoadLocation("Europe/Berlin")
	if got := DateOf(time.Date(2026, 10, 12, 0, 30, 0, 0, berlin)); got != (Date{2026, time.October, 12}) {
		t.Errorf("DateOf = %v", got)
	}
}
