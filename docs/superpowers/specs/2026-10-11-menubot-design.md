# menubot — Casino Catering menu to Telegram

**Date:** 2026-10-11
**Status:** Draft for review

## Goal

A Telegram bot posts the lunch menu from <https://casinocatering.de/speiseplan/>
into one Telegram chat:

- **Daily, Mon–Fri 10:00 Europe/Berlin:** today's menu.
- **Friday 15:00 Europe/Berlin:** next week's full menu, retried hourly until
  Friday 20:00 if the website has not published next week yet.

Runs on the user's Raspberry Pi (64-bit Raspberry Pi OS Lite, headless, SSH
access), as a Go binary triggered by systemd timers.

## Source page

The page is WordPress content, **not** an HTML `<table>`. Observed on
2026-10-11 (a Sunday, already showing the week of 12–16 Oct):

```
Montag ,12, Oktober 2026
•	Peperonata mit Bio Polenta-vegan
•	Hähnchenbruststreifen in Erdnuss-Curry Sauce mit Gemüse und Basmatireis (e)
Risotto mit Pfifferlinge 9,00€
Dienstag, 13, Oktober 2026
•	Hausgemachte Käsespätzle (a1,c,g)
...
Donnerstag, 15, Oktober 2026        <- no special this day
•	Couscous Taler ...
•	Wraps mit Hackfleisch ... (a1,g)
```

Per day: a date heading, usually two bullet dishes, optionally one special
with a price. Spacing and commas in the heading are inconsistent. The page
shows exactly one week at a time; when it switches to the next week is
unknown.

## Behaviour

### Daily job — `menubot daily`

Triggered `Mon..Fri 10:00 Europe/Berlin`, `Persistent=true`.

1. If now (Europe/Berlin) is not Mon–Fri, or is 14:00 or later, exit 0
   without sending. Lunch ends 13:30, so a catch-up run after a Pi outage
   is pointless after that.
2. Fetch and parse the page.
3. If a day matching today's date exists → send it.
4. If parsing succeeded but today is missing (holiday, page already switched
   to next week) → send a notice:
   `Für heute (Mo, 12.10.) ist kein Speiseplan online.`
5. On fetch error or parse error → exit non-zero (systemd retries, see
   *Failure handling*).

### Weekly job — `menubot weekly`

Triggered `Fri 15..20:00 Europe/Berlin` (hourly, 6 runs), `Persistent=true`.

1. If now is not Friday between 15:00 and 20:59 → exit 0. This stops a
   catch-up run after boot from sending on Saturday.
2. Compute *next week* = the Mon–Fri following the current week, and its
   ISO week key (e.g. `2026-W43`).
