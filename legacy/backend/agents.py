"""
Agent registry: how to launch each CLI agent headless, resume its native
session, and parse its output stream into normalized events.

Normalized events emitted by parsers:
    ("ref",   agent_session_ref)
    ("text",  chunk)                       # assistant text
    ("tool",  name, state, detail)         # state: start|end, detail: str
    ("final", full_text)                   # authoritative final text
    ("error", message)

All commands calibrated against the real CLIs on this machine.
"""
import json
import shutil
from pathlib import Path

from .config import HOME


def _which(name):
    return shutil.which(name)


# ---------------------------------------------------------------- parsers
def _parse_claude(line: str):
    """claude -p --output-format stream-json emits JSONL."""
    try:
        ev = json.loads(line)
    except json.JSONDecodeError:
        return []  # ignore stray lines
    out = []
    t = ev.get("type")
    if t == "system" and ev.get("subtype") == "init":
        if ev.get("session_id"):
            out.append(("ref", ev["session_id"]))
    elif t == "assistant":
        for block in ev.get("message", {}).get("content", []):
            if block.get("type") == "text" and block.get("text"):
                out.append(("text", block["text"]))
            elif block.get("type") == "tool_use":
                name = block.get("name", "tool")
                args = block.get("input", {})
                detail = args.get("command") or args.get("file_path") \
                    or json.dumps(args, ensure_ascii=False)[:120]
                out.append(("tool", name, "start", str(detail)[:200]))
    elif t == "result":
        if ev.get("session_id"):
            out.append(("ref", ev["session_id"]))
        if ev.get("subtype") == "success" and ev.get("result"):
            out.append(("final", ev["result"]))
        elif ev.get("subtype") != "success":
            out.append(("error", ev.get("result") or "task failed"))
    return out


def _parse_codex(line: str):
    """codex exec --json emits JSONL (thread.started / item.* / turn.*)."""
    try:
        ev = json.loads(line)
    except json.JSONDecodeError:
        return []
    out = []
    t = ev.get("type")
    if t == "thread.started" and ev.get("thread_id"):
        out.append(("ref", ev["thread_id"]))
    elif t == "item.completed":
        item = ev.get("item", {})
        it = item.get("type")
        if it == "agent_message" and item.get("text"):
            out.append(("text", item["text"]))
            out.append(("final", item["text"]))
        elif it == "command_execution":
            detail = item.get("command") or ""
            out.append(("tool", "bash", "end",
                        f"$ {detail}"[:200] + (
                            f" → exit {item['exit_code']}"
                            if item.get("exit_code") is not None else "")))
        elif it == "file_change":
            out.append(("tool", "edit", "end", str(item.get("path", ""))[:200]))
        elif it in ("mcp_tool_call", "web_search"):
            out.append(("tool", it, "end", str(item.get("name", ""))[:200]))
    elif t == "turn.failed":
        out.append(("error", json.dumps(ev.get("error", "failed"),
                                        ensure_ascii=False)[:400]))
    return out


def _parse_pi(line: str):
    """pi --mode json emits JSONL per docs/json.md."""
    try:
        ev = json.loads(line)
    except json.JSONDecodeError:
        return []
    out = []
    t = ev.get("type")
    if t == "session" and ev.get("id"):
        out.append(("ref", ev["id"]))
    elif t == "message_update":
        ame = ev.get("assistantMessageEvent", {})
        if ame.get("type") == "text_delta" and ame.get("delta"):
            out.append(("text", ame["delta"]))
    elif t == "tool_execution_start":
        args = ev.get("args", {})
        detail = args.get("cmd") or args.get("path") \
            or json.dumps(args, ensure_ascii=False)[:120]
        out.append(("tool", ev.get("toolName", "tool"), "start",
                    str(detail)[:200]))
    elif t == "tool_execution_end":
        out.append(("tool", ev.get("toolName", "tool"), "end",
                    "error" if ev.get("isError") else "ok"))
    elif t == "agent_end":
        texts = []
        for m in ev.get("messages", []):
            if m.get("role") == "assistant":
                for c in m.get("content", []):
                    if c.get("type") == "text" and c.get("text"):
                        texts.append(c["text"])
        if texts:
            out.append(("final", "\n\n".join(texts)))
    return out


