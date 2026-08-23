"""
Runner: spawns agent processes, parses their streams, broadcasts events
over SSE, persists results. One running process per session.
"""
import asyncio
import json
import time

from . import agents as A
from .config import WORKSPACES_DIR
from .store import store


class Runner:
    def __init__(self):
        self.procs: dict[str, asyncio.subprocess.Process] = {}
        self.queues: dict[str, set[asyncio.Queue]] = {}
        self.running: dict[str, bool] = {}

    # -- bus ------------------------------------------------------------
    def subscribe(self, sid: str) -> asyncio.Queue:
        q = asyncio.Queue()
        self.queues.setdefault(sid, set()).add(q)
        return q

    def unsubscribe(self, sid: str, q: asyncio.Queue):
        self.queues.get(sid, set()).discard(q)

    def _publish(self, sid: str, event: dict):
        for q in list(self.queues.get(sid, set())):
            q.put_nowait(event)

    def is_running(self, sid: str) -> bool:
        return self.running.get(sid, False)

    # -- execution --------------------------------------------------------
    async def send(self, sid: str, text: str) -> dict:
        session = store.get_session(sid)
        if not session:
            raise KeyError("session not found")
        if self.running.get(sid):
            raise RuntimeError("agent already running for this session")

        agent = A.get_agent(session["agent"])
        if not agent:
            raise KeyError(f"agent {session['agent']} unavailable")

        store.add_message(sid, "user", text)
        # auto-title: first message names the session (ChatGPT-style)
        if not session.get("title") or session["title"] == "Nova sessão":
            title = text.strip().replace("\n", " ")[:52]
            store.rename_session(sid, title)
        cwd = str(WORKSPACES_DIR / sid)
        # history BEFORE this message: did the agent already answer once?
        has_history = any(m["role"] == "assistant"
                          for m in store.list_messages(sid))
        cmd = agent["cmd"](text, session.get("agent_ref"), cwd, has_history)
        self.running[sid] = True
        self._publish(sid, {"type": "state", "running": True})

        proc = await asyncio.create_subprocess_exec(
            *cmd, cwd=cwd, stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.STDOUT,
        )
        self.procs[sid] = proc

        asyncio.create_task(self._pump(sid, agent, proc))
        return {"ok": True}

    async def _pump(self, sid: str, agent, proc):
        acc, tools, ref, final, error = [], [], None, None, None
        parser = agent["parser"]
        try:
            async for raw in proc.stdout:
                line = raw.decode("utf-8", "replace").rstrip("\n")
                if not line.strip():
                    continue
                try:
                    events = parser(line)
                except Exception:
                    events = []
                for ev in events:
                    kind = ev[0]
                    if kind == "ref":
                        ref = ev[1]
                        store.set_agent_ref(sid, ref)
                    elif kind == "text":
                        acc.append(ev[1])
                        self._publish(sid, {"type": "text", "content": ev[1]})
                    elif kind == "tool":
                        _, name, state, detail = ev
                        tools.append({"name": name, "state": state,
                                      "detail": detail})
                        self._publish(sid, {"type": "tool", "name": name,
                                            "state": state, "detail": detail})
                    elif kind == "final":
                        final = ev[1]
                    elif kind == "error":
                        error = ev[1]
            await proc.wait()
        except Exception as e:  # noqa: BLE001
            error = f"{type(e).__name__}: {e}"
        finally:
            if agent["raw"] and not final and not error:
                final = parser.final
            content = final or ("".join(acc).strip() or error or "(sem resposta)")
            meta = {"agent": store.get_session(sid)["agent"],
                    "tools": tools[-50:],
                    "error": bool(error)}
            msg = store.add_message(sid, "assistant", content, meta)
            self.running[sid] = False
            self.procs.pop(sid, None)
            self._publish(sid, {"type": "message_end", "message": msg})
            self._publish(sid, {"type": "state", "running": False})

    async def stop(self, sid: str):
        proc = self.procs.get(sid)
        if not proc:
            return {"ok": False, "error": "not running"}
        try:
            proc.terminate()
            await asyncio.wait_for(proc.wait(), 5)
        except Exception:
            proc.kill()
        return {"ok": True}


runner = Runner()
