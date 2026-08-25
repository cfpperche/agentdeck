# Study: PiCode conversation rail

- **Date:** 2026-08-24
- **Scope:** replace the conversation scrollbar with a turn rail.
- **Receipts:** file paths below.

## PiCode

- `web/src/lib/rail.js` — `railAnchors(items)` keeps user/agent text
  blocks, skips sys/tool/empty, truncates preview to 140 chars.
- `web/src/components/ConversationRail.jsx` — shown only when there
  are ≥2 anchors **and** the list overflows. Toggles `.with-rail` on
  the scroller (hides native scrollbar). Chevrons page ±85% of the
  viewport. Ticks jump to `[data-rail]`. Hover opens `.rail-pop`.
  Active tick = closest block to ~25% from the top.
- `web/src/styles/app.css` — `.conv-rail` is `absolute; right: 8px;
  top: 16px; bottom: 180px` (sits above the overlay composer).
  Mobile (`max-width: 700px`) hides the rail and restores the
  native scrollbar.
- `ChatSurface.jsx` mounts the rail as a sibling of `.conversation`
  inside `.chat-body` (same stacking context as the composer).

## AgentDeck

One `Chat` surface is shared by all five runtimes (claude / codex /
grok / pi / opencode). A single rail on that surface covers every
runtime tab. Actor label is the runtime (`Claude`, `Codex`, …) vs
`You`. Anchors are persisted messages (`role` + `content`) plus the
in-flight stream.
