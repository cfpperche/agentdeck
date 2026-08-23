#!/usr/bin/env bash
# Install AgentDeck systemd user units (server + cert renewal timer).
# Idempotent: re-run after edits to update the units.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p ~/.config/systemd/user
cp scripts/systemd/agentdeck.service ~/.config/systemd/user/
cp scripts/systemd/agentdeck-cert.service ~/.config/systemd/user/
cp scripts/systemd/agentdeck-cert.timer ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now agentdeck.service agentdeck-cert.timer
echo "✓ agentdeck.service + agentdeck-cert.timer installed and enabled"
echo "  logs: journalctl --user -u agentdeck -f"
echo "  timer: systemctl --user list-timers agentdeck-cert.timer"
