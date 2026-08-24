import { useEffect, useRef, useState, useCallback } from "react";
import { api, openEvents } from "./api.js";
import { Message, Composer, ToolChip } from "./components.jsx";

// Chat: one live session view (messages + SSE + composer + permission
// banner). Designed to stay MOUNTED while hidden (tab switching) so
// streams/drafts/permissions survive.
export function Chat({ session, agentMeta, onOpenSidebar, onHide }) {
  const sid = session.id;
  const [messages, setMessages] = useState(null); // null = loading
  const [stream, setStream] = useState(null);
  const [running, setRunning] = useState(false);
  const [status, setStatus] = useState("idle");
  const [queuedCount, setQueuedCount] = useState(0);
  const [permissions, setPermissions] = useState([]);
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState("");
  const [toast, setToast] = useState(null);
  const [atBottom, setAtBottom] = useState(true);
  const esRef = useRef(null);
  const listRef = useRef(null);
  const shown = !onHide; // visible when no hide callback (active tab)

  const showToast = useCallback((msg) => {
    setToast(msg);
    setTimeout(() => setToast(null), 4000);
  }, []);

  useEffect(() => {
    setMessages(null);
    api.messages(sid).then(setMessages).catch(() => setMessages([]));
    esRef.current?.close();
    const es = openEvents(sid, (ev) => {
      if (ev.type === "state") {
        setRunning(ev.running);
        setStatus(ev.status || (ev.running ? "running" : "idle"));
      } else if (ev.type === "queue") setQueuedCount(ev.count || 0);
      else if (ev.type === "permission") {
        setPermissions((q) => [...q, { request_id: ev.request_id, tool: ev.tool, input: ev.input }]);
      } else if (ev.type === "text") {
        setStream((s) => ({ text: (s?.text || "") + ev.content, tools: s?.tools || [] }));
      } else if (ev.type === "tool") {
        setStream((s) => ({ text: s?.text || "", tools: [...(s?.tools || []), { name: ev.name, state: ev.state, detail: ev.detail }] }));
      } else if (ev.type === "message_end") {
        api.messages(sid).then(setMessages).catch(() => {});
        setStream(null);
        setQueuedCount((q) => Math.max(0, q - 1));
      }
    });
    esRef.current = es;
    return () => es.close();
  }, [sid]);

  useEffect(() => {
    if (shown && atBottom && listRef.current)
      listRef.current.scrollTop = listRef.current.scrollHeight;
  }, [messages, stream, shown]);

  const onScroll = () => {
    const el = listRef.current;
    if (!el) return;
    setAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 80);
  };

  const send = (text) => {
    api.send(sid, text)
      .then((res) => {
        setMessages((m) => [...(m || []), { role: "user", content: text, meta: res.queued ? { queued: true } : null, id: Date.now() }]);
        if (res.queued) setQueuedCount((q) => q + 1);
        setRunning(true);
        setStatus("running");
        setAtBottom(true);
      })
      .catch((e) => showToast(e.detail || "failed to send"));
  };

  const answerPermission = (behavior) => {
    const p = permissions[0];
    if (!p) return;
    let updatedInput;
    if (behavior === "allow" && editing && editText.trim()) {
      try { updatedInput = JSON.parse(editText); }
      catch { showToast("edited input is not valid JSON"); return; }
    }
    api.control(sid, p.request_id, behavior, updatedInput)
      .then(() => { setPermissions((q) => q.slice(1)); setEditing(false); setEditText(""); })
      .catch((e) => showToast(e.detail || "failed to answer"));
  };
  const startEditing = () => {
    const p = permissions[0];
    if (!p) return;
    setEditing(true);
    setEditText(p.input || "{}");
  };

  return (
    <div class="flex-1 flex flex-col min-w-0 relative" style={{ background: "var(--bg-canvas)" }}>
      {/* toolbar */}
      <header class="flex items-center gap-3 h-12 px-4 shrink-0 surface"
        style={{ background: "var(--bg-panel)", borderBottom: "1px solid var(--border-soft)" }}>
        {onOpenSidebar && (
          <button class="md:hidden p-1.5 -ml-1.5" style={{ color: "var(--text-2)" }} onClick={onOpenSidebar} aria-label="menu">
            <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 6h16M4 12h16M4 18h16"/></svg>
          </button>
        )}
        <span class="h-2 w-2 rounded-full shrink-0" style={{ background: agentMeta?.color || "#888" }} />
        <h1 class="font-medium truncate text-[14px]" style={{ color: "var(--text-1)" }}>{session.title || "untitled"}</h1>
        <span class="text-xs px-2 py-0.5 rounded-md shrink-0" style={{ color: "var(--text-2)", background: "var(--bg-raised)" }}>
          {agentMeta?.label || session.agent}
        </span>
        <div class="ml-auto flex items-center gap-2 shrink-0">
          {queuedCount > 0 && (
            <button onClick={() => api.clearQueue(sid).then(() => setQueuedCount(0))}
              class="text-[11px]" style={{ color: "var(--accent-fg)" }}>
              {queuedCount} queued · cancel
            </button>
          )}
          {status === "waiting" && (
            <span class="flex items-center gap-1.5 text-xs" style={{ color: "var(--warn)" }}>
              <span class="h-1.5 w-1.5 rounded-full animate-pulse" style={{ background: "var(--warn)" }} />waiting
            </span>
          )}
          {status === "running" && (
            <span class="flex items-center gap-1.5 text-xs" style={{ color: "var(--ok)" }}>
              <span class="h-1.5 w-1.5 rounded-full animate-pulse" style={{ background: "var(--ok)" }} />running
            </span>
          )}
        </div>
      </header>

      {/* messages */}
      <div ref={listRef} onScroll={onScroll} class="flex-1 overflow-y-auto" style={{ display: "flex", flexDirection: "column" }}>
        <div class="w-full max-w-3xl mx-auto px-4 md:px-6 py-6 flex-1 flex flex-col justify-end">
          {messages === null ? (
            <p class="text-center text-sm py-16" style={{ color: "var(--text-3)" }}>loading…</p>
          ) : messages.length === 0 && !stream ? (
            <p class="text-center text-sm py-16" style={{ color: "var(--text-3)" }}>
              send the first message to <span style={{ color: "var(--text-1)" }}>{agentMeta?.label}</span>
            </p>
          ) : (
            <>
              {messages.map((m) => <Message key={m.id || m.created_at} m={m} />)}
              {stream && (
                <div class="flex justify-start mb-5">
                  <div class="max-w-[90%] md:max-w-[75%] rounded-2xl rounded-bl-md px-4 py-3"
                    style={{ background: "var(--bg-card)", border: "1px solid var(--border-soft)" }}>
                    {stream.text ? (
                      <div class="md caret text-[15px] leading-relaxed whitespace-pre-wrap" style={{ color: "var(--text-1)" }}>{stream.text}</div>
                    ) : (
                      <span class="text-sm flex items-center gap-2" style={{ color: "var(--text-3)" }}>
                        <Dots /> {agentMeta?.label || "agent"} working…
                      </span>
                    )}
                  </div>
                </div>
              )}
              {stream?.tools?.length > 0 && (
                <div class="flex flex-wrap gap-1.5 -mt-3 mb-5">
                  {stream.tools.slice(-6).map((t, i) => <ToolChip key={i} tool={t} />)}
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* permission banner (queue + edit) */}
      {permissions.length > 0 && (() => {
        const p = permissions[0];
        return (
          <div class="px-4 md:px-6 pb-2 max-w-3xl mx-auto w-full">
            <div class="rounded-xl border px-4 py-3 flex flex-col gap-3"
              style={{ background: "var(--warn-soft)", borderColor: "var(--warn-border)" }}>
              <div class="flex items-start gap-3">
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-medium flex items-center gap-2" style={{ color: "var(--warn)" }}>
                    <span class="h-2 w-2 rounded-full animate-pulse" style={{ background: "var(--warn)" }} />
                    {p.tool} permission requested
                    {permissions.length > 1 && (
                      <span class="text-[11px] font-normal" style={{ color: "var(--text-2)" }}>
                        1 of {permissions.length} — answer to continue
                      </span>
                    )}
                  </div>
                  <code class="block mt-1 text-[12px] font-mono truncate" style={{ color: "var(--text-2)" }}>{p.input}</code>
                </div>
                <button onClick={startEditing}
                  class="text-[11px] shrink-0 h-7 px-2.5 rounded-md surface"
                  style={{ color: "var(--text-2)", border: "1px solid var(--border)" }}>
                  {editing ? "cancel edit" : "edit input"}
                </button>
              </div>
              {editing && (
                <div>
                  <textarea value={editText} onInput={(e) => setEditText(e.target.value)} rows="3" spellCheck="false"
                    class="w-full rounded-lg px-3 py-2 text-[12px] font-mono focus:outline-none"
                    style={{ background: "var(--code-bg)", color: "var(--text-1)", border: "1px solid var(--border)" }} />
                  <p class="text-[11px] mt-1" style={{ color: "var(--text-3)" }}>edited JSON is sent as updatedInput on Allow</p>
                </div>
              )}
              <div class="flex gap-2 justify-end">
                <button onClick={() => answerPermission("allow")}
                  class="h-9 px-4 rounded-lg text-sm font-medium transition-colors"
                  style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}>
                  {editing ? "Allow with edits" : "Allow"}
                </button>
                <button onClick={() => answerPermission("deny")}
                  class="h-9 px-4 rounded-lg text-sm font-medium border transition-colors"
                  style={{ borderColor: "var(--err-border)", color: "var(--err)" }}>Deny</button>
              </div>
            </div>
          </div>
        );
      })()}

      {/* jump to bottom */}
      {!atBottom && messages?.length > 0 && (
        <button onClick={() => { setAtBottom(true); listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: "smooth" }); }}
          class="absolute bottom-28 right-5 z-10 h-10 w-10 grid place-items-center rounded-full shadow-xl surface"
          style={{ background: "var(--bg-raised)", border: "1px solid var(--border)", color: "var(--text-2)" }}
          aria-label="jump to bottom">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14m-6-6 6 6 6-6"/></svg>
        </button>
      )}

      <Composer running={running} onSend={send} onStop={() => api.stop(sid)} sessionId={sid} />

      {toast && (
        <div class="absolute bottom-24 left-1/2 -translate-x-1/2 z-50 rounded-lg px-4 py-2.5 text-sm shadow-xl surface"
          style={{ background: "var(--err-soft)", border: "1px solid var(--err-border)", color: "var(--err)" }}>
          {toast}
        </div>
      )}
    </div>
  );
}

function Dots() {
  return (
    <span class="inline-flex gap-1">
      {[0, 1, 2].map((i) => (
        <span class="h-1.5 w-1.5 rounded-full animate-bounce" style={{ background: "var(--text-3)", animationDelay: `${i * 150}ms` }} />
      ))}
    </span>
  );
}
