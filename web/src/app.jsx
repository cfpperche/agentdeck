import { useEffect, useRef, useState, useCallback } from "react";
import { api, openEvents } from "./api.js";
import { Sidebar, Message, Composer, Logo } from "./components.jsx";
import { AgentIcon } from "./icons.jsx";

export function App() {
  const [agents, setAgents] = useState([]);
  const [sessions, setSessions] = useState([]);
  const [activeId, setActiveId] = useState(null);
  const [messages, setMessages] = useState([]);
  const [stream, setStream] = useState(null);
  const [running, setRunning] = useState(false);
  const [status, setStatus] = useState("idle"); // idle | running | waiting
  const [queuedCount, setQueuedCount] = useState(0);
  const [filter, setFilter] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [toast, setToast] = useState(null);
  const [atBottom, setAtBottom] = useState(true);
  const esRef = useRef(null);
  const listRef = useRef(null);

  const active = sessions.find((s) => s.id === activeId);
  const agentMeta = agents.find((a) => a?.id === active?.agent);

  const showToast = (msg) => { setToast(msg); setTimeout(() => setToast(null), 4000); };
  const refreshSessions = useCallback(() => {
    api.sessions().then(setSessions).catch(() => {});
  }, []);

  useEffect(() => {
    api.agents().then(setAgents).catch(() => {});
    refreshSessions();
    // deep-link support: /?s=<session-id>
    const s = new URLSearchParams(location.search).get("s");
    if (s) setActiveId(s);
  }, []);

  useEffect(() => {
    esRef.current?.close();
    setStream(null); setRunning(false); setStatus("idle"); setQueuedCount(0);
    if (!activeId) { setMessages([]); return; }
    api.messages(activeId).then(setMessages).catch(() => {});
    esRef.current = openEvents(activeId, (ev) => {
      if (ev.type === "state") {
        setRunning(ev.running);
        setStatus(ev.status || (ev.running ? "running" : "idle"));
      }
      else if (ev.type === "queue") setQueuedCount(ev.count || 0);
      else if (ev.type === "text")
        setStream((s) => ({ text: (s?.text || "") + ev.content, tools: s?.tools || [] }));
      else if (ev.type === "tool")
        setStream((s) => ({ text: s?.text || "", tools: [...(s?.tools || []), { name: ev.name, state: ev.state }] }));
      else if (ev.type === "message_end") {
        // resync from the server: queued tags resolve once delivered
        api.messages(activeId).then(setMessages).catch(() => {});
        setStream(null);
        setQueuedCount((q) => Math.max(0, q - 1));
        refreshSessions();
      }
    });
    return () => esRef.current?.close();
  }, [activeId]);

  useEffect(() => {
    if (atBottom && listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight;
  }, [messages, stream]);

  const onScroll = () => {
    const el = listRef.current;
    if (!el) return;
    setAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 80);
  };

  const newSession = (agentId) => {
    const agent = agentId || active?.agent || agents[0]?.id;
    if (!agent) return showToast("no agent installed");
    api.createSession(agent)
      .then((s) => { refreshSessions(); setActiveId(s.id); })
      .catch((e) => showToast(e.detail || "failed to create session"));
  };

  const send = (text) => {
    api.send(activeId, text)
      .then((res) => {
        setMessages((m) => [
          ...m,
          { role: "user", content: text, meta: res.queued ? { queued: true } : null, id: Date.now() },
        ]);
        if (res.queued) setQueuedCount((q) => q + 1);
        setRunning(true);
        setStatus("running");
        setAtBottom(true);
      })
      .catch((e) => showToast(e.detail || "failed to send"));
  };

  const answerPermission = (behavior) => {
    if (!permission) return;
    api.control(activeId, permission.request_id, behavior)
      .then(() => setPermission(null))
      .catch((e) => showToast(e.detail || "failed to answer"));
  };

  return (
    <div class="flex h-full overflow-hidden">
      <Sidebar
        sessions={sessions} agents={agents} activeId={activeId}
        filter={filter} setFilter={setFilter}
        onOpen={setActiveId} onNew={() => newSession()}
        onRename={(id, t) => api.renameSession(id, t).then(refreshSessions)}
        onDelete={(id) => api.deleteSession(id).then(() => { if (id === activeId) setActiveId(null); refreshSessions(); })}
        open={sidebarOpen} setOpen={setSidebarOpen}
      />

      <main class="flex-1 flex flex-col min-w-0 relative" style={{ background: "var(--bg-canvas)" }}>
        {/* header */}
        <header
          class="flex items-center gap-3 h-14 px-4 shrink-0 surface z-10"
          style={{ background: "var(--bg-panel)", borderBottom: "1px solid var(--border-soft)" }}
        >
          <button class="md:hidden text-zinc-400 p-1.5 -ml-1.5" onClick={() => setSidebarOpen(true)} aria-label="menu">
            <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 6h16M4 12h16M4 18h16"/></svg>
          </button>
          {active ? (
            <>
              <AgentIcon id={active.agent} size={16} color={agentMeta?.color || "#888"} />
              <h1 class="font-medium text-zinc-200 truncate text-[15px]">{active.title || "untitled"}</h1>
              <span class="text-xs text-zinc-500 shrink-0 hidden sm:inline">{agentMeta?.label || active.agent}</span>
            </>
          ) : (
            <span class="flex items-center gap-2 md:hidden text-zinc-300 text-sm font-medium"><Logo size={17} /> AgentDeck</span>
          )}
          <div class="ml-auto flex items-center gap-2 shrink-0">
            {queuedCount > 0 && active && (
              <button
                onClick={() => api.clearQueue(activeId).then(() => setQueuedCount(0))}
                class="text-[11px] text-sky-300/90 hover:text-sky-200"
                title="cancel queued messages"
              >
                {queuedCount} queued · cancel
              </button>
            )}
            {status === "waiting" && (
              <span class="flex items-center gap-1.5 text-xs text-amber-400">
                <span class="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse" />
                waiting
              </span>
            )}
            {status === "running" && (
              <span class="flex items-center gap-1.5 text-xs text-emerald-400">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                running
              </span>
            )}
          </div>
        </header>

        {/* content area */}
        <div ref={listRef} onScroll={onScroll} class="flex-1 overflow-y-auto">
          {!active ? (
            <EmptyState agents={agents} onNew={newSession} />
          ) : (
            <div class="max-w-3xl mx-auto w-full px-4 md:px-6 py-6">
              {messages.length === 0 && !stream && (
                <div class="text-center mt-20 text-sm text-zinc-500">
                  send the first message to <span class="text-zinc-300">{agentMeta?.label}</span>
                </div>
              )}
              {messages.map((m) => <Message key={m.id || m.created_at} m={m} />)}

              {stream && (
                <div class="flex justify-start mb-5">
                  <div class="max-w-[90%] md:max-w-[75%] rounded-2xl rounded-bl-md px-4 py-3"
                    style={{ background: "var(--bg-card)", border: "1px solid var(--border-soft)" }}>
                    {stream.text ? (
                      <div class="md caret text-[15px] leading-relaxed whitespace-pre-wrap text-zinc-200">{stream.text}</div>
                    ) : (
                      <span class="text-sm text-zinc-500 flex items-center gap-2">
                        <Dots /> {agentMeta?.label || "agent"} working…
                      </span>
                    )}
                  </div>
                </div>
              )}
              {stream?.tools?.length > 0 && (
                <div class="flex flex-wrap gap-1.5 -mt-3 mb-5">
                  {stream.tools.slice(-6).map((t, i) => (
                    <span key={i} class={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-mono ${t.state === "end" ? "text-zinc-400" : "text-amber-400/90 animate-pulse"}`}
                      style={{ border: "1px solid var(--border)" }}>
                      {t.state === "end" ? "✓" : "⟳"} {t.name}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {!atBottom && messages.length > 0 && (
          <button
            onClick={() => { setAtBottom(true); listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: "smooth" }); }}
            class="absolute bottom-28 right-5 z-10 h-10 w-10 grid place-items-center rounded-full text-zinc-300 shadow-xl surface"
            style={{ background: "var(--bg-raised)", border: "1px solid var(--border)" }}
            aria-label="jump to bottom"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14m-6-6 6 6 6-6"/></svg>
          </button>
        )}

        {permission && (
          <div class="px-3 md:px-6 pt-3 max-w-3xl mx-auto w-full">
            <div class="rounded-xl border px-4 py-3 flex flex-col sm:flex-row sm:items-center gap-3"
              style={{ background: "rgba(217,119,55,0.08)", borderColor: "rgba(217,119,55,0.35)" }}>
              <div class="flex-1 min-w-0">
                <div class="text-sm font-medium text-amber-300/90 flex items-center gap-2">
                  <span class="h-2 w-2 rounded-full bg-amber-400 animate-pulse" />
                  {permission.tool} permission requested
                </div>
                <code class="block mt-1 text-[12px] text-zinc-400 font-mono truncate">{permission.input}</code>
              </div>
              <div class="flex gap-2 shrink-0">
                <button onClick={() => answerPermission("allow")}
                  class="h-9 px-4 rounded-lg text-sm font-medium text-white transition-colors hover:brightness-110"
                  style={{ background: "#2c8a4b" }}>Allow</button>
                <button onClick={() => answerPermission("deny")}
                  class="h-9 px-4 rounded-lg text-sm font-medium border transition-colors hover:bg-red-950/40"
                  style={{ borderColor: "#7f1d1d", color: "#fca5a5" }}>Deny</button>
              </div>
            </div>
          </div>
        )}
        {active && (
          <Composer running={running} onSend={send} onStop={() => api.stop(activeId).then(refreshSessions)} />
        )}
      </main>

      {toast && (
        <div class="fixed bottom-24 left-1/2 -translate-x-1/2 z-50 rounded-lg px-4 py-2.5 text-sm shadow-xl surface"
          style={{ background: "#2a1215", border: "1px solid #7f1d1d", color: "#fca5a5" }}>
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
        <span class="h-1.5 w-1.5 rounded-full bg-zinc-500 animate-bounce" style={{ animationDelay: `${i * 150}ms` }} />
      ))}
    </span>
  );
}

function EmptyState({ agents, onNew }) {
  return (
    <div class="h-full flex items-center justify-center px-6">
      <div class="max-w-sm w-full text-center" style={{ transform: "translateY(-4vh)" }}>
        <h2 class="text-2xl font-semibold text-zinc-100 tracking-tight mb-2">AgentDeck</h2>
        <p class="text-[15px] text-zinc-400 leading-relaxed balance mb-9">
          Talk to your local coding agents from the browser — from anywhere.
        </p>

        {agents.length > 0 && (
          <>
            <p class="text-[11px] uppercase tracking-wider text-zinc-400 mb-3">pick an agent to get started</p>
            <div class="flex flex-wrap justify-center gap-2 max-w-[360px] mx-auto">
              {agents.map((a) => (
                <button
                  key={a.id}
                  onClick={() => onNew(a.id)}
                  class="flex flex-col items-center justify-center gap-2.5 h-[84px] w-[108px] rounded-xl text-sm text-zinc-300 surface hover:text-zinc-100 hover:bg-white/[0.03] active:scale-[0.97] transition-all"
                  style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }}
                >
                  <AgentIcon id={a.id} size={a.id === "pi" ? 23 : a.id === "opencode" ? 24 : 22} color={a.color} />
                  <span class="text-[13px]">{a.label}</span>
                </button>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
