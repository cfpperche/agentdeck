# ADR-0004: Persistent protocol sessions — the web UI as a native agent client

- **Status:** accepted
- **Date:** 2026-08-23
- **Supersedes:** none (evolves the model in ADR-0001; **discards** the
  tmux-console alternative evaluated before it)

## Context

The Phase-1 runner spawns a headless process per message
(`agent -p …`), persisting continuity via each CLI's native resume
(`--resume <ref>`). Two hard limitations follow: agents cannot ask the
user anything mid-task (permission prompts, clarifying questions), and
steering a running agent is impossible.

Two alternatives were evaluated:

1. **tmux-backed consoles** (web terminal attached to TUI sessions):
   rejected — controlling the agent would mean scraping rendered TUI
   frames (ANSI repaint diffs, five different TUI dialects, layouts
   changing every release). That is scraping a website with no DOM.
   **tmux integration is fully discarded.**
2. **Native programmatic protocols**: the TUIs are themselves clients
   of documented underlying protocols. Evidence on this machine:
   `claude --input-format stream-json` (bidirectional JSON on
   stdin/stdout), `pi --mode rpc` (documented JSONL protocol,
   explicitly "for embedding the agent in other applications … or
   custom UIs"), `codex app-server` (JSON-RPC daemon the TUI itself
   connects to), `opencode serve/attach/web` (server + clients).
   This is the same channel the official UIs use — a contract, not a
   scrape.

A spike (`tests/spikes/claude-interactive-spike.py`) proved the model
against the real claude CLI:

- **Persistent process, in-process memory**: second message into the
  same living process recalled "42" planted by the first — no resume
  flag, no restart.
- **Structured events**: tool calls arrive as JSON (`tool_use`
  blocks) — the existing parser tier already consumes this shape.
- Permission round-trip did not trigger under this machine's
  `bypassPermissions` settings; the exact control-request handshake
  (per agent) is flagged for validation during implementation.

## Decision

AgentDeck pivots its session model to **persistent agent processes
speaking each agent's native programmatic protocol**; the web UI
becomes a first-class client of those protocols (like the TUIs are).

Protocol tiers:

| Tier | Agents | Mechanism | Status |
|---|---|---|---|
| 1 — bidirectional JSONL | claude, pi | long-lived process, stdin/stdout JSONL (`stream-json` / `--mode rpc`) | target of runner v2 |
| 2 — server protocols | codex, grok, opencode | `app-server` JSON-RPC; `serve`+`attach` | ported after tier 1 |
| Fallback | any | current spawn-per-message headless (adapters unchanged) | current behavior |

Runner v2 (tier 1) shape:

- One long-lived `SessionRunner` per session: process handle + stdin
  writer + event pump → existing SSE bus → existing UI.
- `POST /api/sessions/{id}/messages` writes a user message to stdin
  (409 semantics preserved while a turn is in flight).
- If the process dies (crash, AgentDeck restart), the runner
  **restarts it resuming the native session ref** — the adapter layer
  already resolves refs; context survives via the agent's own session
  store.
- New event kinds (permission requests, queue/steering updates) map to
  UI affordances (approve/deny buttons, message queueing) — better than
  the TUI on mobile.

## Consequences

- **Gains**: interactive control (answer prompts, steer mid-task),
  in-process context (no fragile resume chains), richer tool events,
  foundation for queueing/automation on a living process.
- **Costs**: AgentDeck's lifecycle now owns agent processes — an
  AgentDeck restart kills them (mitigated: restart-with-ref). Process
  supervision (idle reaping, zombie sweep) becomes runner
  responsibility.
- The permission handshake per agent must be validated empirically
  before promising UI buttons; do not assume the shape across agents.
- tmux is not used anywhere in the architecture; no console mode.
