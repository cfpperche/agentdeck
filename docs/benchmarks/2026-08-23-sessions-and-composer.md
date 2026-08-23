# Study: session management & composer — t3code vs paseo vs AgentDeck

- **Date:** 2026-08-23
- **Sources:** shallow clones of both repos at HEAD (receipts are file
  paths; verify before relying on specifics)
- **Scope:** how sessions are created/controlled, and the composer

## 1. Session management

### t3code — normalized runtime over SDKs/protocols

Sessions are **threads** with explicit lifecycle, driven by a normalized
event model (`packages/contracts/src/providerRuntime.ts`):

- `RuntimeEventRawSource` unifies per-provider wire formats:
  `codex.app-server.notification`, `claude.sdk.message`,
  **`claude.sdk.permission`**, `codex.sdk.thread-event`,
  `opencode.sdk.event`, `acp.jsonrpc`.
  → They do NOT shell out to CLIs and parse stdout: they embed the
  **agent SDKs** (Claude Agent SDK, Codex SDK, OpenCode SDK) and speak
  **ACP** (Agent Client Protocol, `packages/effect-acp/`).
- `RuntimeSessionState`: `starting → ready → running → waiting →
  stopped | error`. **`waiting` is a first-class state** (agent paused
  on user input — permissions/questions). Our model is a `running`
  boolean; we cannot even represent that.
- Identity model separates **ThreadId / TurnId / RuntimeTaskId /
  ProviderInstanceId** — threads contain turns contain tasks; a session
  can host parallel agent tasks. We conflate message ≈ turn, and one
  agent process per session.
- Extras observed: git **worktree per task** (`worktreeCleanup.ts`),
  orchestration with recovery (`orchestrationEventEffects.ts`,
  `orchestrationRecovery.ts`), plan approval flow (`proposedPlan.ts`).

### paseo — daemon + PTY terminals + agent-hooks + task graph

- A local **daemon** (`packages/server/`) manages agents; all clients
  (desktop/mobile/web/CLI) connect to it — same server/clients split
  we have, but with an **E2E-encrypted relay + device pairing**
  (`packages/relay/`, "Pair Device") instead of assuming Tailscale.
- Agent control runs through **PTY terminals with provider hooks**
  (`packages/server/src/terminal/agent-hooks/{claude,codex}/…`): the
  agent TUI runs in a terminal; hooks intercept provider-native events
  (settings-aware, `claude-settings.ts`). This is the "keep the TUI
  alive, observe its protocol" bet — different from ours (process +
  stdin/stdout JSONL) and from t3code (SDK embedding).
- Sessions are **tasks** with graph semantics:
  `packages/server/src/tasks/{task-graph,task-document,execution-order,
  task-store}.ts` — dependencies, ordering, parallel execution across
  agents/machines ("Run agents in parallel on your own machines").
- Voice mode, zero telemetry.

### Convergence worth noticing

All three projects (them and us, via ADR-0004) reject screen-scraping
the TUI. They differ in which channel they use *under* it: SDKs
(t3code), PTY+hooks (paseo), CLI stdio protocols (us, today).

## 2. Composer

### t3code (`apps/web/src/`)

- `composer-logic.ts` + `composerDraftStore` — **drafts persist** across
  reloads/routes.
- `promptStashStore.ts` — save/reuse prompts.
- `composer-editor-mentions.ts` — **@-mentions** (files, agents) inside
  the composer.
- `pendingUserInput.ts` — first-class UX for answering agent questions/
  permissions (including editing the tool input, not just allow/deny).
- `modelSelection.ts` — model/agent selection per thread; switchable.
- `threadRoutes.ts` — threads are **URL-routed**
  (`/_chat/$environmentId/$threadId`): deep-linkable, back-button works.

### paseo (`packages/app/` + mobile)

- Composer tied to the task graph: create task, assign agent/model,
  queue it. Native apps add **voice input**; mobile composer built for
  thumb use with quick actions.

### AgentDeck today

Single textarea, Enter-to-send, Shift+Enter newline. Draft lost on
navigation/reload. No mentions, no stash, no queueing (busy → 409
error toast). Permission banner v1 (allow/deny, no input editing).
Agent fixed per session; `?s=` deep-link only, no history routing.

## 3. Adversarial gap list (vs AgentDeck)

Priority: P0 dogfood blockers · P1 leverage · P2 strategic.

| # | Gap | Priority | Receipt |
|---|---|---|---|
| G1 | Session **state machine** with `waiting` (permission pending) — today `running:bool` cannot represent it | P0 | t3code `providerRuntime.ts` |
| G2 | **Draft persistence** — composer text survives reload/route switch | P0 | t3code `composerDraftStore` |
| G3 | **Message queueing** instead of 409-on-busy (steering: next message delivered when turn ends) | P0 | pi RPC has `queue_update`; paseo task queue |
| G4 | **URL-routed sessions** (back button, shareable, reload-safe) | P0 | t3code `threadRoutes.ts` |
| G5 | **Claude Agent SDK** for tier-1 claude: unlocks real permission events (`claude.sdk.permission`) — the exact blocker we hit with the CLI | P1 | t3code `RuntimeEventRawSource` |
| G6 | **Typed contract sharing** web↔server (generated TS from Go) | P1 | t3code `packages/contracts` |
| G7 | **@-mentions** (attach files / reference agents) in composer | P1 | t3code `composer-editor-mentions` |
| G8 | **Turn/Task identity model** (ThreadId/TurnId/TaskId) to enable parallel tasks | P1 | t3code `baseSchemas.ts` |
| G9 | **Permission request queue + input editing** (banner v2) | P1 | t3code `pendingUserInput.ts` |
| G10 | **ACP adapter** — one protocol, many agents (Claude/Gemini/others ship ACP) | P2 | t3code `effect-acp` |
| G11 | **Git worktree isolation** per task | P2 | t3code `worktreeCleanup.ts` |
| G12 | **Device pairing relay** (E2EE) beyond Tailscale assumption | P2 | paseo `packages/relay` |
| G13 | **Web tests** — our repo has zero (t3code colocates `*.test.ts` everywhere) | P2 (rising) | both |

### Postscript (G7 deep-dive, 2026-08-24): pending-user-input UX

Receipts from t3code:
- `apps/web/src/pendingUserInput.ts`: a *progress* model over a list of
  `UserInputQuestion`s — activeQuestion, draft answer (customAnswer /
  selectedOptionLabels), answeredCount/isLast/canAdvance. Editing is a
  first-class draft, not an afterthought.
- `ComposerPendingUserInputPanel.tsx`: the pending-input UI is ANCHORED
  TO THE COMPOSER (where the user's attention already is), not a
  floating banner; ChatComposer integrates it.
- Contract shape (`UserInputQuestion`): options[], multiSelect,
  Requested/Resolved payload pairs in `packages/contracts`.

Applied to AgentDeck v2: keep our banner position (mobile-first
thumb-zone) but adopt the draft-edit model (Allow with edits via
`updatedInput`), a queue counter with navigation (1/N), and
resolved-state feedback. multiSelect/options do not apply to our
tool-permission flow (single decision), skipped deliberately.

## 4. What we keep (not everything is worth copying)

- Our single-binary Go distribution and embedded UI — neither has that
  (t3code ships via npm/Electron; paseo via downloads/Docker).
- SQLite + filesystem simplicity vs their heavier runtimes.
- Tailscale-first remote access is *simpler* than a pairing relay for
  our current scale — revisit at G12 time.

Decision impact: G1–G4 go to the board now; G5 deserves an ADR
(SDK embedding vs CLI stdio) — evidence says SDK is where the
industry went.
