# ADR-0005: Claude driver via Agent SDK shim (tier 1.5)

- **Status:** accepted
- **Date:** 2026-08-24
- **Related:** ADR-0004 (persistent protocol sessions), benchmark study
  G5 (issue #5)

## Context

The claude CLI in `-p` mode never emits permission `control_request`s:
without a prompt surface, "ask" decisions are terminal (auto-deny) —
validated empirically twice (spike + adversarial round). Our Allow/Deny
banner therefore only worked with fakes.

The **Agent SDK** (`@anthropic-ai/claude-agent-sdk`) exposes exactly the
missing surface: `query({ options: { canUseTool } })` delivers a real
`control_request` (tool name + input) to a callback and awaits our
answer (`allow | deny | updatedInput`). Spiked end-to-end against the
live subscription (`tests/spikes/claude-sdk-permission-spike.mjs`):
tool_use → permission request → allow → tool executed → "DONE".

## Decision

Add an **SDK driver for claude**, implemented as a small **Node shim**
process that speaks our existing wire protocol:

```
AgentDeck (Go) ── stdin/stdout JSONL (ADR-0004 shapes) ──► shim (Node)
                                                            │ Agent SDK
                                                            ▼
                                                        claude (real)
```

- The shim normalizes SDK messages into the SAME event kinds the
  adapters already parse (`init→ref`, assistant text/tool_use,
  `control_request`, `result`). User messages and control_responses
  flow back over stdin.
- The Go adapter for claude prefers the shim when the SDK runtime is
  detectable (`node` + the package resolvable), falling back to CLI
  stdio otherwise. The registry stays adapter-shaped: no server
  changes.
- The shim is vendored in `agent-sdk-shim/` with its own package.json;
  AgentDeck does not require npm at runtime beyond `node` itself.

## Consequences

- Real interactive permissions for claude sessions (banner Allow/Deny
  finally lives against production claude).
- `updatedInput` support becomes real: we can EDIT a tool input before
  allowing (feeds issue #7, permission banner v2).
- Runtime requirement grows: node must exist for the SDK path (CLI
  fallback keeps zero-node working).
- The shim is a translation layer we own; SDK version drift is
  contained there (contract tests with a fake SDK pin the shapes).
- Kill/restart semantics: the shim is the live process (same
  process-group + restart-with-ref handling as tier 1; session ref =
  SDK session id, resumable via `resume` option).
