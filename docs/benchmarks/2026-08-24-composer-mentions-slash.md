# Study: composer @-mentions and slash — PiCode / t3code / AgentDeck

- **Date:** 2026-08-24
- **Scope:** Fase D of the composer — `@file` fuzzy and `/` commands.
  Image paste is deferred (needs a persist endpoint; agents can then
  Read the path).
- **Receipts:** file paths below. t3code mention module is cited from
  the 2026-08-23 study (G7); we do not have a live clone this session.

## PiCode

- `web/src/lib/slash.js` — static catalog, `filterSlash` only when the
  draft **starts with `/`**. Prefix match on id/label.
- `web/src/components/Composer.jsx` — popup above the textarea;
  arrows / Tab / Enter pick, Esc clears. `onSlash(cmd)` to the app.
- `web/src/desktop/App.jsx` `onSlash` — local UI (focus chips, new
  session, rename) vs `POST /api/agents/:id/command` for TUI-native
  commands (`/login`, `/reload`).
- No `@file` picker in the web composer. Mentions are a t3code thing.

## t3code (from 2026-08-23 study)

- `apps/web/src/composer-editor-mentions.ts` — **@-mentions** for files
  and agents inside the composer. Gap **G7 / P1**.

## Cursor (public UX, inference)

- `@` in the chat composer opens a file/symbol picker. We copy the
  trigger and the "insert a path token" outcome, not a reimplementation
  of their index.

## AgentDeck decision

- Slash catalog is **AgentDeck-native** (not pi's `/login`): `/new`,
  `/term`, `/stop`, `/settings`, `/devices`, `/system`. `/term` and
  `/stop` stay in the composer (already have handlers). Overlays ride
  the existing `agentdeck-share` custom-event pattern (`agentdeck-slash`).
- `@` walks the session cwd via `GET /api/fs/files?path=&q=` (skip
  VCS/deps, cap visit/results, stay under `$HOME`). Insert `@rel ` at
  the caret. The agent then Reads the path — no multimodal wire.
- Image paste stays out of this commit.
