#!/usr/bin/env python3
"""Persistent bidirectional fake claude (runner v2 test double).

Wire contract mirrors 'claude -p --input-format stream-json
--output-format stream-json':

stdin  (we read):  {"type":"user","message":{"content":[{"type":"text","text":...}]}}
                  {"type":"control_response", "request_id":..., "response":{...}}
stdout (we emit): system/init, assistant tool_use, control_request
                  (when FAKE_ASK=1), assistant text, result

In-process memory across turns — no session files — plus FAKE_CRASH_AFTER
to simulate a dying agent for the restart-with-ref path.
"""
import json
import os
import sys

MEMORY = {}
SESSION_ID = "fake-live-session-42"
TURNS = 0


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def user_text(msg):
    for block in msg.get("message", {}).get("content", []):
        if isinstance(block, dict) and block.get("type") == "text":
            return block.get("text", "")
    return ""


def answer(text):
    global TURNS
    TURNS += 1
    emit({"type": "system", "subtype": "init", "session_id": SESSION_ID})

    low = text.lower()
    if low.startswith("remember:"):
        MEMORY["fact"] = text[len("remember:"):].strip()
        emit({"type": "assistant", "message": {"role": "assistant", "content": [
            {"type": "text", "text": "OK"}]}})
        emit({"type": "result", "subtype": "success", "result": "OK",
              "session_id": SESSION_ID})
    elif "favorite number" in low or "what do you remember" in low:
        fact = MEMORY.get("fact", "nothing")
        emit({"type": "assistant", "message": {"role": "assistant", "content": [
            {"type": "text", "text": fact}]}})
        emit({"type": "result", "subtype": "success", "result": fact,
              "session_id": SESSION_ID})
    elif low.startswith("write file") or low.startswith("create file"):
        if os.environ.get("FAKE_ASK") == "1":
            req_id = "perm-001"
            emit({"type": "control_request", "request_id": req_id,
                  "tool_name": "Bash",
                  "input": {"command": "echo hello > /tmp/agentdeck-live.txt"}})
            # handled from stdin (control_response) by the main loop
            return None  # turn suspended; resume on control_response
        with open("/tmp/agentdeck-live.txt", "w") as f:
            f.write("hello")
        emit({"type": "assistant", "message": {"role": "assistant", "content": [
            {"type": "tool_use", "name": "Bash",
             "input": {"command": "echo hello > /tmp/agentdeck-live.txt"}}]}})
        emit({"type": "assistant", "message": {"role": "assistant", "content": [
            {"type": "text", "text": "File written"}]}})
        emit({"type": "result", "subtype": "success", "result": "File written",
              "session_id": SESSION_ID})
    else:
        emit({"type": "assistant", "message": {"role": "assistant", "content": [
            {"type": "text", "text": "echo: " + text}]}})
        emit({"type": "result", "subtype": "success", "result": "echo: " + text,
              "session_id": SESSION_ID})

    crash_after = os.environ.get("FAKE_CRASH_AFTER")
    if crash_after is not None and TURNS >= int(crash_after):
        os._exit(7)  # abrupt death — no result, no goodbye
    return None


pending = None  # suspended turn awaiting control_response
emit({"type": "system", "subtype": "init", "session_id": SESSION_ID})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except json.JSONDecodeError:
        continue

    if msg.get("type") == "user":
        pending = user_text(msg)
        answer(pending)
    elif msg.get("type") == "control_response":
        # resume the suspended permission turn
        behavior = msg.get("response", {}).get("behavior")
        if behavior == "allow":
            with open("/tmp/agentdeck-live.txt", "w") as f:
                f.write("hello")
            emit({"type": "assistant", "message": {"role": "assistant", "content": [
                {"type": "tool_use", "name": "Bash",
                 "input": {"command": "echo hello > /tmp/agentdeck-live.txt"}}]}})
            emit({"type": "assistant", "message": {"role": "assistant", "content": [
                {"type": "text", "text": "File written"}]}})
            emit({"type": "result", "subtype": "success", "result": "File written",
                  "session_id": SESSION_ID})
        else:
            emit({"type": "assistant", "message": {"role": "assistant", "content": [
                {"type": "text", "text": "Permission denied by user"}]}})
            emit({"type": "result", "subtype": "success",
                  "result": "Permission denied by user",
                  "session_id": SESSION_ID})
        pending = None
