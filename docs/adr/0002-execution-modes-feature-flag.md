# ADR-0002: Execution modes behind a feature flag

- **Status:** accepted
- **Date:** 2026-08-23

## Context

Agents execute with the privileges of the user running AgentDeck. Two
audiences exist:

1. Developers who want zero setup and accept "the agent is me".
2. Users who want isolation: a dedicated system user with a restricted
   sudo allowlist (what
   [aiagent-linux](https://github.com/cfpperche/aiagent-linux) provisions).

Making the dedicated user a hard dependency would raise the install cost
for audience 1; ignoring it would ship an unsafe default for audience 2.

## Decision

AgentDeck does **not** depend on iaagent-linux. A single boot-time
feature flag selects the execution mode:

```toml
[server]
run_mode = "auto"        # auto | personal | dedicated
```

- `personal` — run under the current user (default; zero config).
- `dedicated` — run under a dedicated user; the server detects it via
  `getpwuid` and surfaces the mode through `/api/server-info`.
- `auto` — detect: dedicated user → `dedicated`, otherwise `personal`.

The UI shows the active mode as a badge.

## Consequences

- One code path for both audiences; the flag changes diagnostics and
  docs shown, not behavior branches scattered around.
- iaagent-linux becomes an *optional hardening layer* we can recommend
  and document, not a dependency we must vendor.
- Security documentation must be explicit that `personal` mode grants
  agents the operator's full local privileges.
