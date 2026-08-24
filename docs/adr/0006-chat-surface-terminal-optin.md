# ADR-0006: Chat is the conversation surface; terminal is an opt-in panel

Date: 2026-08-24
Status: Accepted

## Context

Dogfooding a peer cockpit showed the failure mode: a terminal panel open
by default, empty, eating ~40% of the viewport, with a floating close
button orphaned on the splitter. It read as broken even when it worked.

In our architecture (ADR-0004) the conversation is structured protocol
events (text, tool_use, control_request) rendered as chat — never a TUI
screen-scrape. A raw terminal adds nothing to the conversation itself;
it is an inspection/manual-intervention instrument.

## Decision

1. **Chat is the one and only conversation surface.** All agent
   interaction renders through the structured chat view.
2. **A terminal panel, when built, must be closed by default** — absent
   from layout and visual tree until explicitly opened via a toolbar
   toggle.
3. **When open, the panel owns a real header**: icon + cwd/target label
   + functional close button inside the panel chrome. Close controls
   never float on splitters.
4. Panel state persists per browser (not per session), defaulting to
   closed on first visit.

## Consequences

- New-visit users never see dead chrome.
- The terminal becomes a power-user affordance; its absence cannot
  block any core flow (send/steer/permissions all live in chat).
- If we ever add raw PTY attach (agent-dependent), this ADR governs its
  presentation; protocol-level interaction remains primary.
