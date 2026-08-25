# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [SemVer](https://semver.org/). The repo language is English.

## [Unreleased]

### Added
- **Pi composer chips** (PiCode parity): Provider, Model, Thinking,
  Full/Read-only, Prompt/Steer/Follow-up. Live catalog from
  `get_available_models` grouped by provider. Kind is the RPC command;
  Read-only is `--tools read,grep,find,ls` on the next spawn.
- **Conversation rail** (PiCode): when a chat overflows, the native
  scrollbar is replaced by ticks (You vs runtime label) + chevrons +
  hover preview. Shared Chat surface, so all five runtimes get it.
  Mobile keeps the native bar.

### Changed
- **TUI inherits composer chips**: opening Terminal POSTs
  model/thinking/mode (and pi provider/op_mode) and the TUI argv
  gets `--model haiku --permission-mode …` instead of the CLI default
  (Opus 5 / auto).
- **TUI resumes the protocol session**: `claude --resume <id>`,
  `pi --session <jsonl>`, `codex resume <id>`. Opening Terminal no
  longer starts a blank splash next to an existing chat.
- **Chat | Terminal toggle** (no split dock): opening Terminal fills
  the column; Chat returns to the conversation. Exclusive views — the
  squeezed dock + empty chat overlay is gone. ADR-0008 amended.
- **Composer focus**: no blue `:focus-visible` ring on the textarea;
  the card border just darkens (`:focus-within`), like PiCode.
- **Composer overlays the conversation** (PiCode): the message list
  is `inset: 0` so the scrollbar runs to the bottom edge; the
  composer floats on top (`right: 12px` leaves the gutter free) with
  a fade shadow. Last messages pad 220px so they can scroll above it.

### Added
- **Composer Fase D (slash + @file)**: `/` palette (`/new` `/term`
  `/stop` `/settings` `/devices` `/system`) and `@` fuzzy file picker
  over the session cwd (`GET /api/fs/files`). Image paste still open.
- **Composer statusline — OpenCode**: scan
  `~/.local/share/opencode/opencode.db` (`session.directory` = cwd)
  for cost + tokens_input/output/cache. Same Bar as pi. Window 128k.
- **Composer statusline — Claude**: SDK/CLI `result.usage`
  (`input_tokens`, cache read/create, `total_cost_usd`) becomes the
  same usage pulse. Shim emits it; parseClaude also reads usage
  nested on `result` so the CLI fallback works. Window 200k.
- **Composer statusline — Grok**: ACP `_x.ai/session_notification`
  and prompt-result `_meta` (input/output/cache/totalTokens) become
  the same usage pulse. Default window 500k.
- **Composer statusline — Codex**: live tokens from
  `thread/tokenUsage/updated` (window included). Same Bar as pi.
- **Composer statusline (pi first)**: cwd, git branch/dirty, context
  meter, tokens in/out, cache hit and cost — PiCode's Bar, fed by
  `GET /api/sessions/{id}/status`. Pi usage is scanned from the latest
  `~/.pi/agent/sessions/<cwd>/*.jsonl`. Other runtimes get cwd+git now;
  their usage scanners come next.
- **/system route** (PiCode-shaped): Host, Network, Dependencies
  (tmux, mkcert, tailscale, installed agents) and About. Settings
  keeps only Appearance + Server port — version/mode/about moved out.
- **Terminal dock (ADR-0008)**: opt-in xterm.js panel (closed by
  default, real header, × detaches). Opens the runtime's genuine TUI
  in tmux; exclusive with the protocol process (PiCode ADR-0006
  receipts — two writers corrupt pi session files). Send from chat
  auto-pairs back to RPC. All five runtimes that ship a TUI binary
  are wired (`Adapter.BuildTUI`); first dogfood target is pi.
- **codex app-server driver (ADR-0007 tier-1)**: persistent
  `codex app-server --stdio` via `internal/codexbridge`. Live
  catalog from `model/list` (gpt-5.6-terra / 5.5 / 5.4-mini +
  reasoning efforts), mid-session model/effort on `turn/start`,
  streaming `item/agentMessage/delta`, turn completion on
  `turn/completed` (start is ack-only). Approvals map to the
  existing permission banner. Always picks a model from the live
  catalog — the config.toml default (`gpt-5.6-sol`) 400s on
  ChatGPT accounts.
