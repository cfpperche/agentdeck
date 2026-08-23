# Benchmark studies

Before designing or building any feature, study how the benchmark
projects solved the same problem, then propose our architecture in the
PR/ADR. Findings live here as dated study notes with file-path receipts.

## Benchmarks

| Project | What it is | Why we watch it |
|---|---|---|
| [pingdotgg/t3code](https://github.com/pingdotgg/t3code) | "Agent harness control surface" — server + web/mobile/desktop clients controlling Claude Code, Codex, Cursor, Grok Build, OpenCode | Closest to AgentDeck's mission; gold-standard runtime normalization (SDKs, ACP) and composer depth |
| [getpaseo/paseo](https://github.com/getpaseo/paseo) | Daemon + cross-device clients for Claude, Codex, Copilot, OpenCode, Pi | Same mission, different bet (PTY + agent-hooks, task graphs, pairing relay, voice) |

Both are agent-native repos (root `AGENTS.md`) — like us.

## Studies

- [2026-08-23 — Session management & composer](2026-08-23-sessions-and-composer.md)
