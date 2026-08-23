#!/usr/bin/env python3
"""
Spike: claude as a PERSISTENT bidirectional process (no TUI, no tmux).

Proves:
  1. Multi-turn memory in ONE living process (no --resume dance)
  2. Permission round-trip: request arrives as JSON, we answer via stdin

Run: python3 tests/spikes/claude-interactive-spike.py
"""
import json
import subprocess
import sys
import time
from pathlib import Path

CWD = str(Path(__file__).resolve().parents[2])  # repo root (trusted folder)

proc = subprocess.Popen(
    ["claude", "-p", "--input-format", "stream-json",
     "--output-format", "stream-json", "--verbose"],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE,
    stderr=subprocess.STDOUT, cwd=CWD, text=True, bufsize=1,
)

events = []


def send_user(text):
    msg = {"type": "user", "message": {"role": "user",
                                       "content": [{"type": "text", "text": text}]}}
    proc.stdin.write(json.dumps(msg) + "\n")
    proc.stdin.flush()


def pump(timeout_s, stop_types):
    """Read JSONL lines until one of stop_types arrives or timeout."""
    deadline = time.time() + timeout_s
    import select
    while time.time() < deadline:
        r, _, _ = select.select([proc.stdout], [], [], 0.5)
        if not r:
            if proc.poll() is not None:
                break
            continue
        line = proc.stdout.readline()
        if not line.strip():
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            print("  [non-json]", line.strip()[:100])
            continue
        events.append(ev)
        t = ev.get("type")
        yield ev
        if t in stop_types:
            return
    return


def result_text():
    for ev in reversed(events):
        if ev.get("type") == "result":
            return ev.get("result", "")
    return ""


print("=== TURN 1: plant a fact in THIS process ===")
send_user("Remember this for the whole conversation: my favorite number is 42. Reply with just: OK")
for ev in pump(120, {"result"}):
    if ev.get("type") == "result":
        print("  result:", ev.get("result", "")[:60])

print("\n=== TURN 2: same living process, ask it back (no resume flag!) ===")
events.clear()
send_user("What is my favorite number? Reply with just the number.")
for ev in pump(120, {"result"}):
    if ev.get("type") == "result":
        print("  result:", ev.get("result", "")[:60])

ans = result_text()
mem_ok = "42" in ans
print("  → MEMORY IN LIVING PROCESS:", "PASS ✓" if mem_ok else "FAIL ✗")

print("\n=== TURN 3: permission round-trip (bash write) ===")
events.clear()
send_user("Create the file /tmp/agentdeck-spike.txt containing 'hello'. Use the Bash tool.")
perm_request = None
for ev in pump(120, {"result"}):
    t = ev.get("type")
    # permission requests surface as control_request (can_use_tool)
    if t == "control_request":
        print("  CONTROL REQUEST:", json.dumps(ev)[:300])
        perm_request = ev
    elif t == "assistant":
        for b in ev.get("message", {}).get("content", []):
            if b.get("type") == "tool_use":
                print("  tool_use:", b.get("name"), str(b.get("input"))[:80])

if perm_request:
    print("\n  → answering ALLOW via stdin control_response...")
    resp = {"type": "control_response",
            "request_id": perm_request.get("request_id") or perm_request.get("id"),
            "response": {"behavior": "allow",
                         "updatedInput": {}}}
    proc.stdin.write(json.dumps(resp) + "\n")
    proc.stdin.flush()
    for ev in pump(120, {"result"}):
        if ev.get("type") == "result":
            print("  result:", ev.get("result", "")[:80])
else:
    print("  (no control_request observed — check allowedTools / mode)")

import os
made = os.path.exists("/tmp/agentdeck-spike.txt")
print("  → PERMISSION ROUND-TRIP:", "PASS ✓" if (perm_request and made) else
      ("NO-ASK (pre-approved) file=%s" % made))

proc.stdin.close()
proc.wait(timeout=15)
print("\nspike done — process exited", proc.returncode)
