# Security Policy

## Threat model

AgentDeck deliberately does two dangerous-sounding things:

1. It **executes agent CLIs** (which run shells, edit files, use your
   credentials) with the privileges of the user running the server.
2. It **exposes that capability over HTTPS** to whatever can reach the
   port.

Treat an AgentDeck endpoint like an SSH endpoint: whoever can reach it
can act on your machine.

## Current guarantees and limits

| Aspect | Status |
|---|---|
| Transport | HTTPS with a self-signed certificate (generate your own / front with a proxy if you want valid TLS) |
| Authentication | **None yet** — network position *is* the access control. Roadmap: token auth |
| Agent privileges | Whatever the server user has (`personal` mode: your full user). See [ADR-0002](adr/0002-execution-modes-feature-flag.md) for the isolated `dedicated` mode |
| Session data | SQLite + workspace files under the data dir, unencrypted |

## Deployment recommendations

1. **Never bind to the public internet.** The default is fine on a
   personal machine; on a shared host, bind to `127.0.0.1` or a VPN.
2. **Prefer a tailnet** (Tailscale/WireGuard): reach it from anywhere
   without exposing a port; the certificate warning you accept is the
   self-signed cert, traffic stays inside the VPN.
3. **Run in `dedicated` mode** when the machine holds anything you care
   about — a dedicated user (see
   [aiagent-linux](https://github.com/cfpperche/aiagent-linux)) with a
   sudo allowlist bounds what a hallucinating agent can touch.
4. **Rotate credentials** if an endpoint was ever reachable by someone
   you didn't intend.

## Reporting a vulnerability

Open a private security advisory
(GitHub → *Security* → *Report a vulnerability*) or email the maintainer.
Please do not open public issues for vulnerabilities.
