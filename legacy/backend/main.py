"""AgentDeck — FastAPI app: session CRUD, message sending, SSE streaming."""
import asyncio
import json

from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel

from . import agents as A
from .runner import runner
from .store import store

app = FastAPI(title="AgentDeck")


class SessionIn(BaseModel):
    agent: str
    title: str | None = None


class RenameIn(BaseModel):
    title: str


class MessageIn(BaseModel):
    text: str


# -- agents ---------------------------------------------------------------
@app.get("/api/agents")
def get_agents():
    return A.list_agents()


# -- sessions ---------------------------------------------------------------
@app.get("/api/sessions")
def get_sessions():
    return store.list_sessions()


@app.post("/api/sessions")
def create_session(body: SessionIn):
    if not A.get_agent(body.agent):
        raise HTTPException(400, f"unknown agent: {body.agent}")
    return store.create_session(body.agent, body.title)


@app.patch("/api/sessions/{sid}")
def rename_session(sid: str, body: RenameIn):
    s = store.rename_session(sid, body.title.strip() or "Nova sessão")
    if not s:
        raise HTTPException(404, "session not found")
    return s


@app.delete("/api/sessions/{sid}")
async def delete_session(sid: str):
    if runner.is_running(sid):
        await runner.stop(sid)
    if not store.delete_session(sid):
        raise HTTPException(404, "session not found")
    return {"ok": True}


@app.get("/api/sessions/{sid}/messages")
def get_messages(sid: str):
    if not store.get_session(sid):
        raise HTTPException(404, "session not found")
    return store.list_messages(sid)


@app.post("/api/sessions/{sid}/messages")
async def send_message(sid: str, body: MessageIn):
    text = body.text.strip()
    if not text:
        raise HTTPException(400, "empty message")
    try:
        return await runner.send(sid, text)
    except KeyError as e:
        raise HTTPException(404, str(e)) from e
    except RuntimeError as e:
        raise HTTPException(409, str(e)) from e


@app.post("/api/sessions/{sid}/stop")
async def stop_session(sid: str):
    return await runner.stop(sid)


# -- SSE stream -------------------------------------------------------------
@app.get("/api/sessions/{sid}/events")
async def session_events(sid: str):
    if not store.get_session(sid):
        raise HTTPException(404, "session not found")
    q = runner.subscribe(sid)

    async def gen():
        try:
            # state snapshot on connect
            yield f"data: {json.dumps({'type': 'state', 'running': runner.is_running(sid)})}\n\n"
            while True:
                try:
                    ev = await asyncio.wait_for(q.get(), timeout=15)
                    yield f"data: {json.dumps(ev)}\n\n"
                except asyncio.TimeoutError:
                    yield ": heartbeat\n\n"
        finally:
            runner.unsubscribe(sid, q)

    return StreamingResponse(
        gen(), media_type="text/event-stream",
        headers={"Cache-Control": "no-store", "X-Accel-Buffering": "no"},
    )


# -- static frontend (built via `make build`) --------------------------------
static = StaticFiles(directory="web/dist", html=True, check_dir=False)
app.mount("/", static, name="frontend")