- **grok ACP driver (ADR-0007 tier-2)**: `grok agent stdio` is a
  real ACP server (docs in `~/.grok` + live handshake on 1.0.5).
  Reuses the generic `__acp` bridge; argv is `agent stdio` not
  `acp`. Live catalog from `session/new.models` (grok-4.6 / 4.5),
  mid-session `session/set_model` + `session/set_mode` (thinking
  low…xhigh). Permission prompts ride the existing ACP
  request_permission path. Fallback CLI spawn stays as safety net.
- **pi native driver (ADR-0007 tier-1)**: `internal/pibridge` drives
  `pi --mode rpc` (protocol receipts: paseo providers/pi, verified
  against the installed CLI). Live model catalog via
  get_available_models (composer shows pi's real models, e.g.
  anthropic/claude-fable-5), mid-session set_model +
  set_thinking_level from composer controls, streaming text deltas,
  tool events, and hardened turn completion (agent_end/agent_settled
  dedup; empty provider responses finish as errored turns instead of
  hanging).

- **Open on phone**: QR in the sidebar. `GET /api/share` checks HTTPS,
  bind, reachable IP, cert SAN and mkcert. QR uses the selected LAN or
  this-node Tailscale address (not the Windows host). HTTP `:8471`
  installs the CA (iOS/Android wizard, Safari/Chrome gate, Opening…
  splash). Cert SANs refresh via mkcert + TLS reload — no rebuild.
- **Devices** (`/devices`): host vs other browsers (15s ping, 45s online).

### Fixed
- **Terminal maximize hid nothing**: fullscreen only grew the dock
  under the composer. Maximize now fills the session column and hides
  the composer (PiCode `.dock.maximized` + hidden conversation).
- **Terminal dock never showed the TUI**: two stacked bugs. (1)
  `withWebUI` only forwarded `/api/`, so `/ws/term` was served as the
  SPA HTML. (2) the attach PTY inherited the systemd unit's empty
  TERM, and tmux exited with "terminal does not support clear".
  `/ws/` now reaches the API mux; attach/new-session set
  `TERM=xterm-256color`.
- **Thinking buried in the model menu**: Grok (and any runtime with
  named models + thinking_options) hid Low/Medium/High/Extra high
  inside the model dropdown. Thinking is now a first-class strip
  control with a "Thinking" label in front of its own selector.
- **/devices and /settings trapped the view**: selecting a tab, home or
  a sidebar session no longer leaves those overlay pages stuck over the
  agent view; both panels gained an explicit back button (reachable
  even with zero tabs open).
- **overlay routes stuck in the address bar**: back on Devices/Settings
  used `pushState` with the URL in the unused title slot, and tab
  navigation reused `location.pathname`, so `/devices` and `/settings`
  never left the bar (and a refresh reopened the overlay). Chat now
  always lives at `/`; overlays push `/devices|/settings` (keeping the
  tab query) and back *replaces* them with the chat URL. Opening an
  overlay no longer clears the active tab.

## [0.10.0] - 2026-08-24

### Added
- **ACP driver (ADR-0007, tier-2)**: generic Agent Client Protocol
  client (`internal/acp`, NDJSON JSON-RPC over stdio) + bridge
  subcommand (`agentdeck __acp <agent…>`) translating to the ADR-0004
  wire — the runner needs zero protocol awareness. opencode now runs
  as a persistent process: live model catalog straight from the agent
  (82 options vs 1 curated), mid-session model switching via
  session/set_config_option, streaming, and native resume through
  ACP loadSession. Error results from providers now finish the turn
  (previously left it running forever).
- **Curated catalogs for all five runtimes** (benchmark parity —
  paseo/t3code keep static per-provider catalogs too): codex
  (GPT-5 Codex/GPT-5, reasoning effort, sandbox-mapped modes), grok
  (grok-4.6/4.5 from `grok models`, reasoning-effort), pi
  (documented thinking levels on the default model), opencode
  (--variant). Fallback tier applies controls as real CLI flags
  (`ApplyControls` per adapter), verified against each CLI's --help
  and unit-tested.
- **Composer per runtime, phases A–C (ADR-0006)**: agents report a
  `capabilities` event (models with nested thinking variants, named
  permission modes); the composer renders a Cursor-style control strip
  inside the input box (model picker with search-free popover +
  thinking sub-options, cycling mode chip with colored dot), persists
  the selection per agent, and sends `controls` with each message;
  runner pushes `set_controls` to the live process before delivery and
  the shim applies them to SDK options (proven E2E: model reaches the
  agent). Deep-link fix: `/s/<id>` paths now open the session tab
  instead of falling back to home.
