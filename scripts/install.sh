#!/usr/bin/env bash
# ============================================================================
# install.sh — install AgentDeck from GitHub releases
#
#   curl -fsSL https://raw.githubusercontent.com/cfpperche/agentdeck/main/scripts/install.sh | bash
#
# Downloads the binary for your platform into ~/.local/bin/agentdeck.
# Flags:
#   --version vX.Y.Z   pin a version (default: latest release)
#   --systemd          also install the systemd user unit + start it
# ============================================================================
set -euo pipefail

REPO="cfpperche/agentdeck"
VERSION=""
WANT_SYSTEMD=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --systemd) WANT_SYSTEMD=1; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,2\}//'; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done

say() { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# --- platform detection -----------------------------------------------------
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"     # linux / darwin
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $ARCH" ;;
esac
[[ "$OS" == linux || "$OS" == darwin ]] || die "unsupported OS: $OS"
BIN_NAME="agentdeck-${OS}-${ARCH}"

# --- resolve version --------------------------------------------------------
if [[ -z "$VERSION" ]]; then
  say "resolving latest release…"
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)" \
    || die "could not reach GitHub (offline? private repo?)"
  [[ -n "$VERSION" ]] || die "no releases published yet"
fi
say "installing ${VERSION} (${BIN_NAME})"

# --- download + verify ------------------------------------------------------
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
curl -fL --progress-bar -o "$TMP/agentdeck"      "${BASE_URL}/${BIN_NAME}"
curl -fsSL           -o "$TMP/SHA256SUMS"        "${BASE_URL}/SHA256SUMS"

(cd "$TMP" && grep " ${BIN_NAME}\$" SHA256SUMS | sha256sum -c -) \
  || die "checksum mismatch — download corrupted or tampered"

# --- install ----------------------------------------------------------------
DEST="${HOME}/.local/bin"
mkdir -p "$DEST"
install -m 0755 "$TMP/agentdeck" "${DEST}/agentdeck"
case ":$PATH:" in
  *":${DEST}:"*) ;;
  *) say "NOTE: ${DEST} is not on your PATH — add it to your shell profile." ;;
esac
say "installed: ${DEST}/agentdeck ($("${DEST}/agentdeck" --version 2>/dev/null || echo "${VERSION}"))"

# --- optional systemd user unit ---------------------------------------------
if [[ "$WANT_SYSTEMD" == 1 ]]; then
  UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
  mkdir -p "$UNIT_DIR"
  cat > "${UNIT_DIR}/agentdeck.service" <<EOF
[Unit]
Description=AgentDeck — cockpit for local AI coding agents
After=network-online.target

[Service]
ExecStart=${DEST}/agentdeck
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
EOF
  systemctl --user daemon-reload
  systemctl --user enable --now agentdeck
  say "service running → journalctl --user -u agentdeck"
fi

say "done. run: agentdeck  (then open the printed URL)"
