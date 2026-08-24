# 2026-08-24 — PiCode composer statusline

## Why

Pi's TUI footer (cwd, git, context %, tokens, cost) is the user's
session pulse. PiCode lifts that into the composer so managed (RPC)
mode is not a black box. AgentDeck chat is also protocol-first; we
want the same pulse, starting with pi.

## Receipts (`~/picode`)

- `internal/session/status.go` — `BuildBar(cwd, sessionPath, window)`
  reads git + scans the pi JSONL (`type=message` → `message.usage`)
- Session files: `~/.pi/agent/sessions/<DirName(cwd)>/*.jsonl`
  (`DirName` slashes → dashes, wrapped in `--`)
- `GET /api/workspaces/{id}/status`
- UI: `web/src/lib/statusbar.js` + `ComposerStatus.jsx` — slash-separated
  segments, context as a 80px meter (ok/warn/bad at 70/90%)
- Poll: on workspace change, then every 15s while managed

## Take

Same Bar shape and segment rules. AgentDeck endpoint is per session
(`GET /api/sessions/{id}/status`). Pi usage comes from the latest
JSONL for that session's cwd. Other runtimes get cwd+git immediately;
their usage scanners come later.

## Refuse

Scraping the TUI footer. Dual-write into pi session files.