def _parse_grok(line: str):
    """grok --output-format streaming-json emits JSONL deltas."""
    try:
        ev = json.loads(line)
    except json.JSONDecodeError:
        return []
    out = []
    t = ev.get("type")
    if t == "text" and ev.get("data"):
        out.append(("text", ev["data"]))
    elif t and t.startswith("tool"):
        out.append(("tool", t, "end", str(ev.get("data", ""))[:200]))
    if ev.get("sessionId"):  # any event may carry it
        out.append(("ref", ev["sessionId"]))
    return out


def _parse_opencode(line: str):
    """opencode run --format json emits JSONL (lenient parsing)."""
    try:
        ev = json.loads(line)
    except json.JSONDecodeError:
        return []
    out = []
    t = ev.get("type")
    if t == "error":
        err = ev.get("error", {})
        data = err.get("data", {}) if isinstance(err, dict) else {}
        msg = data.get("message") or (err.get("message") if isinstance(err, dict)
                                      else str(err)) or "error"
        out.append(("error", str(msg)[:400]))
    elif t in ("session.id", "sessionID") and ev.get("sessionID"):
        out.append(("ref", ev["sessionID"]))
    elif t == "message.part.updated":
        part = ev.get("part", {})
        if part.get("type") == "text" and part.get("text"):
            out.append(("text", part["text"]))
        elif part.get("type") == "tool":
            out.append(("tool", part.get("tool", "tool"), "end",
                        str(part.get("state", {}).get("input", ""))[:200]))
    elif t == "message.updated":
        # final full message: use as authoritative final if it has text
        info = ev.get("info", {})
        parts = info.get("parts", [])
        texts = [p.get("text", "") for p in parts
                 if isinstance(p, dict) and p.get("type") == "text"]
        joined = "".join(texts).strip()
        if ev.get("partID") is None and joined:
            out.append(("final", joined))
    return out


class RawParser:
    """Agents without JSON streaming: raw stdout text (grok)."""

    def __init__(self):
        self.buf = []

    def __call__(self, line: str):
        self.buf.append(line)
        return [("text", line + "\n")]

    @property
    def final(self):
        return "".join(self.buf).strip()


# ---------------------------------------------------------------- registry
def _build_registry():
    reg = {}
    if p := _which("claude"):
        def claude_cmd(text, ref, cwd, has_history, p=p):
            cmd = [p, "-p", text, "--output-format", "stream-json",
                   "--verbose", "--add-dir", str(HOME)]
            if ref:
                cmd += ["--resume", ref]
            return cmd
        reg["claude"] = dict(label="Claude", color="#E07856", bin=p,
                             cmd=claude_cmd, parser=_parse_claude, raw=False)
    if p := _which("codex"):
        def codex_cmd(text, ref, cwd, has_history, p=p):
            if ref:
                return [p, "exec", "resume", ref, "--json",
                        "--skip-git-repo-check", "-s", "workspace-write", text]
            return [p, "exec", "--json", "--skip-git-repo-check",
                    "-s", "workspace-write", text]
        reg["codex"] = dict(label="Codex", color="#33B08C", bin=p,
                            cmd=codex_cmd, parser=_parse_codex, raw=False)
    if p := _which("grok"):
        def grok_cmd(text, ref, cwd, has_history, p=p):
            cmd = [p, "--output-format", "streaming-json"]
            if ref:                      # native resume by session id
                cmd += ["--resume", ref]
            elif has_history:            # fallback: continue per-cwd session
                cmd += ["--continue"]
            return cmd + ["-p", text]
        reg["grok"] = dict(label="Grok", color="#C9CEDC", bin=p,
                           cmd=grok_cmd, parser=_parse_grok, raw=False)
    if p := _which("pi"):
        def pi_cmd(text, ref, cwd, has_history, p=p):
            cmd = [p, "-p", "--mode", "json"]
            if ref:
                cmd += ["--session", ref]
            return cmd + [text]
        reg["pi"] = dict(label="Pi", color="#7DA2F7", bin=p,
                         cmd=pi_cmd, parser=_parse_pi, raw=False)
    if p := _which("opencode"):
        def opencode_cmd(text, ref, cwd, has_history, p=p):
            cmd = [p, "run", "--format", "json", "--dir", cwd]
            if ref:
                cmd += ["--session", ref]
            return cmd + [text]
        reg["opencode"] = dict(label="OpenCode", color="#E5C558", bin=p,
                               cmd=opencode_cmd, parser=_parse_opencode,
                               raw=False)
    return reg


REGISTRY = _build_registry()


def get_agent(agent_id: str):
    return REGISTRY.get(agent_id)


def list_agents():
    return [
        {"id": k, "label": v["label"], "color": v["color"]}
        for k, v in REGISTRY.items()
    ]
