# 2026-08-24 — PiCode terminal dock (RPC vs TUI)

## Why this study

User report: in PiCode they can drive Pi and still see the real Pi TUI
in a bottom dock. We were asked to copy that for every AgentDeck
runtime, starting with pi.

Receipts: `~/picode` on this machine (docs/decisions, internal/term,
internal/tmux, web/src/components/TerminalDock.jsx) plus the 2026-08-24
17:57 screenshot (`localhost:8445`, session "QA AgentDeck Ref").

## What the screenshot actually is

The agent badge is `interactive`. The chat empty-state reads
"Agent is running in the terminal. Use the Terminal button to pair
with it." The dock is the genuine Pi TUI (`pi v0.84.3`) inside tmux
(`picode-qa-agentdeck-ref-agent-…`). The composer is painted, but
this session is **not** in managed RPC mode.

## Dual-channel was tried and withdrawn

PiCode ADR-0002 designed *simultaneous* tmux TUI + `pi --mode rpc`
"over the same session state". ADR-0006 (2026-08-24) **amends** that:

> pi session JSONL files are append-only trees owned by the running
> process. Two live processes on one session file means two concurrent
> appenders — corruption risk.

So an agent is in exactly one mode:

| Mode | Process | Chat | TUI |
|---|---|---|---|
| `interactive` | `pi` in tmux | banner only | full |
| `managed` | `pi --mode rpc` | full | — |

Starting one stops the other. Closing the dock **detaches** the
xterm attach (`tmux attach` PTY dies); the tmux session keeps running.

## Mechanism we can take

- Dock closed by default; opens only from an explicit Terminal control
  (matches AgentDeck ADR-0006).
- Real header (`Terminal · <name>` + maximize + ×). × hides, does not
  kill. Never an X on the splitter.
- WebSocket ↔ PTY: binary frames = bytes, text frames = `{type:resize}`.
- `tmux new-session -d` owns the agent; each browser attach is a
  short-lived `tmux attach-session`. Tab close ≠ agent death.
- xterm.js + FitAddon, dark theme, 10k scrollback.

## What we refuse

- Simultaneous RPC + TUI writers on the same pi session (PiCode
  already burned this).
- Screen-scraping the TUI as the conversation (AgentDeck ADR-0004).
- tmux send-keys as a control channel (PiCode ADR-0002 rejected it).

## AgentDeck mapping

Chat stays the conversation (protocol drivers we just finished).
Terminal is an exclusive inspection/TUI surface: opening it stops the
live protocol process for that session; sending from chat stops the
TUI and resumes the protocol (auto-pair). Same exclusive rule for
every runtime that has a TUI binary (pi, grok, claude, codex,
opencode).
