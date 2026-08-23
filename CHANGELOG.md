# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [SemVer](https://semver.org/). The repo language is English.

## [Unreleased]

### Added
- **Permission banner v2** (G7, issue #7): pending approvals are a
  QUEUE (counter "1 of N" when multiple, late-subscriber SSE snapshot
  replays the whole queue), and Allow can carry **edited input** —
  "edit input" opens a JSON textarea; "Allow with edits" sends it as
  `updatedInput`, which the agent executes (validated end-to-end:
  edited command reached the fake SDK's executor).
- Critical shim fix: stdin is now parsed **by line** (readline) —
  `for await (chunk)` silently dropped messages when the Go writer
  coalesced multiple JSON messages into one pipe chunk (found via a
  reproduced race: fast control_responses glued to the user message).
- **Claude driver via Agent SDK** (ADR-0005, issue #5): a vendored
  Node shim (`agent-sdk-shim/`) drives the real claude through
  `@anthropic-ai/claude-agent-sdk` while speaking AgentDeck's existing
  wire protocol — unlocking REAL permission round-trips the CLI's bare
  `-p` never emits (validated live: `manual` mode + canUseTool →
  permission arrives as a control_request with tool+input → allow →
  executed). The Go adapter auto-detects the shim (node + package) and
  falls back to CLI stdio otherwise; restart passes the SDK session ref.
  Deterministic contract tests via a fake SDK (memory across turns,
  deny, allow) — zero tokens in CI; live validation recorded in
  `tests/spikes/`.
- **Settings route** (`/settings`, deep-linkable, back-button safe):
  Appearance section with System / Dark / Light theme cards (instant
  apply, current resolved theme shown on System) + About section with
  live server info (version, execution mode). Gear icon in the sidebar
  footer next to the quick theme toggle. Route follows the benchmark
  pattern (t3code `routes/settings.tsx` + panels).
- **Automatic TLS renewal**: `setup-cert.sh --check <days>` (no-op when
  healthy, unattended re-issuance otherwise — the trusted CA outlives
  leafs, so no UAC/browser action is ever needed again), systemd units
  shipped in `scripts/systemd/` with `install-systemd.sh` (server unit +
  weekly renewal timer), and a boot-time expiry warning in the binary.
- `scripts/setup-cert.sh`: environment-aware TLS provisioning — installs
  mkcert if missing, discovers SANs (localhost/LAN/tailscale, IPv6 and
  docker bridges filtered), issues the server cert, imports the CA into
  the Windows trust store from WSL (UAC, idempotent), restarts the
  service; `--ios` serves the CA on the tailnet with a QR code.
  CI now syntax-checks shell scripts.
- **URL-routed sessions** (G4, issue #4): sessions live at `/s/<id>` —
  back/forward and reload work; legacy `?s=` deep-links normalize in
  place. (History API; the Go SPA fallback serves the app on any path.)
- **Composer draft persistence** (G2, issue #2): per-session drafts in
  localStorage — survive reloads and session switches, cleared on send.
- **Session state machine** (`idle | running | waiting`) — the SSE
  `state` event carries `status` (G1, issue #1). `waiting` = a turn is
  in flight AND the agent asked for approval; late subscribers get the
  pending permission replayed on SSE connect. UI header shows
  waiting/running distinctly.
- **Message queueing / steering** (G3, issue #3): sending while a turn
  is in flight returns `202 {"queued":true}` and the message is
  delivered automatically at turn end (cap 5, then 409).
  `POST /api/sessions/{id}/queue/cancel` discards the queue; the UI
  shows a queued tag and a cancel affordance. Queued messages persist
  to history immediately (visible while waiting); an AgentDeck
  restart mid-queue drops undelivered ones (documented limitation).
- Benchmark-first workflow: t3code and paseo adopted as reference
  projects (AGENTS.md); first study (sessions & composer) with a
  13-gap adversarial comparison → 8 tracked issues (#1–#8).
- Self-signed certificate now covers localhost, LAN and tailnet IPs
  (all local IPv4 SANs) — trusting it once in the OS removes browser
  warnings on every access route (incl. VS Code Simple Browser, which
  has no cert bypass button).
- **Runner v2 / live sessions (ADR-0004, tier 1)**: tier-1 agents
  (claude) now run as persistent bidirectional processes — the web UI
  is a native client of the agent protocol, like the TUIs.
  - In-process conversation memory across turns (no resume dance);
    transparent restart-with-ref when the agent process dies.
  - Permission round-trip: agents' `control_request` events surface in
    the UI as Allow/Deny buttons (`POST /api/sessions/{id}/control`).
  - Fake claude gained a bidirectional `live` mode (stdin/stdout JSONL)
    powering deterministic tests: memory, permission allow/deny,
    crash-restart.
  - One-shot spawn remains the fallback tier for other agents
    (`Registry.DisableLive` for tests/config).
- Go implementation (Phase 1): single binary embedding the React SPA
  (`go:embed`), self-signed TLS generated natively (no openssl).
  - `internal/agent`: Adapter interface + registry with env overrides
    (`AGENTDECK_BIN_<id>`) and event parsers for claude / codex / grok /
    pi / opencode, unit-tested against JSONL recorded from the real CLIs.
  - `internal/store`: SQLite (WAL), schema-identical to the Phase-0
    Python server (shared data dir), ANSI-safe sidebar previews.
  - `internal/runner`: subprocess lifecycle with merged stderr
    (explicit `os.Pipe`), one process per session (409 on busy),
    stop-persists-partial, process-group kill on stop.
  - `internal/server`: HTTP API + SSE, contract-identical to the
    Phase-0 surface (parity suite in `server_test.go`).
  - `internal/config`: execution-mode feature flag (ADR-0002):
    `AGENTDECK_MODE=auto|personal|dedicated`; `AGENTDECK_INSECURE=1`
    disables TLS.
- `tests/fakes/`: fake agent binaries powering deterministic, offline,
  zero-token tests.
- `tests/spikes/`: evidence scripts for architecture decisions (see
  ADR-0004 — persistent bidirectional agent processes proven against
  the real claude CLI; tmux integration discarded).
- `.pi/skills/agent-browser/`: skill for verifying the web UI with a
  real browser (tested against the CLI, includes stale-ref and zombie
  discipline, install instructions and fallback).
- Repo foundation: `AGENTS.md` (agent handbook + invariants), ADRs
  0001–0003, `docs/SECURITY.md` with threat model, CONTRIBUTING with
  the TDD workflow, issue/PR templates, CI (Go + web + legacy).
- `docs/HANDOFF.md`: living state-of-the-project doc for cold starts
  and agent-to-agent handoff.
- `CHANGELOG.md` (this file) with a mandatory docs-sync policy
  (see `AGENTS.md`).

### Fixed
- Web app crashed on load (`permission is not defined`) — state and
  SSE handler for the approval banner were lost in an edit chain;
  restored. Root-cause amplifier: a stale agentdeck process (old
  build + old cert) was holding :8444 while the service ran elsewhere,
  serving a broken embedded UI.
- AgentDeck now ships a **systemd user unit** (`~/.config/systemd/user/
  agentdeck.service`): single supervised process, restart-on-failure,
  survives shell exit and reboots. (Manual: `systemctl --user start
  agentdeck`; logs: `journalctl --user -u agentdeck -f`.)
- Runner: killing an agent now kills its **process group** — children
  inheriting the output pipe kept the scanner blocked after stop
  (caught by CI, `TestStopPersistsPartial`).
- Sidebar previews: ANSI escapes and ASCII-art banners are stripped;
  box-drawing lines never become previews.

### Removed
- (Nothing yet — Phase-0 Python stays in `legacy/` until parity is
  proven in the field.)

[Unreleased]: https://github.com/cfpperche/agentdeck/commits/main
