#!/usr/bin/env bash
# smoke-e2e.sh — end-to-end smoke against the real binary + fake agents.
# Used by CI: proves boot, session create, live turn, permission flow,
# fs endpoints and port info through the ACTUAL HTTP surface.
set -euo pipefail
cd "$(dirname "$0")/.."

DATA=$(mktemp -d); PORT=8488
cleanup() { kill "$SRV_PID" 2>/dev/null || true; rm -rf "$DATA"; }
trap cleanup EXIT

# CI machines have no agent CLIs: inject fakes via the env-override
# mechanism (AGENTDECK_BIN_<id>) — same production code path.
FAKES="$PWD/tests/fakes"
AGENTDECK_DATA="$DATA" AGENTDECK_PORT="$PORT" AGENTDECK_INSECURE=1 \
  AGENTDECK_SDK_SHIM="$PWD/agent-sdk-shim/shim.mjs" \
  AGENTDECK_SDK_FAKE=1 FAKE_ASK=1 \
  AGENTDECK_BIN_CLAUDE="$FAKES/fake-claude" \
  AGENTDECK_BIN_PI="$FAKES/fake-pi" \
  ./bin/agentdeck > "$DATA/server.log" 2>&1 &
SRV_PID=$!

for i in $(seq 1 50); do
  curl -sf "http://localhost:$PORT/api/server-info" > /dev/null && break
  sleep 0.2
done

BASE="http://localhost:$PORT"
fail() { echo "SMOKE FAIL: $*" | tee -a "$DATA/server.log" >&2; exit 1; }

# 1. agents
curl -sf "$BASE/api/agents" | grep -q claude || fail "agents"

# 2. create + live turn through the fake SDK shim
SID=$(curl -sf -X POST "$BASE/api/sessions" -H 'Content-Type: application/json' \
  -d '{"agent":"claude"}' | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')
curl -sf -X POST "$BASE/api/sessions/$SID/messages" -H 'Content-Type: application/json' \
  -d '{"text":"Remember: smoke works"}' > /dev/null || fail "send"
for i in $(seq 1 40); do
  OUT=$(curl -sf "$BASE/api/sessions/$SID/messages" | python3 -c 'import json,sys;ms=json.load(sys.stdin);print(ms[-1]["content"] if ms and ms[-1]["role"]=="assistant" else "")')
  [ "$OUT" = "OK" ] && break
  sleep 0.25
done
[ "$OUT" = "OK" ] || fail "live turn (got: $OUT)"

# 2b. composer controls (ADR-0006): model must reach the SDK options
curl -sf -X POST "$BASE/api/sessions/$SID/messages" -H 'Content-Type: application/json' \
  -d '{"text":"Which model am I?","controls":{"model":"opus","thinking":"","mode":""}}' > /dev/null
for i in $(seq 1 40); do
  OUT=$(curl -sf "$BASE/api/sessions/$SID/messages" | python3 -c 'import json,sys;ms=json.load(sys.stdin);print(ms[-1]["content"] if ms and ms[-1]["role"]=="assistant" else "")')
  [ "$OUT" = "model: opus" ] && break
  sleep 0.25
done
[ "$OUT" = "model: opus" ] || fail "composer controls -> SDK options (got: $OUT)"

# 3. memory in the same process
curl -sf -X POST "$BASE/api/sessions/$SID/messages" -H 'Content-Type: application/json' \
  -d '{"text":"What do you remember?"}' > /dev/null
for i in $(seq 1 40); do
  OUT=$(curl -sf "$BASE/api/sessions/$SID/messages" | python3 -c 'import json,sys;ms=json.load(sys.stdin);print(ms[-1]["content"] if ms and ms[-1]["role"]=="assistant" else "")')
  [ "$OUT" = "smoke works" ] && break
  sleep 0.25
done
[ "$OUT" = "smoke works" ] || fail "memory (got: $OUT)"

# 4. fs endpoints (mkdir is home-scoped by design)
curl -sf "$BASE/api/fs/dirs" | grep -q '"dirs"' || fail "fs/dirs"
D="$HOME/.agentdeck-smoke-$$"
curl -sf -X POST "$BASE/api/fs/mkdir" -H 'Content-Type: application/json' \
  -d "{\"path\":\"$D/sub\"}" | grep -q sub || fail "fs/mkdir"
curl -sf -X POST "$BASE/api/sessions" -H 'Content-Type: application/json' \
  -d "{\"agent\":\"claude\",\"cwd\":\"$D/sub\"}" | grep -q '"cwd"' || fail "create with cwd"
rm -rf "$D"

# 4b. composer surface (ADR-0006): capabilities event on the SSE stream
CAPS=$(timeout 3 curl -sfN "$BASE/api/sessions/$SID/events" | head -c 4000 | grep -o '"type":"capabilities"' | head -1 || true)
[ -n "$CAPS" ] || fail "capabilities on SSE"

# 5. port info + reject range via UI contract
curl -sf "$BASE/api/server/port" | grep -q serving || fail "port info"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE/api/server/port" \
  -H 'Content-Type: application/json' -d '{"port":"9000-9010"}')
[ "$CODE" = "400" ] || fail "range rejected (got $CODE)"

echo "SMOKE E2E PASS ✓ (boot, agents, live turn, memory, fs, cwd, port)"
