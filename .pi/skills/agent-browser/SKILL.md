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

- Refs from `snapshot` (like `@e3`) are the most reliable way to target elements.
- Prefer `snapshot` over `screenshot` for acting; use screenshots to verify visuals.
- Session persists across commands — no need to re-open the URL between steps.

## Testing AgentDeck itself

The dev server uses a self-signed certificate. Pass the flag once per session:

```bash
agent-browser open https://localhost:8444 --ignore-https-errors
```

(Or `export AGENT_BROWSER_IGNORE_HTTPS_ERRORS=1` before starting the session.)
