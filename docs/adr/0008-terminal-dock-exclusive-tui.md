# ADR-0008: Terminal dock — exclusive TUI, never a second writer

Date: 2026-08-24
Status: Accepted
Amends: ADR-0006 (presentation still holds); does not reopen ADR-0004

## Context

PiCode shows the genuine agent TUI in a bottom dock. Their ADR-0002
tried to run that TUI *and* `pi --mode rpc` at once; ADR-0006 withdrew
it because two processes appending the same session file corrupt it.

AgentDeck already made chat the conversation (ADR-0004) and required
any terminal to be opt-in with a real header (ADR-0006). This ADR
decides how the TUI process relates to the protocol process.

## Decision

1. **At most one live agent process per session.** Opening the TUI
   stops the protocol driver; sending from chat stops the TUI and
   resumes the protocol (auto-pair). No concurrent writers.
2. **The dock is an attach, not the process.** `tmux` owns the TUI;
   the browser holds a short-lived `tmux attach` PTY over WebSocket.
   Closing the dock or the tab detaches; the TUI keeps running until
   chat takes over or the session is deleted.
3. **Presentation is still ADR-0006:** closed by default, real header
   (`Terminal · <title>` + maximize + close), never an X on a splitter.
4. **Any runtime with a TUI binary can use the same path.** First
   ship is pi (`pi` with no `--mode rpc`); the argv is per-adapter
   (`Adapter.BuildTUI`).

## Consequences

- Users get the real TUI as a door (login, `/`, ad-hoc) without
  abandoning structured chat.
- Mode switch is visible: while the TUI owns the session, chat shows
  a banner instead of pretending RPC is live.
- tmux becomes a dependency of the dock, not of the conversation.

## Alternatives considered

- Simultaneous RPC + TUI: rejected — PiCode ADR-0006 receipts.
- PTY of the RPC child: rejected — NDJSON, not a TUI.
- Workspace shell only: useful later, not what the screenshot is.
