import { useEffect, useRef, useState, useCallback } from "react";
import { api, openEvents } from "./api.js";
import { Sidebar, Message, Composer, Logo } from "./components.jsx";
import { AgentIcon } from "./icons.jsx";
import { useTheme } from "./theme.js";

export function App() {
  const theme = useTheme();
  const [agents, setAgents] = useState([]);
  const [sessions, setSessions] = useState([]);
  const [activeId, setActiveId] = useState(null);
  const [messages, setMessages] = useState([]);
  const [stream, setStream] = useState(null);
  const [running, setRunning] = useState(false);
  const [status, setStatus] = useState("idle"); // idle | running | waiting
  const [queuedCount, setQueuedCount] = useState(0);
  const [permission, setPermission] = useState(null); // pending approval (ADR-0004)
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

  // ---- URL routing (G4): /s/<id> with working back/forward ----
  const sidFromURL = () => {
    const m = location.pathname.match(/^\/s\/([A-Za-z0-9]+)/);
    if (m) return m[1];
    return new URLSearchParams(location.search).get("s"); // legacy ?s=
  };

  useEffect(() => {
    api.agents().then(setAgents).catch(() => {});
    refreshSessions();
    const s = sidFromURL();
    if (s) {
      setActiveId(s);
      if (location.search) // normalize legacy deep-links in place
        history.replaceState({}, "", `/s/${s}`);
    }
  }, []);

  useEffect(() => {
    const onPop = () => setActiveId(sidFromURL());
    addEventListener("popstate", onPop);
    return () => removeEventListener("popstate", onPop);
  }, []);

  useEffect(() => {
    const want = activeId ? `/s/${activeId}` : "/";
    if (location.pathname !== want) history.pushState({}, "", want);
  }, [activeId]);

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
      else if (ev.type === "permission") {
        setPermission({ request_id: ev.request_id, tool: ev.tool, input: ev.input });
      }
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
        theme={theme.current} onToggleTheme={theme.toggle}
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
              <h1 class="font-medium truncate text-[15px]" style={{ color: "var(--text-1)" }}>{active.title || "untitled"}</h1>
              <span class="text-xs shrink-0 hidden sm:inline" style={{ color: "var(--text-3)" }}>{agentMeta?.label || active.agent}</span>
            </>
          ) : (
            <span class="flex items-center gap-2 md:hidden text-sm font-medium" style={{ color: "var(--text-1)" }}><Logo size={17} /> AgentDeck</span>
          )}
          <div class="ml-auto flex items-center gap-2 shrink-0">
            {queuedCount > 0 && active && (
              <button
                onClick={() => api.clearQueue(activeId).then(() => setQueuedCount(0))}
                class="text-[11px]" style={{ color: "var(--accent-fg)" }}
                title="cancel queued messages"
              >
                {queuedCount} queued · cancel
              </button>
            )}
            {status === "waiting" && (
              <span class="flex items-center gap-1.5 text-xs" style={{ color: "var(--warn)" }}>
                <span class="h-1.5 w-1.5 rounded-full animate-pulse" style={{ background: "var(--warn)" }} />
                waiting
              </span>
            )}
            {status === "running" && (
              <span class="flex items-center gap-1.5 text-xs" style={{ color: "var(--ok)" }}>
                <span class="h-1.5 w-1.5 rounded-full animate-pulse" style={{ background: "var(--ok)" }} />
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
                <div class="text-center mt-20 text-sm" style={{ color: "var(--text-3)" }}>
                  send the first message to <span style={{ color: "var(--text-1)" }}>{agentMeta?.label}</span>
                </div>
              )}
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
                  {stream.tools.slice(-6).map((t, i) => (
                    <span key={i} class={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-mono ${t.state === "end" ? "" : "animate-pulse"}`}
                      style={{ border: "1px solid var(--border)", color: t.state === "end" ? "var(--text-3)" : "var(--warn)" }}
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
            class="absolute bottom-28 right-5 z-10 h-10 w-10 grid place-items-center rounded-full shadow-xl surface" style={{ color: "var(--text-2)" }}
            style={{ background: "var(--bg-raised)", border: "1px solid var(--border)" }}
            aria-label="jump to bottom"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14m-6-6 6 6 6-6"/></svg>
          </button>
        )}

        {permission && (
          <div class="px-3 md:px-6 pt-3 max-w-3xl mx-auto w-full">
            <div class="rounded-xl border px-4 py-3 flex flex-col sm:flex-row sm:items-center gap-3"
              style={{ background: "var(--warn-soft)", borderColor: "var(--warn-border)" }}>
              <div class="flex-1 min-w-0">
                <div class="text-sm font-medium flex items-center gap-2" style={{ color: "var(--warn)" }}>
                  <span class="h-2 w-2 rounded-full animate-pulse" style={{ background: "var(--warn)" }} />
                  {permission.tool} permission requested
                </div>
                <code class="block mt-1 text-[12px] font-mono truncate" style={{ color: "var(--text-3)" }}>{permission.input}</code>
              </div>
              <div class="flex gap-2 shrink-0">
                <button onClick={() => answerPermission("allow")}
                  class="h-9 px-4 rounded-lg text-sm font-medium transition-colors"
                  style={{ background: "var(--ok)", color: "#fff" }}>Allow</button>
                <button onClick={() => answerPermission("deny")}
                  class="h-9 px-4 rounded-lg text-sm font-medium border transition-colors"
                  style={{ borderColor: "var(--err-border)", color: "var(--err)" }}>Deny</button>
              </div>
            </div>
          </div>
        )}
        {active && (
          <Composer running={running} onSend={send} onStop={() => api.stop(activeId).then(refreshSessions)} sessionId={activeId} />
        )}
      </main>

      {toast && (
        <div class="fixed bottom-24 left-1/2 -translate-x-1/2 z-50 rounded-lg px-4 py-2.5 text-sm shadow-xl surface"
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
        <span class="h-1.5 w-1.5 rounded-full animate-bounce" style={{ background: "var(--text-3)" }} style={{ animationDelay: `${i * 150}ms` }} />
      ))}
    </span>
  );
}

function EmptyState({ agents, onNew }) {
  return (
    <div class="h-full flex items-center justify-center px-6">
      <div class="max-w-sm w-full text-center" style={{ transform: "translateY(-4vh)" }}>
        <h2 class="text-2xl font-semibold tracking-tight mb-2" style={{ color: "var(--text-1)" }}>AgentDeck</h2>
        <p class="text-[15px] leading-relaxed balance mb-9" style={{ color: "var(--text-2)" }}>
          Talk to your local coding agents from the browser — from anywhere.
        </p>

        {agents.length > 0 && (
          <>
            <p class="text-[11px] uppercase tracking-wider mb-3" style={{ color: "var(--text-3)" }}>pick an agent to get started</p>
            <div class="flex flex-wrap justify-center gap-2 max-w-[360px] mx-auto">
              {agents.map((a) => (
                <button
                  key={a.id}
                  onClick={() => onNew(a.id)}
                  class="chip flex flex-col items-center justify-center gap-2.5 h-[84px] w-[108px] rounded-xl text-sm active:scale-[0.97] transition-all"
                  style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }}
                >
                  <AgentIcon id={a.id} size={a.id === "pi" ? 23 : a.id === "opencode" ? 24 : 22} color={a.color} />
                  <span class="text-[13px]" style={{ color: "var(--text-2)" }}>{a.label}</span>
                </button>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