- **Releases + installer**: tag-push workflow cross-compiles
  linux/darwin/windows (amd64+arm64) with embedded UI and publishes
  SHA256SUMS + CHANGELOG-derived notes; `scripts/install.sh` one-liner
  (platform detect, checksum verify, `~/.local/bin`, optional
  `--systemd`). Windows now builds: process-group kill split into
  per-platform helpers (`procs_unix.go` / `procs_windows.go`);
  `--version` flag added.
- ADR-0006: chat is the conversation surface; a terminal panel (when
  built) must be closed by default, toggled from the toolbar, and own
  a real header with a functional close control.
- **Web test suite (vitest + Testing Library)**: 16 tests covering the
  API client contract (request shapes incl. optional cwd/updatedInput,
  SSE parsing with malformed-line tolerance), the theme hook (system
  resolution, persistence, live OS tracking), and the Chat component's
  critical flows (optimistic send, queued tag, permission queue with
  counter + edit-input + allow-with-edits + deny, running/waiting
  badges).
- **Generated API contract types** (`scripts/gen-types.sh` →
  `web/src/api-types.d.ts`): mirrors the Go handler payloads; single
  source of truth for the HTTP surface.
- **Smoke E2E in CI** (`scripts/smoke-e2e.sh`): real binary + fake SDK
  — boot, agent registry, live turn with in-process memory, fs/dirs +
  mkdir (home-scoped), session-with-cwd, port info + range rejection.
- **Port configuration (PiCode-parity mechanism)**: `PortConfig`
  (single or range) — boot tries the range and binds the first free
  port; Settings → Server lets the user move the app to a specific
  port with **bind-new-then-drop-old** semantics (probe-bind 409
  courtesy check, rollback of the setting if the final bind fails,
  202 + UI auto-reconnect to the new port). UI accepts a single port
  (ranges are for AGENTDECK_PORT headless); precedence:
  saved setting > env > default 8444-8454. Ports are not in the cert
  SANs, so mkcert keeps working across moves. /api/server/port
  (GET/PUT) + settings key/value table.
- **User menu (Vercel-style) at the sidebar bottom** (benchmark:
  t3code SidebarChrome + Vercel dashboard): avatar with initials,
  username + execution mode from /api/server-info (now also exposes
  user/host), dropdown opens upward — Settings route, quick theme
  toggle, version/status footer. Closes on outside-click/Escape.
  Replaces the old static "local agents online" bar (the gear button).
