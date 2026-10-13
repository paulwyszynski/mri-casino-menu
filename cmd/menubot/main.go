// Command menubot posts the Casino Catering menu to Telegram.
// It runs once and exits; systemd timers decide when.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
	_ "time/tzdata" // the Pi image may lack zoneinfo; embed it

	"github.com/paulwyszynski/mri-casino-menu/internal/fetch"
	"github.com/paulwyszynski/mri-casino-menu/internal/jobs"
	"github.com/paulwyszynski/mri-casino-menu/internal/state"
	"github.com/paulwyszynski/mri-casino-menu/internal/telegram"
)

const defaultMenuURL = "https://casinocatering.de/speiseplan/"

const usage = `usage: menubot [--dry-run] [--now 2006-01-02T15:04] daily|weekly|notify-failure <unit>`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	log := slog.New(slog.NewTextHandler(stderr, nil))

	fs := flag.NewFlagSet("menubot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	dryRun := fs.Bool("dry-run", false, "print messages instead of sending; do not write state")
	nowFlag := fs.String("now", "", "override the current time (Europe/Berlin), format 2006-01-02T15:04")

	// Flags may come before or after the subcommand.
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	cmd := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 2
	}
	rest := fs.Args()

	// Validate the command before config, so typos are reported as such.
	switch cmd {
	case "daily", "weekly":
	case "notify-failure":
		if len(rest) != 1 {
			fs.Usage()
			return 2
		}
	default:
		fmt.Fprintf(stderr, "unknown command %q\n%s\n", cmd, usage)
		return 2
	}

	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		fmt.Fprintln(stderr, "load timezone:", err)
		return 2
	}
	now := time.Now().In(loc)
	if *nowFlag != "" {
		if now, err = time.ParseInLocation("2006-01-02T15:04", *nowFlag, loc); err != nil {
			fmt.Fprintln(stderr, "invalid --now:", err)
			return 2
		}
	}

	deps, err := buildDeps(*dryRun, stdout, log)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch cmd {
	case "daily":
		err = jobs.Daily(ctx, now, deps)
	case "weekly":
		err = jobs.Weekly(ctx, now, deps)
	case "notify-failure":
		err = jobs.NotifyFailure(ctx, now, rest[0], deps)
	}
	if err != nil {
		log.Error("job failed", "job", cmd, "err", err)
		return 1
	}
	return 0
}

func buildDeps(dryRun bool, stdout io.Writer, log *slog.Logger) (jobs.Deps, error) {
	stateDir := os.Getenv("STATE_DIR")
	if stateDir == "" {
		stateDir = os.Getenv("STATE_DIRECTORY") // set by systemd StateDirectory=
	}
	if stateDir == "" {
		stateDir = "."
	}
	menuURL := os.Getenv("MENU_URL")
	if menuURL == "" {
		menuURL = defaultMenuURL
	}

	d := jobs.Deps{
		Fetcher: fetch.New(menuURL),
		State:   state.New(stateDir, dryRun),
		Log:     log,
	}
	if dryRun {
		d.Sender = printSender{w: stdout}
		return d, nil
	}
	token, chatID := os.Getenv("TELEGRAM_BOT_TOKEN"), os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		return jobs.Deps{}, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set (or use --dry-run)")
	}
	d.Sender = telegram.New(token, chatID)
	return d, nil
}

// printSender is the --dry-run sender.
type printSender struct{ w io.Writer }

func (p printSender) Send(_ context.Context, text string) error {
	_, err := fmt.Fprintf(p.w, "%s\n---\n", text)
	return err
}
