// Package menu parses the Casino Catering "Speiseplan" page.
package menu

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoMenu is returned when the page contains no recognisable day.
var ErrNoMenu = errors.New("menu: no days found on page")

// Date is a calendar date without time or location.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the calendar date of t in t's location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{y, m, d}
}

// Time returns midnight UTC of d.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

func (d Date) AddDays(n int) Date { return DateOf(d.Time().AddDate(0, 0, n)) }

func (d Date) Before(o Date) bool { return d.Time().Before(o.Time()) }

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// Dish is one menu line. Price is normalised to "9,00 €" and empty when the
// line has no price.
type Dish struct {
	Name  string
	Price string
}

// Day is the menu of one date, in page order.
type Day struct {
	Date   Date
	Dishes []Dish
}