- **Create folders from the cwd browser**: `POST /api/fs/mkdir`
  (parents allowed; guardrails: absolute paths only, restricted to the
  user's home, 409 on duplicates, rejects files) + a "new folder" row
  in the directory browser — type a name, create, it becomes the
  selected cwd. Answers the dogfood question "how do I create a new
  folder from the interface?" without leaving the flow.
- **Per-session working directory (cwd)**: the New-session config tab
  gains a "working directory" section — free path input + server-side
  directory browser (directories only, hidden filtered, navigate up/
  into, "use this"). Sessions created with a cwd run the agent in it
  (live process cwd verified via /proc); without one they keep the
  isolated scratch workspace — the safe default (no surprise edits).
  The chat header shows the cwd (basename, full path on hover).
  `GET /api/fs/dirs` powers the browser; POST /sessions validates and
  canonicalizes the path (must be an existing directory).
- Tab titles now sync after each turn (auto-title propagates to open
  tabs via onSessionUpdated → refreshSessions).
- **"New session" is now a configuration TAB**: the sidebar button (and
  the mobile CTA) opens a "New session" tab with the session setup —
  runtime (agent) picker today, designed as the extension point for
  future options (model, workdir, permission mode, ...). Home agent
  chips keep instant-create. The ghost "AgentDeck" label no longer
  renders when zero tabs are open (empty bar, per user report).
- **Editor-style session tabs + home redesign**: sessions open as TABS
  (bar on top, one active, closeable ×, all mounted — switching never
  loses SSE streams, drafts or pending permissions); home `/` is the
  hero (vertically centered agent picker; recent list shown only when
  the sidebar is hidden, e.g. mobile). URLs model the tab state
  (`?tabs=a,b&tab=a`); legacy `/s/<id>` deep-links normalize.
- Chat view extracted to its own component (`chat.jsx`) — the session
  logic is now independent of the shell.
- Review fixes: messages anchored to the bottom of the viewport (no
  dead space under short threads), tool chips show the command detail,
  composer placeholder in English, sidebar hover no longer overlaps
  the timestamp.
- Benchmark postscript: t3code/paseo use route-per-thread instead of
  tabs — tabs chosen deliberately for desktop-primary multi-session
  (documented rationale in docs/benchmarks).
- **Permission banner v2** (G7, issue #7): pending approvals are a
  QUEUE (counter "1 of N" when multiple, late-subscriber SSE snapshot
  replays the whole queue), and Allow can carry **edited input** —
  "edit input" opens a JSON textarea; "Allow with edits" sends it as
  `updatedInput`, which the agent executes (validated end-to-end:
  edited command reached the fake SDK's executor).
- Critical shim fix: stdin is now parsed **by line** (readline) —
  `for await (chunk)` silently dropped messages when the Go writer
  coalesced multiple JSON messages into one pipe chunk (found via a
  reproduced race: fast control_responses glued to the user message).
- **Claude driver via Agent SDK** (ADR-0005, issue #5): a vendored
  Node shim (`agent-sdk-shim/`) drives the real claude through
  `@anthropic-ai/claude-agent-sdk` while speaking AgentDeck's existing
  wire protocol — unlocking REAL permission round-trips the CLI's bare
  `-p` never emits (validated live: `manual` mode + canUseTool →
  permission arrives as a control_request with tool+input → allow →
  executed). The Go adapter auto-detects the shim (node + package) and
  falls back to CLI stdio otherwise; restart passes the SDK session ref.
  Deterministic contract tests via a fake SDK (memory across turns,
  deny, allow) — zero tokens in CI; live validation recorded in
  `tests/spikes/`.
- **Settings route** (`/settings`, deep-linkable, back-button safe):
  Appearance section with System / Dark / Light theme cards (instant
  apply, current resolved theme shown on System) + About section with
  live server info (version, execution mode). Gear icon in the sidebar
  footer next to the quick theme toggle. Route follows the benchmark
  pattern (t3code `routes/settings.tsx` + panels).
- **Automatic TLS renewal**: `setup-cert.sh --check <days>` (no-op when
  healthy, unattended re-issuance otherwise — the trusted CA outlives
  leafs, so no UAC/browser action is ever needed again), systemd units
  shipped in `scripts/systemd/` with `install-systemd.sh` (server unit +
  weekly renewal timer), and a boot-time expiry warning in the binary.
- `scripts/setup-cert.sh`: environment-aware TLS provisioning — installs
  mkcert if missing, discovers SANs (localhost/LAN/tailscale, IPv6 and
  docker bridges filtered), issues the server cert, imports the CA into
  the Windows trust store from WSL (UAC, idempotent), restarts the
  service; `--ios` serves the CA on the tailnet with a QR code.
  CI now syntax-checks shell scripts.
- **URL-routed sessions** (G4, issue #4): sessions live at `/s/<id>` —
  back/forward and reload work; legacy `?s=` deep-links normalize in
  place. (History API; the Go SPA fallback serves the app on any path.)
- **Composer draft persistence** (G2, issue #2): per-session drafts in
  localStorage — survive reloads and session switches, cleared on send.
- **Session state machine** (`idle | running | waiting`) — the SSE
  `state` event carries `status` (G1, issue #1). `waiting` = a turn is
  in flight AND the agent asked for approval; late subscribers get the
  pending permission replayed on SSE connect. UI header shows
  waiting/running distinctly.
- **Message queueing / steering** (G3, issue #3): sending while a turn
  is in flight returns `202 {"queued":true}` and the message is
  delivered automatically at turn end (cap 5, then 409).
  `POST /api/sessions/{id}/queue/cancel` discards the queue; the UI
  shows a queued tag and a cancel affordance. Queued messages persist
  to history immediately (visible while waiting); an AgentDeck
  restart mid-queue drops undelivered ones (documented limitation).
- Benchmark-first workflow: t3code and paseo adopted as reference
  projects (AGENTS.md); first study (sessions & composer) with a
  13-gap adversarial comparison → 8 tracked issues (#1–#8).
- Self-signed certificate now covers localhost, LAN and tailnet IPs
  (all local IPv4 SANs) — trusting it once in the OS removes browser
  warnings on every access route (incl. VS Code Simple Browser, which
  has no cert bypass button).
- **Runner v2 / live sessions (ADR-0004, tier 1)**: tier-1 agents
  (claude) now run as persistent bidirectional processes — the web UI
  is a native client of the agent protocol, like the TUIs.
  - In-process conversation memory across turns (no resume dance);
    transparent restart-with-ref when the agent process dies.
  - Permission round-trip: agents' `control_request` events surface in
    the UI as Allow/Deny buttons (`POST /api/sessions/{id}/control`).
  - Fake claude gained a bidirectional `live` mode (stdin/stdout JSONL)
    powering deterministic tests: memory, permission allow/deny,
    crash-restart.
  - One-shot spawn remains the fallback tier for other agents
    (`Registry.DisableLive` for tests/config).
- Go implementation (Phase 1): single binary embedding the React SPA
  (`go:embed`), self-signed TLS generated natively (no openssl).
  - `internal/agent`: Adapter interface + registry with env overrides
    (`AGENTDECK_BIN_<id>`) and event parsers for claude / codex / grok /
    pi / opencode, unit-tested against JSONL recorded from the real CLIs.
  - `internal/store`: SQLite (WAL), schema-identical to the Phase-0
    Python server (shared data dir), ANSI-safe sidebar previews.
  - `internal/runner`: subprocess lifecycle with merged stderr
    (explicit `os.Pipe`), one process per session (409 on busy),
    stop-persists-partial, process-group kill on stop.
  - `internal/server`: HTTP API + SSE, contract-identical to the
    Phase-0 surface (parity suite in `server_test.go`).
  - `internal/config`: execution-mode feature flag (ADR-0002):
    `AGENTDECK_MODE=auto|personal|dedicated`; `AGENTDECK_INSECURE=1`
    disables TLS.
- `tests/fakes/`: fake agent binaries powering deterministic, offline,
  zero-token tests.
- `tests/spikes/`: evidence scripts for architecture decisions (see
  ADR-0004 — persistent bidirectional agent processes proven against
  the real claude CLI; tmux integration discarded).
- `.pi/skills/agent-browser/`: skill for verifying the web UI with a
  real browser (tested against the CLI, includes stale-ref and zombie
  discipline, install instructions and fallback).
- Repo foundation: `AGENTS.md` (agent handbook + invariants), ADRs
  0001–0003, `docs/SECURITY.md` with threat model, CONTRIBUTING with
  the TDD workflow, issue/PR templates, CI (Go + web + legacy).
- `docs/HANDOFF.md`: living state-of-the-project doc for cold starts
  and agent-to-agent handoff.
- `CHANGELOG.md` (this file) with a mandatory docs-sync policy
  (see `AGENTS.md`).

### Fixed
- Ghost toolbar band: the (now state-only) chat toolbar rendered even
  when empty on desktop — a ~59px gray strip under the tab bar, taller
  than the tab bar itself (user report). It now renders only when it
  has content (cwd badge, status, queue, stop) or on mobile (menu
  button). Verified: zero visible headers on a fresh desktop session.
- Agent discovery missed user-local installs under systemd: the
  registry only consulted PATH, and service environments don't include
  ~/.local/bin or ~/.bun/bin — claude, grok and opencode vanished from
  the picker when running as a service (regression surfaced on the
  home screen). Lookup now falls back to user-local dirs; EnvWhich
  chains the same base resolver (bug: passing an explicit `which`
  bypassed the fallback entirely).
- Web app crashed on load (`permission is not defined`) — state and
  SSE handler for the approval banner were lost in an edit chain;
  restored. Root-cause amplifier: a stale agentdeck process (old
  build + old cert) was holding :8444 while the service ran elsewhere,
  serving a broken embedded UI.
- AgentDeck now ships a **systemd user unit** (`~/.config/systemd/user/
  agentdeck.service`): single supervised process, restart-on-failure,
  survives shell exit and reboots. (Manual: `systemctl --user start
  agentdeck`; logs: `journalctl --user -u agentdeck -f`.)
- Runner: killing an agent now kills its **process group** — children
  inheriting the output pipe kept the scanner blocked after stop
  (caught by CI, `TestStopPersistsPartial`).
- Sidebar previews: ANSI escapes and ASCII-art banners are stripped;
  box-drawing lines never become previews.

### Removed
- (Nothing yet — Phase-0 Python stays in `legacy/` until parity is
  proven in the field.)

[Unreleased]: https://github.com/cfpperche/agentdeck/commits/main
