# Contributing to AgentDeck

Thanks for considering a contribution. The repo language is **English**
(docs, code comments, commit messages, issues).

## Development setup

```bash
git clone https://github.com/cfpperche/agentdeck
cd agentdeck

# web UI
cd web && npm install && npm run build && cd ..

# run the prototype server (Phase 0)
python3 -m legacy.backend.__main__    # https://localhost:8444

# frontend dev with hot reload (proxies /api)
cd web && npm run dev
```

## Workflow

1. **Issues first** for anything non-trivial — avoids wasted work.
2. **Conventional commits** (`feat:`, `fix:`, `docs:`, `refactor:` …).
3. **Small PRs**, one logical change each. Main is kept green.
4. **Tests with your change** — see the testing philosophy below.

## Testing philosophy (TDD)

The agent adapters are tested against **fake agents**: tiny scripts that
emit the same JSONL the real CLIs emit. Tests are deterministic, offline,
and spend zero tokens. Real-agent smoke tests are opt-in and never block
CI.

When porting or adding an adapter: write the fake first, then the parser,
then the adapter. Bug fixes come with a regression test that fails
before the fix.

You can also run the real server against fakes end-to-end:
`AGENTDECK_BIN_CLAUDE=$PWD/tests/fakes/fake-claude ./bin/agentdeck`.

**UI verification** uses the [agent-browser](https://www.npmjs.com/package/agent-browser)
CLI (`npm i -g agent-browser`) — see `.pi/skills/agent-browser/SKILL.md`;
a plain headless-Chrome screenshot works as fallback. `AGENTDECK_INSECURE=1`
disables TLS when you don't want to handle the self-signed cert.

## Architecture pointers

- `docs/adr/` — why things are the way they are (read before "why don't
  you just…" PRs 🙂)
- Adapters: one command builder + one event parser per agent, behind a
  single interface. The server never talks to subprocesses directly.
- Frontend only knows the HTTP contract; keep TypeScript types in sync.

## Reporting bugs

Use the issue templates. For anything security-related, see
[docs/SECURITY.md](docs/SECURITY.md) — do not open public issues.
