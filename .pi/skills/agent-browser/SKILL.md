---
name: agent-browser
description: Browser automation with a real Chromium via the agent-browser CLI (Vercel). Use when the task requires opening websites, clicking, typing, filling forms, reading/scraping page content, screenshots, PDFs, console/network inspection, or testing web UIs end-to-end.
---

# agent-browser

`agent-browser` is a stateful CLI that drives a Chromium session. Every bash
invocation continues the same browser session until you run `close`.

## Load the full guide first

The CLI ships its own version-matched skill docs. Before automating, run:

```bash
agent-browser skills get core
```

For specialized scenarios there are more guides: `agent-browser skills list`
(exploratory testing, Electron apps, Slack, cloud browsers, ...).

## Quick reference

```bash
agent-browser open <url>          # Navigate (starts the session)
agent-browser snapshot            # Accessibility tree with @refs (e.g. @e3)
agent-browser click @e3           # Click element by ref or CSS selector
agent-browser type @e5 "text"     # Type into element
agent-browser fill @e5 "text"     # Clear field and fill
agent-browser press Enter         # Press key (Tab, Control+a, ...)
agent-browser select <sel> <val>  # Select dropdown option
agent-browser get text <sel>      # text | html | title | url | value | attr <name>
agent-browser read <url>          # Fetch agent-readable page text
agent-browser eval "<js>"         # Run JavaScript in the page
agent-browser screenshot out.png  # Save screenshot (view it with the read tool)
agent-browser pdf out.pdf         # Save page as PDF
agent-browser wait <sel|ms>       # Wait for element or time
agent-browser close               # Close the browser session
```

## Standard workflow

1. `open <url>`
2. `snapshot` to discover interactive elements and their `@refs`
3. Act: `click`, `type`, `fill`, `press`, `select`...
4. Verify: `snapshot` again, `get text`, or `screenshot` + `read` the image
5. `close` when finished

## Tips

- **Refs go stale after any action that changes the page** (clicks, typing,
  navigation, React re-renders). Snapshot again before every action — a
  `@ref` from an older snapshot may target nothing or the wrong element.
- Prefer `snapshot` over `screenshot` for acting; use screenshots to verify visuals.
- Session persists across commands — no need to re-open the URL between steps.
- **Zombie discipline**: sessions survive aborted tasks. Start tricky work with
  `close` (a failed close is harmless) and always `close` before finishing —
  otherwise a headless Chromium keeps running on the host.

## Setup & fallback

Requires the CLI (not installed by this repo):

```bash
npm install -g agent-browser
```

No CLI? Fallback for visual checks (no interaction, screenshot only):

```bash
google-chrome --headless=new --no-sandbox --ignore-certificate-errors \
  --window-size=1440,900 --screenshot=out.png https://localhost:8444
```

## Testing AgentDeck itself

Two dev routes, pick either:

```bash
# A) Go server (:8444, HTTPS self-signed) — pass the flag once per session:
agent-browser open https://localhost:8444 --ignore-https-errors
#    (or export AGENT_BROWSER_IGNORE_HTTPS_ERRORS=1)

# B) Vite dev server (:5173, plain HTTP, hot reload) — no flag needed:
agent-browser open http://localhost:5173
```

Tip: `AGENTDECK_INSECURE=1 ./bin/agentdeck` serves the Go server over plain
HTTP when you want to skip certificate handling entirely.
