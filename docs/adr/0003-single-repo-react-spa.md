# ADR-0003: Single repository, React SPA frontend

- **Status:** accepted
- **Date:** 2026-08-23

## Context

AgentDeck has two codebases: a backend and a web SPA, tightly coupled by
an API contract (sessions, messages, SSE events). Decisions needed:
one repo or many? which frontend framework?

## Decision

**Single repository** (Go module at the root), frontend in `web/` built
with **React + TypeScript + Vite + Tailwind**.

## Rationale

*Monorepo:* contract changes ship as atomic PRs (backend + frontend +
tests in one review); one issue tracker and one clone for contributors;
releases are a single artifact (the binary embeds `web/dist`).

*React over alternatives:*

- The prototype used Preact — API-compatible, but React's ecosystem,
  hiring pool and contributor familiarity win for an open-source project.
- SSR frameworks (Next/Remix) are wrong here: the SPA is embedded in the
  binary and served statically; there is no server-rendering need.
- Svelte/Solid would be rewrites for taste, not need.

## Consequences

- `go install github.com/cfpperche/agentdeck@latest` works because the
  module lives at the root.
- CI has two jobs (Go, web) on the same PR.
- Boundary rules: `internal/agent/` defines the adapter interface;
  `internal/server/` never touches subprocesses directly; `web/` only
  knows the HTTP contract. TypeScript types mirror it and are validated
  in CI.
