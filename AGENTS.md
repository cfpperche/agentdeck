# AgentDeck

The web cockpit for local AI coding agents. Go backend + embedded React SPA
that wraps agent CLIs (Claude Code, Codex, Grok, Pi, OpenCode) with persistent
sessions and SSE streaming. See `README.md` for the full picture.

Repo language is **English**: docs, code, comments, commits, issues.

## Commands

```bash
make build     # web UI + single binary → bin/agentdeck
make dev-go    # build & run the Go server (https://localhost:8444)
make dev       # run the legacy Phase-0 Python server
make web       # frontend dev server with hot reload (proxies /api)
make test      # go test ./... (deterministic, fake agents, no tokens)
make lint      # go vet ./...
make clean     # remove artifacts (restores web/dist stub)
```

- Go 1.22+ (`go.mod`), Node 22+ for the web build (`web/`). The SPA is
  plain JSX today; the TypeScript migration promised by ADR-0003 is still
  pending — do not start it unprompted.
- Server listens on `https://localhost:8444` with an auto-generated
  self-signed cert. Web tests with a browser must ignore cert errors
  (see the agent-browser skill). `AGENTDECK_INSECURE=1` disables TLS for
  dev/behind-proxy setups.

### Running against fake agents (biggest dev lever)

Point any agent slot at a fake via the env override — no tokens, no auth:

```bash
AGENTDECK_BIN_CLAUDE=$PWD/tests/fakes/fake-claude ./bin/agentdeck
```

The fakes emit the recorded JSONL of each CLI; the whole UI/backend can be
exercised end-to-end deterministically. This is the same mechanism the unit
tests use (`AGENTDECK_BIN_*`).

## Architecture

```
main.go            entrypoint: flags, TLS setup, go:embed of web/dist
internal/agent/    Adapter interface + registry + per-agent event parsers
internal/runner/   subprocess lifecycle: spawn, stream JSONL, kill
internal/store/    SQLite persistence (sessions, history)
internal/server/   HTTP handlers + SSE wiring
internal/config/   flags/env configuration
web/               React SPA (Vite + Tailwind), plain JS
legacy/            Phase-0 Python prototype — frozen, port on demand
tests/fakes/       fake agent scripts that emit real JSONL shapes
```

Invariants — respect these in every change:

- The server **never** talks to subprocesses directly; only through an
  `Adapter` (`internal/agent/agent.go`).
- One adapter = one command builder + one event parser. Adding an agent
  never touches server code.
- Parsers normalize every agent's wire format into the `Event` kinds
  (`ref`, `text`, `tool`, `final`, `error`).
- The frontend only knows the HTTP contract; keep `web/src/api.js` in
  sync with handler changes.
- `docs/adr/` records the big decisions — read the relevant ADR before
  proposing alternatives, and open an issue before non-trivial work.

## Testing (TDD)

Tests run offline against fake agents (`tests/fakes/`) — deterministic,
zero tokens, never hit real CLIs.

- Porting/adding an adapter: write the fake first, then the parser, then
  the adapter.
- Bug fixes come with a regression test that fails before the fix.
- `make test && make lint` must pass before any commit.

## Web UI verification

When a change touches the UI (`web/src/`), verify it in a real browser
with the **agent-browser** skill (`.pi/skills/agent-browser/`):
`make dev-go`, then open `https://localhost:8444` with
`--ignore-https-errors`, act via `snapshot` refs, confirm with a
`screenshot`. Prefer this over describing what the code "should" render.

## Keeping docs in sync (mandatory)

Documentation is part of the change, not an afterthought. A PR that changes
behavior and not docs is incomplete:

- **Any user-visible change** → add a line under `[Unreleased]` in
  `CHANGELOG.md` (Keep a Changelog format).
- **New/changed command, env var or architecture piece** → update this file
  (`AGENTS.md`) in the same commit.
- **User-facing install/usage change** → update `README.md`.
- **Decision changes** → do not rewrite accepted ADRs; write a new one that
  supersedes the old (mark the old `Superseded by ADR-XXXX`).
- **Session boundaries** → if you hand off mid-task, update
  `docs/HANDOFF.md` so the next agent (or human) can pick up cold.

Rule of thumb: if a fresh clone + `AGENTS.md` + `docs/HANDOFF.md` wouldn't
let you reproduce your own workflow, the docs are behind.

## Conventions

- Conventional commits: `feat:`, `fix:`, `docs:`, `refactor:` …
- Small PRs, one logical change; main stays green.
- Never commit secrets or agent credentials. Runtime data (`data/`,
  `cert.pem`, `key.pem`) is gitignored — keep it that way.
- Security-sensitive reports follow `docs/SECURITY.md`, never public
  issues.
