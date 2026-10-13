# mri-casino-menu

Telegram bot that posts the [Casino Catering menu](https://casinocatering.de/speiseplan/) to a chat:

- **Mon–Fri 10:00** — today's menu (or a short notice if today isn't online)
- **Fri 15:00** — next week's menu; retried hourly until 20:00 if not published yet

Runs as a one-shot Go binary triggered by systemd timers on a Raspberry Pi (arm64).
Design: [`docs/superpowers/specs/2026-10-11-menubot-design.md`](docs/superpowers/specs/2026-10-11-menubot-design.md)

## Try it locally

```bash
go test ./...
go run ./cmd/menubot --dry-run daily
go run ./cmd/menubot --dry-run --now 2026-10-09T15:00 weekly
```

`--dry-run` prints instead of sending and needs no token.

## Set up on the Raspberry Pi

You do **not** copy this project to the Pi. Everything is built on your Mac;
`deploy.sh` sends only the compiled binary and the systemd files over SSH.
The Pi needs neither Go nor git.

```
Mac (this repo)                      Raspberry Pi
go test + build (linux/arm64) ─scp─▶ /tmp/menubot-deploy/
                              ─ssh─▶ sudo install.sh → /usr/local/bin/menubot
                                                       /etc/systemd/system/menubot-*
                                                       /etc/menubot/env (first time only)
```

1. **Bot token** — from @BotFather (`/mybots` → your bot → *API Token*).
2. **Chat ID** — send a message in the chat, then:
   ```bash
   curl -s "https://api.telegram.org/bot<TOKEN>/getUpdates"
   ```
   Use the `"chat":{"id":…}` value (groups are negative, e.g. `-1001234567890`).
3. **Deploy** — run on your Mac, in this project folder. Needs Go and working
   SSH to the Pi; `pi` is a host alias from `~/.ssh/config`, or use
   `user@ip`. All steps share one SSH connection, so you type your SSH
   password at most once (none with an SSH key), plus your sudo password on
   the Pi if your user needs one.
   ```bash
   ./deploy.sh pi@192.168.0.3
   ```
4. **Configure** on the Pi:
   ```bash
   ssh pi sudo nano /etc/menubot/env
   ```
   ```
   TELEGRAM_BOT_TOKEN=123456789:AA...
   TELEGRAM_CHAT_ID=-1001234567890
   ```
   Then run `./deploy.sh pi` once more — the timers are only enabled when the config has no placeholder values.
5. **Test**:
   ```bash
   ssh pi /usr/local/bin/menubot --dry-run daily
   ssh pi sudo systemctl start menubot-daily.service
   ```

## Operations

| What | Command |
|---|---|
| Next runs | `ssh pi systemctl list-timers 'menubot*'` |
| Logs | `ssh pi journalctl -u menubot-daily -u menubot-weekly -u 'menubot-failure@*' --since today` |
| Update | `./deploy.sh pi` (keeps `/etc/menubot/env`) |
| State | `/var/lib/menubot/state.json` |

If fetching or parsing fails, systemd retries 3× (5 min apart) and then posts one ⚠️ notice per day to the chat.

## What is installed where on the Pi

| Path | What | Owner / mode | Written |
|---|---|---|---|
| `/usr/local/bin/menubot` | the program (single static binary, ~7 MB) | root, `755` | every deploy |
| `/etc/systemd/system/menubot-daily.service` | runs `menubot daily` | root, `644` | every deploy |
| `/etc/systemd/system/menubot-daily.timer` | Mon–Fri 10:00 Europe/Berlin | root, `644` | every deploy |
| `/etc/systemd/system/menubot-weekly.service` | runs `menubot weekly` | root, `644` | every deploy |
| `/etc/systemd/system/menubot-weekly.timer` | Fri 15:00–20:00, hourly | root, `644` | every deploy |
| `/etc/systemd/system/menubot-failure@.service` | posts the ⚠️ notice | root, `644` | every deploy |
| `/etc/menubot/env` | bot token and chat ID | root, `600` | first deploy only, never overwritten |
| `/var/lib/menubot/state.json` | last weekly menu sent, last ⚠️ date | user `menubot` | by systemd on first run |
| user `menubot` | system user the services run as (no login, no home) | – | first deploy |

Also:
- **Timer symlinks:** `systemctl enable` creates them in `/etc/systemd/system/timers.target.wants/`.
- **Time sync:** `systemd-time-wait-sync.service` is enabled so that runs after boot wait for a real clock. The Pi has no RTC.
- **Temporary files:** `/tmp/menubot-deploy/` exists only during a deploy.
- **Logs:** they go to the systemd journal (`journalctl`), not to a file.

Check everything at once:

```bash
ssh pi 'ls -l /usr/local/bin/menubot /etc/systemd/system/menubot-* /etc/menubot/ /var/lib/menubot/'
```

## Uninstall

```bash
ssh -t pi 'sudo systemctl disable --now menubot-daily.timer menubot-weekly.timer \
  && sudo rm -f /usr/local/bin/menubot /etc/systemd/system/menubot-* \
  && sudo rm -rf /etc/menubot /var/lib/menubot \
  && sudo systemctl daemon-reload \
  && sudo userdel menubot'
```

This leaves `systemd-time-wait-sync.service` enabled, which is harmless. To turn it off as well:
`ssh pi sudo systemctl disable systemd-time-wait-sync.service`.
