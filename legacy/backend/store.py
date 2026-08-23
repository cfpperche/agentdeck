"""SQLite store: sessions + messages. WAL mode, single-connection + lock."""
import json
import re
import shutil
import sqlite3
import threading
import time
import uuid
from pathlib import Path

from .config import DATA_DIR, WORKSPACES_DIR


class Store:
    def __init__(self):
        self.db = DATA_DIR / "agentdeck.db"
        self.db.parent.mkdir(parents=True, exist_ok=True)
        self.lock = threading.Lock()
        self.conn = sqlite3.connect(self.db, check_same_thread=False)
        self.conn.row_factory = sqlite3.Row
        self.conn.execute("PRAGMA journal_mode=WAL")
        self._migrate()

    def _migrate(self):
        with self.lock, self.conn:
            self.conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS sessions(
                    id TEXT PRIMARY KEY,
                    agent TEXT NOT NULL,
                    title TEXT NOT NULL DEFAULT 'Nova sessão',
                    agent_ref TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );
                CREATE TABLE IF NOT EXISTS messages(
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    session_id TEXT NOT NULL,
                    role TEXT NOT NULL,
                    content TEXT NOT NULL,
                    meta TEXT,
                    created_at TEXT NOT NULL
                );
                CREATE INDEX IF NOT EXISTS idx_messages_session
                    ON messages(session_id, id);
                """
            )

    # -- sessions -----------------------------------------------------------
    def create_session(self, agent: str, title: str = None) -> dict:
        sid = uuid.uuid4().hex[:12]
        now = time.strftime("%Y-%m-%dT%H:%M:%S")
        ws = WORKSPACES_DIR / sid
        ws.mkdir(parents=True, exist_ok=True)
        with self.lock, self.conn:
            self.conn.execute(
                "INSERT INTO sessions(id, agent, title, created_at, updated_at)"
                " VALUES(?,?,?,?,?)",
                (sid, agent, title or "Nova sessão", now, now),
            )
        return self.get_session(sid)

    def get_session(self, sid: str) -> dict | None:
        with self.lock:
            row = self.conn.execute(
                "SELECT * FROM sessions WHERE id=?", (sid,)
            ).fetchone()
        return dict(row) if row else None

    def list_sessions(self) -> list[dict]:
        with self.lock:
            rows = self.conn.execute(
                "SELECT s.*," 
                " (SELECT COUNT(*) FROM messages m"
                "  WHERE m.session_id = s.id) AS message_count,"
                " (SELECT m.content FROM messages m"
                "  WHERE m.session_id = s.id ORDER BY m.id DESC LIMIT 1)"
                "   AS preview"
                " FROM sessions s ORDER BY s.updated_at DESC"
            ).fetchall()
        out = []
        for r in rows:
            d = dict(r)
            d["preview"] = self._clean_preview(d.get("preview"))
            out.append(d)
        return out

    @staticmethod
    def _clean_preview(text):
        """Sidebar preview: strip ANSI/banners; None when nothing useful."""
        if not text:
            return None
        t = re.sub(r"\x1b\[[0-9;]*[a-zA-Z]", "", text)   # ANSI escapes
        lines = [l.strip() for l in t.splitlines() if l.strip()]
        # first line that contains mostly normal text (letters/digits)
        for l in lines:
            letters = sum(c.isalnum() or c.isspace() for c in l)
            if letters / max(len(l), 1) > 0.7 and len(l) >= 4:
                return re.sub(r"\s+", " ", l)[:90]
        return None

    def rename_session(self, sid: str, title: str) -> dict | None:
        with self.lock, self.conn:
            self.conn.execute(
                "UPDATE sessions SET title=?, updated_at=? WHERE id=?",
                (title, time.strftime("%Y-%m-%dT%H:%M:%S"), sid),
            )
        return self.get_session(sid)

    def delete_session(self, sid: str) -> bool:
        with self.lock, self.conn:
            cur = self.conn.execute("DELETE FROM sessions WHERE id=?", (sid,))
            self.conn.execute("DELETE FROM messages WHERE session_id=?", (sid,))
        # remove workspace (best-effort)
        shutil.rmtree(WORKSPACES_DIR / sid, ignore_errors=True)
        return cur.rowcount > 0

    def set_agent_ref(self, sid: str, ref: str):
        with self.lock, self.conn:
            self.conn.execute(
                "UPDATE sessions SET agent_ref=? WHERE id=?", (ref, sid)
            )

    def touch(self, sid: str):
        with self.lock, self.conn:
            self.conn.execute(
                "UPDATE sessions SET updated_at=? WHERE id=?",
                (time.strftime("%Y-%m-%dT%H:%M:%S"), sid),
            )

    # -- messages -----------------------------------------------------------
    def add_message(
        self, sid: str, role: str, content: str, meta: dict | None = None
    ) -> dict:
        now = time.strftime("%Y-%m-%dT%H:%M:%S")
        with self.lock, self.conn:
            cur = self.conn.execute(
                "INSERT INTO messages(session_id, role, content, meta, created_at)"
                " VALUES(?,?,?,?,?)",
                (sid, role, content, json.dumps(meta) if meta else None, now),
            )
            self.conn.execute(
                "UPDATE sessions SET updated_at=? WHERE id=?", (now, sid)
            )
        return {
            "id": cur.lastrowid,
            "session_id": sid,
            "role": role,
            "content": content,
            "meta": meta,
            "created_at": now,
        }

    def list_messages(self, sid: str) -> list[dict]:
        with self.lock:
            rows = self.conn.execute(
                "SELECT * FROM messages WHERE session_id=? ORDER BY id", (sid,)
            ).fetchall()
        out = []
        for r in rows:
            d = dict(r)
            d["meta"] = json.loads(d["meta"]) if d["meta"] else None
            out.append(d)
        return out


store = Store()
