#!/usr/bin/env bash
# Runs on the Pi as root, from the directory deploy.sh copied the files to.
set -euo pipefail

src="$(cd "$(dirname "$0")" && pwd)"

id menubot >/dev/null 2>&1 \
  || useradd --system --no-create-home --shell /usr/sbin/nologin menubot

install -m 755 "$src/menubot" /usr/local/bin/menubot
install -m 644 "$src"/*.service "$src"/*.timer /etc/systemd/system/

install -d -m 755 /etc/menubot
if [ ! -f /etc/menubot/env ]; then
  install -m 600 "$src/env.example" /etc/menubot/env
fi

# Let time-sync.target wait for a real NTP sync (the Pi has no RTC).
systemctl enable systemd-time-wait-sync.service

# The failure template is checked indirectly through OnFailure=.
systemd-analyze verify \
  /etc/systemd/system/menubot-daily.service /etc/systemd/system/menubot-daily.timer \
  /etc/systemd/system/menubot-weekly.service /etc/systemd/system/menubot-weekly.timer
systemctl daemon-reload
rm -rf "$src"

# Do not start the timers while the config still has placeholder values.
if grep -q 'replace-me' /etc/menubot/env; then
  echo ">>> Fill in /etc/menubot/env (TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID),"
  echo ">>> then run ./deploy.sh again to enable the timers."
  exit 0
fi

systemctl enable --now menubot-daily.timer menubot-weekly.timer
systemctl list-timers 'menubot*' --no-pager
