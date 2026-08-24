# ADR-0007: Driver tiers — native protocols, ACP, CLI fallback

Date: 2026-08-24
Status: Accepted

## Context

Only Claude runs as a first-class persistent process (ADR-0005 SDK
shim). Codex/grok/pi/opencode ride headless CLI spawns per message:
no steering, no queue semantics, controls apply per spawn, continuity
via `--resume` heuristics. The user asked the obvious question: isn't
there a smarter way?

Feasibility spikes on this machine (2026-08-24, receipts):

- **opencode**: native ACP server (`opencode acp`, v1.18.20). Verified:
  NDJSON framing; `initialize` → rich agentCapabilities;
  `session/new` → sessionId + **live model catalog** (81 entries via
  configOptions); `session/prompt` streams `agent_message_chunk`
  updates ending with `stopReason`; `session/set_config_option`
  switches models mid-session; `loadSession:true` (resume supported).
- **pi**: `--mode rpc` — own bidirectional RPC protocol.
- **codex**: `app-server` (experimental) — own JSON-RPC protocol;
  t3code drives codex through exactly this.
- **grok**: CLI only (streaming-json + resume flags).
- **gemini**: not installed; speaks ACP natively when added
  (`--experimental-acp`).

## Decision

Three driver tiers, best available per agent:

1. **Native driver** — richest protocol the agent offers:
   claude via Agent SDK shim (ADR-0005). Later: pi rpc, codex
   app-server.
2. **ACP driver** — one generic client for any Agent-Client-Protocol
   agent. First target: opencode (native). Future agents (gemini)
   join by configuration, not code.
3. **CLI fallback** — spawn-per-message with flag injection
   (ADR-0006). Stays as safety net, never removed.

The ACP integration follows the proven shim pattern: a bridge process
speaks ACP towards the agent and our ADR-0004 wire towards the runner,
so the runner needs zero protocol awareness. The bridge emits the
claude dialect (system/init, assistant text, result) because every
parser already normalizes it.

## Consequences

- Model lists become **live data** where the agent provides them
  (opencode's catalog comes from the agent itself), replacing curated
  statics — the benchmark-endorsed dynamic override path.
- Model switching becomes mid-session state, not a respawn.
- One new integration surface to maintain (the ACP client); contained
  in `internal/acp` + a bridge entrypoint.
- Agents without any protocol stay on tier-3 honestly (grok today).
