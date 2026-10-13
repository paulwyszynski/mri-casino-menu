package menu

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Parse extracts the days from the page. Date headings (<h1>–<h6>) start a
// day, any other heading ends it, and <a class="elementor-toggle-title">
// elements are the dishes of the current day.
func Parse(r io.Reader) ([]Day, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("menu: parse html: %w", err)
	}

	var days []Day
	cur := -1
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case isHeading(n):
				if date, ok := parseDate(clean(textOf(n))); ok {
					days = append(days, Day{Date: date})
					cur = len(days) - 1
				} else {
					cur = -1
				}
				return
			case n.DataAtom == atom.A && hasClass(n, "elementor-toggle-title"):
				if cur >= 0 {
					if dish, ok := parseDish(textOf(n)); ok {
						days[cur].Dishes = append(days[cur].Dishes, dish)
					}
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(days) == 0 {
		return nil, ErrNoMenu
	}
	for _, d := range days {
		if len(d.Dishes) == 0 {
			return nil, fmt.Errorf("menu: %s has no dishes", d.Date)
		}
	}
	return days, nil
}

func isHeading(n *html.Node) bool {
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// clean turns non-breaking spaces into spaces, collapses whitespace and trims.
func clean(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

var dateRe = regexp.MustCompile(`(?i)^(?:montag|dienstag|mittwoch|donnerstag|freitag|samstag|sonntag)\s*,?\s*(\d{1,2})\s*[.,]?\s*(\p{L}+)\s*,?\s*(\d{4})$`)

var months = map[string]time.Month{
	"januar": time.January, "februar": time.February, "märz": time.March,
	"maerz": time.March, "april": time.April, "mai": time.May,
	"juni": time.June, "juli": time.July, "august": time.August,
	"september": time.September, "oktober": time.October,
	"november": time.November, "dezember": time.December,
}

// parseDate parses a cleaned heading such as "Montag ,12, Oktober 2026".
// The weekday word is required but not checked against the date.
func parseDate(s string) (Date, bool) {
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return Date{}, false
	}
	day, _ := strconv.Atoi(m[1])
	month, ok := months[strings.ToLower(m[2])]
	if !ok {
		return Date{}, false
	}
	year, _ := strconv.Atoi(m[3])
	d := Date{year, month, day}
	if DateOf(d.Time()) != d { // rejects e.g. 31 February
		return Date{}, false
	}
	return d, true
}

var (
	priceRe    = regexp.MustCompile(`(\d{1,3}(?:[.,]\d{1,2})?)\s*€`)
	allergenRe = regexp.MustCompile(`(?i)\(\s*(?:[a-z]\d?|\d{1,2})(?:\s*,\s*(?:[a-z]\d?|\d{1,2}))*\s*\)`)
)

// parseDish turns a raw dish line into a Dish: bullet, price and allergen
// codes are removed from the name; the price is kept separately.
func parseDish(raw string) (Dish, bool) {
	s := clean(raw)
	s = strings.TrimLeft(s, "•·*-– ")
	var price string
	if m := priceRe.FindStringSubmatchIndex(s); m != nil {
		price = normalizePrice(s[m[2]:m[3]])
		s = s[:m[0]] + " " + s[m[1]:]
	}
	s = allergenRe.ReplaceAllString(s, " ")
	s = strings.TrimRight(clean(s), " ,-–")
	if s == "" {
		return Dish{}, false
	}
	return Dish{Name: s, Price: price}, true
}

// normalizePrice turns "9", "9.5", "9,50" into "9,00 €", "9,50 €", "9,50 €".
func normalizePrice(p string) string {
	p = strings.ReplaceAll(p, ".", ",")
	euros, cents, found := strings.Cut(p, ",")
	if !found {
		cents = "00"
	}
	if len(cents) == 1 {
		cents += "0"
	}
	return euros + "," + cents + " €"
}
