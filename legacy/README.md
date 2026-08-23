# Phase-0 prototype (Python)

This is the original implementation that validated the architecture:
FastAPI + SSE + SQLite, subprocess adapters for each agent CLI, and the
React build served from `../../web/dist`.

**It still works and is what the README's quick start runs.** It stays
here until the Go port (`../../internal/`) reaches feature parity, then
it will be removed.

Notes:

- Built rapidly as a prototype; some comments are in Portuguese — kept
  for historical accuracy rather than churned.
- `python3 -m legacy.backend.__main__` from the repo root.
- The HTTP API of this prototype is the **contract** the Go port must
  reproduce (parity tests are written against it).
