"""AgentDeck entrypoint: uvicorn with self-signed HTTPS (Tailscale-gated)."""
import ssl
import subprocess
import sys
from pathlib import Path

import uvicorn

from .config import DATA_DIR, HOST, PORT

ROOT = Path(__file__).resolve().parents[2]
CERT = ROOT / "cert.pem"
KEY = ROOT / "key.pem"


def ensure_cert():
    if CERT.exists() and KEY.exists():
        return
    subprocess.run(
        ["openssl", "req", "-x509", "-newkey", "rsa:2048",
         "-keyout", str(KEY), "-out", str(CERT), "-days", "3650", "-nodes",
         "-subj", "/CN=agentdeck"],
        check=True, capture_output=True,
    )
    print(f"[agentdeck] self-signed cert generated at {CERT}")


def main():
    ensure_cert()
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.load_cert_chain(CERT, KEY)
    print(f"[agentdeck] https://{HOST}:{PORT}  (data: {DATA_DIR})")
    uvicorn.run(
        "legacy.backend.main:app", host=HOST, port=PORT, ssl_certfile=str(CERT),
        ssl_keyfile=str(KEY), log_level="warning",
    )


if __name__ == "__main__":
    sys.path.insert(0, str(ROOT))
    main()
