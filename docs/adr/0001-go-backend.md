# ADR-0001: Backend language — Go

- **Status:** accepted
- **Date:** 2026-08-23

## Context

AgentDeck's backend is a thin orchestration layer: HTTP + SSE, SQLite,
and spawning agent CLIs as subprocesses while parsing their JSON output.
It needs no AI SDKs — the agents are external processes. The Phase-0
prototype is Python (FastAPI), which validated the architecture fast.

For an open-source tool whose entire value proposition is "runs on *your*
machine", distribution friction is the main adoption cost.

## Decision

Port the backend to **Go**, keeping the current HTTP API as the contract.
The frontend does not change.

## Rationale

- **Distribution**: one static binary, `go install`/`curl | sh`/Homebrew,
  no runtime to install. Benchmarks: lazygit, gh, caddy, PocketBase.
- **Embedded UI**: `go:embed` ships the React build inside the binary —
  one artifact for server + web app (the PocketBase model).
- **Cross-compile**: trivial matrix for linux/macOS/windows/arm.
- **Subprocess + streaming**: native strengths (exec, context cancel).
- **Typed API contract**: generate TypeScript types from Go/OpenAPI.

Alternatives considered:

- **Stay on Python** — fastest to keep iterating, but every user needs a
  Python env; packaging (PyInstaller) is brittle.
- **Bun/TypeScript** — good fit for a JS-heavy team and compiles to
  binaries; chosen against because the backend is smaller than the
  frontend and Go's distribution story (incl. cross-compile) is stronger.

## Consequences

- A port effort (~1.2k LoC), mitigated by parity tests against the
  existing API and fake-agent test doubles (no token spend in CI).
- Python prototype stays in `legacy/` until feature parity, then is
  removed.
