# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [SemVer](https://semver.org/). The repo language is English.

## [Unreleased]

### Added
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
- Runner: killing an agent now kills its **process group** — children
  inheriting the output pipe kept the scanner blocked after stop
  (caught by CI, `TestStopPersistsPartial`).
- Sidebar previews: ANSI escapes and ASCII-art banners are stripped;
  box-drawing lines never become previews.

### Removed
- (Nothing yet — Phase-0 Python stays in `legacy/` until parity is
  proven in the field.)

[Unreleased]: https://github.com/cfpperche/agentdeck/commits/main
