# 2026-08-24 — Cursor as a design reference benchmark

## Why add a closed-source benchmark

Our two existing benchmarks (t3code, paseo) are open code — perfect for
architecture receipts, but both are young products whose UX is still
converging. Cursor is the most polished agent-composer experience
shipping today; millions of users have been trained on its interaction
patterns. For **design, UI/UX and product decisions** it is the
strongest reference available, even though its source is closed.

**Method adjustment:** no file-path receipts are possible. Receipts are
public artifacts instead: docs.cursor.com pages, the official
changelog (cursor.com/changelog), and shipped behavior verifiable in
the app. Architecture claims are marked as inference where we cannot
verify them.

## What we study in Cursor

### 1. The composer is the product (design)

One input owns everything. The composer is a single rounded field with
a thin control strip — model picker bottom-left/right, mode selector,
attach controls — and it never leaves the screen. Contextual controls
appear only when relevant (e.g. plan-mode affordances). Nothing about
the conversation lives outside it except the transcript above.

**Take-away for AgentDeck:** our composer must grow *into* this shape —
one persistent control strip under the textarea, not scattered buttons
around the chat.

### 2. Mode selector as first-class citizen (UX decision)

Agent / Ask / Plan (and formerly Manual/Auto approval inside those) sit
as an explicit, always-visible selector next to the model picker. The
user chooses *how the agent may act* before typing, not after.
Renaming "permission levels" into plain-language modes was a product
decision: users understand "Plan" better than "read-only tools".

**Take-away:** expose permissionMode in the composer as named modes
(our wire already speaks manual/acceptEdits/plan/bypassPermissions —
ADR-0005), labeled for humans, not as settings buried in a panel.

### 3. Model picker with reasoning variants (UI)

Model dropdown groups by provider/family; reasoning-effort variants
(e.g. low/medium/high thinking for GPT-class models, Max mode for extra
budget) appear as sub-options rather than separate top-level models.
Selection persists across sessions.

**Take-away:** mirrors paseo's `thinkingOptions[]` +
`defaultThinkingOptionId` on `AgentModelDefinition` (see the paseo
study). Our composer should render models with nested thinking
variants, persisting the last choice per runtime.

### 4. @-mentions and context injection (interaction)

`@Files`, `@Folders`, `@Docs`, `@Code` bring context into the prompt
with fuzzy search in an inline popup; selected context shows as chips
inside the composer. Images paste directly into the field.

**Take-away:** same direction as t3code's composer triggers
(`path | slash-command | slash-model | skill`). We adopt the pattern in
phases (files first, skills/docs later).

### 5. Checkpoints and rewind (product decision)

Every agent edit snapshots state; "Restore checkpoint" appears on
hover over earlier user messages. This is a trust feature: aggressive
agents are acceptable when undo exists.

**Take-away (deferred):** valuable but heavy (worktree/git
integration). Recorded here so a future ADR can lean on it; not in the
composer scope.

## Architecture notes (inference, unverifiable)

- Composer state survives navigation → likely a store outside the
  route tree (mirrors our tab-mounted sessions keeping composer drafts).
- Model/mode changes apply to the *next* turn without dropping session
  state — consistent with resumable agent processes (our ADR-0004
  restart-with-ref achieves the same for CLIs that need re-spawn).

## Direct influence on AgentDeck

This study feeds the composer plan (2026-08-24): persistent control
strip, named permission modes as a visible selector, model picker with
nested reasoning variants, phased @-mentions/image support.