3. If the state file already records that key → exit 0.
4. Fetch and parse the page.
5. If the parsed days fall in next week (at least one parsed date is in
   next week's Mon–Fri, and none are outside it) → send the week, then
   write the key to the state file.
6. Otherwise (page still shows the current week) → log
   `next week not published yet` and exit 0. The next hourly run retries.
   After the 20:00 run nothing more happens for that week.
7. On fetch error or parse error → exit non-zero.

### Definition of "parse succeeded"

At least one date heading found **and** each found day has at least one
dish. Zero days is a parse error, never "no menu today". That way a
changed page layout leads to the failure notice, not a misleading
"no menu".

### Failure handling

- Each service has `Restart=on-failure`, `RestartSec=5min`, and a start
  limit of 3 attempts (`StartLimitBurst=4`, `StartLimitIntervalSec=30min`).
- When the restarts are used up, the unit enters `failed` and
  `OnFailure=menubot-failure@%n.service` runs `menubot notify-failure <unit>`,
  which sends:
  `⚠️ Speiseplan konnte nicht abgerufen werden (menubot-daily.service). Details: <code>journalctl -u menubot-daily.service</code>`
- **At most one ⚠️ notice per calendar day (Europe/Berlin), across all
  jobs.** `notify-failure` records the date of the last notice in the
  state file. If a notice was already sent today, it only logs and exits 0.
  Without this, a page that stays broken on a Friday afternoon would cause
  up to 6 weekly notices plus the daily one. One notice per day is enough,
  since daily and weekly failures share the same cause.
- `notify-failure` itself does not fetch the page. If Telegram is
  unreachable too, the error is only in the journal.

## State file

A single file, `$STATE_DIR/state.json` (`/var/lib/menubot/state.json` on the Pi):

```json
{ "weekly_sent": "2026-W43", "failure_notified": "2026-10-16" }
```

- Each field holds only the latest value and is overwritten in place, so
  the file never grows and needs no cleanup or rotation.
- The daily job never writes it. The weekly job writes it once per week
  after a successful send. `notify-failure` writes it at most once per day,
  only when a notice is sent.
- Writes are atomic: write `state.json.tmp`, `fsync`, rename.
- A missing or corrupt file is treated as empty (logged as a warning). The
  worst case is one duplicate weekly message or failure notice.

## Message format

Telegram `parse_mode=HTML`. Dish text is HTML-escaped. Allergen codes such
as `(a1,c,g)`, `(g,2)`, `(e)` are removed. Prices are kept and normalised to
`9,00 €`. The special (the line with a price) gets ⭐.

Daily:

```
🍽 <b>Speiseplan Montag, 12.10.</b>

• Peperonata mit Bio Polenta-vegan
• Hähnchenbruststreifen in Erdnuss-Curry Sauce mit Gemüse und Basmatireis
⭐ Risotto mit Pfifferlinge – 9,00 €
```

Weekly:

```
📅 <b>Speiseplan nächste Woche (12.–16.10.)</b>

<b>Montag, 12.10.</b>
• ...
⭐ ... – 9,00 €

<b>Dienstag, 13.10.</b>
• ...
```

## Parsing rules

- Use `golang.org/x/net/html` to walk the page in document order. Only two
  kinds of element matter (verified against the 2026-10-12 page):
  - headings `<h1>`–`<h6>`: a heading that matches the date pattern starts
    a new day; any other heading ends the current day;
  - `<a class="elementor-toggle-title">`: one dish, added to the current
    day (ignored before the first date heading).
  Everything else (nav, footer, `og:description`, the `Inhalt umschalten`
  accordion bodies) is never looked at. Non-breaking spaces count as
  spaces.
- **Date heading:** regex, case-insensitive, flexible whitespace/commas:
  `(Montag|Dienstag|Mittwoch|Donnerstag|Freitag)\s*,?\s*(\d{1,2})\s*[,.]?\s*(<German month>)\s*,?\s*(\d{4})`.
  German month names map to `time.Month` (`März` with umlaut, also `Maerz`).
- **Dish:** a line following a date heading, before the next heading.
  Leading bullet characters (`•`, `-`, tab) are trimmed.
- **Price:** `(\d+,\d{2})\s*€` anywhere in the line. Removed from the name
  and stored separately.
- **Allergens:** trailing or embedded parenthesised groups that consist
  only of codes like `a1`, `c`, `g`, `2`, separated by commas/spaces, are
  removed. Parentheses with real words (e.g. `(Hähnchen)`, `(Rind und Schwein)`)
  are kept.
- `Täglich wechselnde Suppen und frische Salate` is a non-date heading and
  therefore ignored; `Inhalt umschalten` is not a dish link. No skip list
  is needed.

## Code layout

```
mri-casino-menu/
├── cmd/menubot/main.go           # subcommands daily | weekly | notify-failure; --dry-run; env config
├── internal/
│   ├── menu/                     # parse.go, parse_test.go, testdata/speiseplan-2026-10-12.html
│   ├── fetch/fetch.go            # HTTP GET, 20s timeout, User-Agent
│   ├── telegram/                 # client.go (sendMessage), client_test.go (httptest)
│   ├── format/                   # format.go, format_test.go
│   ├── state/state.go            # JSON file: last weekly key sent, last failure-notice date
│   └── jobs/                     # jobs.go (Daily, Weekly, NotifyFailure), jobs_test.go
├── deploy/
│   ├── menubot-daily.service / .timer
│   ├── menubot-weekly.service / .timer
│   ├── menubot-failure@.service
│   └── env.example
├── deploy.sh
├── go.mod                        # only external dependency: golang.org/x/net
└── README.md
```

Jobs take their dependencies as interfaces/values so they can be tested
without network or real time:

```go
type Fetcher interface{ Fetch(ctx context.Context) ([]byte, error) }
type Sender  interface{ Send(ctx context.Context, html string) error }

func Daily(ctx context.Context, now time.Time, f Fetcher, s Sender) error
func Weekly(ctx context.Context, now time.Time, f Fetcher, s Sender, st *state.Store) error
```

## Configuration

Environment variables (via `EnvironmentFile=/etc/menubot/env`, mode `600`):

| Variable             | Required | Default                                   |
|----------------------|----------|-------------------------------------------|
| `TELEGRAM_BOT_TOKEN` | yes      | —                                         |
| `TELEGRAM_CHAT_ID`   | yes      | —                                         |
| `MENU_URL`           | no       | `https://casinocatering.de/speiseplan/`   |
| `STATE_DIR`          | no       | `$STATE_DIRECTORY` (systemd), else `.`    |

`--dry-run` prints the message to stdout instead of sending, and does not
write state. With `--dry-run` the token and chat ID are not required.
`--now 2026-10-12T10:00` overrides the clock for manual testing.

## systemd units

- Services are `Type=oneshot`, `User=menubot`, `Group=menubot`,
  `StateDirectory=menubot` (→ `/var/lib/menubot`), plus basic hardening
  (`ProtectSystem=strict`, `ProtectHome=yes`, `NoNewPrivileges=yes`).
  A fixed system user (not `DynamicUser`) is used because all three
  services share one state file.
- Timers carry the timezone explicitly, so the Pi's own timezone does not
  matter:
  - `OnCalendar=Mon..Fri *-*-* 10:00:00 Europe/Berlin`
  - `OnCalendar=Fri *-*-* 15..20:00:00 Europe/Berlin`
  - both `Persistent=true`; the time-window checks in the jobs make late
    catch-up runs harmless.
- Services order themselves `After=network-online.target time-sync.target`,
  and `install.sh` enables `systemd-time-wait-sync.service`. The Pi has no
  hardware clock; without this, a catch-up run right after boot could see
  the last-shutdown time (fake-hwclock) and judge the time windows wrongly.
- Note: with `Type=oneshot`, `Restart=on-failure` requires systemd ≥ 244.
  Raspberry Pi OS Bookworm ships 252.

## Deployment

The Pi needs neither Go nor Docker.

`./deploy.sh <ssh-host>` (run on the Mac):

1. `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o build/menubot ./cmd/menubot`
2. `scp` the binary and `deploy/*` to a temp directory on the Pi.
3. Over `ssh -t` with `sudo`:
   - create the system user `menubot` if missing
     (`useradd --system --no-create-home --shell /usr/sbin/nologin menubot`)
   - install the binary to `/usr/local/bin/menubot`
   - install the unit files to `/etc/systemd/system/`
   - create `/etc/menubot/env` from `env.example` with mode `600`
     **only if it does not exist**
   - `systemctl daemon-reload`
   - `systemctl enable --now menubot-daily.timer menubot-weekly.timer`,
     but only once `/etc/menubot/env` no longer contains the placeholder
     values; otherwise print a hint to fill it in and run `deploy.sh` again

### First-time setup (documented in README)

1. Get the bot token from @BotFather (bot already exists).
2. Find the chat ID: send a message in the chat, then
   `curl -s "https://api.telegram.org/bot<TOKEN>/getUpdates"` and read
   `chat.id` (negative for groups).
3. `./deploy.sh pi`
4. `ssh pi sudo nano /etc/menubot/env` → fill in the token and chat ID,
   then `./deploy.sh pi` again to enable the timers.
5. Test: `ssh pi /usr/local/bin/menubot daily --dry-run`, then
   `ssh pi sudo systemctl start menubot-daily.service`.

Day-to-day: `systemctl list-timers 'menubot*'`,
`journalctl -u menubot-daily -u menubot-weekly`.

## Testing

- **Parser:** real-page fixture (expected: 5 days, Mon–Fri 12–16 Oct 2026;
  Thursday has no special; Wednesday's special price `11,00 €` with
  allergens stripped; `Picata (Hähnchen)` keeps its parenthesis). Plus
  synthetic variants: odd comma/space placement, missing special, empty
  page → parse error.
- **Format:** expected-output tests for daily, weekly and the no-menu notice,
  including HTML escaping of `&`, `<`.
- **Jobs:** table tests with a fake clock/fetcher/sender and a temp-dir state:
  - daily: today found; today missing → notice; weekend → nothing;
    after 14:00 → nothing; fetch error → error returned.
  - weekly: next week present → sent + state written; already sent →
    nothing; current week only → nothing, no error; outside the Friday
    window → nothing; fetch error → error returned.
  - notify-failure: first failure of the day → sent + date recorded;
    second failure same day (any unit) → not sent; next day → sent.
- **Telegram client:** `httptest` server checks the request path, chat ID,
  `parse_mode`, and maps a non-OK API response to an error.
- `go vet` and `go test ./...` must pass. No live network in tests.

## Out of scope

- Multiple chats, bot commands (e.g. `/heute`), soups/salads, translations.
- Detecting menu changes during the week and re-sending.
