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

## Local TLS with mkcert (recommended for daily use)

The server self-signs a cert covering localhost + local IPs, but
browsers show "Not secure" until the issuer is trusted. For warning-free
daily use, issue from a local CA with [mkcert](https://github.com/FiloSottile/mkcert):

```bash
# once per machine: install the CA (Linux side)
mkcert -install

# issue the server cert (AgentDeck reads data/cert.pem + data/key.pem)
cd agentdeck
mkcert -cert-file data/cert.pem -key-file data/key.pem \
  localhost 100.87.149.83 192.168.15.110   # your local + tailnet IPs

systemctl --user restart agentdeck
```

Windows browsers must trust the CA too — as Administrator:

```powershell
Import-Certificate -FilePath "$env:USERPROFILE\mkcert-rootCA.pem" -CertStoreLocation Cert:\LocalMachine\Root
```

(the `mkcert-rootCA.pem` is `$(mkcert -CAROOT)/rootCA.pem`; iOS: Settings
→ Profile Downloaded → Install, then enable full trust in
Settings → General → About → Certificate Trust Settings).
