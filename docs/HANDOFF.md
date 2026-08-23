# Handoff — state of the project

> **Purpose:** let any agent (or human) pick this project up **cold** —
> where it stands, what is true today, what is next, and where the
> bodies are buried. Update this file when you hand off mid-task or
> finish a phase. Companion to `AGENTS.md` (conventions/invariants);
> this file is the *snapshot*, that one is the *law*.
>
> Last updated: 2026-08-23 (end of Phase 1)

## Where things stand

**Phase 1 is complete and green.** The Go port is the primary
implementation; the React SPA is embedded in the binary; CI runs
Go (vet + tests + build + artifact), web build and legacy syntax on
every push. The Python prototype still runs from `legacy/` and shares
the SQLite schema — it stays until parity is proven in the field.

- Working server: `make build && ./bin/agentdeck` → `https://localhost:8444`
- Tests: `make test` — offline, deterministic, fake agents (`tests/fakes/`)
- Biggest dev lever: run the real server against fakes —
  `AGENTDECK_BIN_CLAUDE=$PWD/tests/fakes/fake-claude ./bin/agentdeck`

## Verified facts (do not re-derive)

- Agent CLIs on the dev machine: claude, codex, grok, pi, opencode —
  all have headless modes; their JSONL shapes are recorded in
  `internal/agent/agent_test.go` and mirrored by the fakes.
- grok CLI is a fork of opencode's (same flags family:
  `--resume`, `--continue`, `--output-format streaming-json`).
- opencode requires a funded provider (z.ai wallet) — its adapter is
  implemented and parses, but runtime errors surface as task errors.
- The API contract = `internal/server/server_test.go` (parity with the
  Python prototype). Frontend consumes exactly that surface
  (`web/src/api.js`).

## Bodies buried (war stories — read before touching these areas)

1. **Subprocess kill**: `CommandContext` kills only the parent; agent
   children inherit the output pipe and block the scanner forever.
   Fix in place: `Setpgid` + group SIGKILL via `cmd.Cancel` + `WaitDelay`.
   Never simplify this away.
2. **stderr is where agents diagnose** ("No session found for current
   directory"): it is merged into the parse stream via an explicit
   `os.Pipe`. `StdoutPipe()` + `cmd.Stderr = cmd.Stdout` silently
   discards stderr (assigns nil) — known Go trap, do not "clean up".
3. **Session continuity per agent** (resume chains): claude/codex/pi/
   opencode resume by native id (`agent_ref` in the sessions table);
   grok resumes by id with `--continue` per-cwd as fallback — which
   FAILS on a fresh cwd. Hence `hasHistory` (assistant already
   replied) is computed **before** inserting the user message. Order
   matters; see `runner.Send`.
4. **Preview sanitization**: banners (ANSI art, box drawing) must never
   reach the sidebar. `store.CleanPreview` + tests own this.

## Next steps (Phase 2 — not started)

- [ ] `install.sh` (curl | sh) + Homebrew tap
- [ ] Tagged releases with SHA256SUMS (adapt the workflow from
      cfpperche/aiagent-linux, including changelog-sourced notes)
- [ ] Execution-mode badge in the UI (consume `/api/server-info`)
- [ ] systemd unit + docs for the `dedicated` mode via
      [aiagent-linux](https://github.com/cfpperche/aiagent-linux)
- [ ] Phase 3 hardening: token auth, rate limiting, metrics

## Open questions

- TypeScript migration of `web/` (promised by ADR-0003) — when, and
  big-bang or incremental?
- Whether `legacy/` removal happens at parity-in-the-field or at a
  fixed date.
