import { useEffect, useRef, useState } from "react";
import { Markdown } from "./markdown.jsx";
import { AgentIcon } from "./icons.jsx";

/* ------------------------------------------------------------------ logo */
export function Logo({ size = 20 }) {
  return (
    <svg
      width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke="currentColor" stroke-width="1.7" stroke-linecap="round"
      style={{ stroke: "var(--logo-stroke)" }} class="shrink-0"
    >
      <path d="M5 7h14M5 12h14M5 17h14" opacity="0.45" />
      <circle cx="10" cy="7" r="2.4" fill="var(--bg-panel)" />
      <circle cx="15.5" cy="12" r="2.4" fill="var(--bg-panel)" />
      <circle cx="8" cy="17" r="2.4" fill="var(--bg-panel)" />
    </svg>
  );
}

/* ---------------------------------------------------------------- icons */
const I = {
  plus: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>,
  search: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>,
  pencil: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 3a2.8 2.8 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg>,
  trash: <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4h8v2m1 0v14a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V6"/></svg>,
  send: <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>,
  stop: <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor"><rect x="5" y="5" width="14" height="14" rx="2.5"/></svg>,
  menu: <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 6h16M4 12h16M4 18h16"/></svg>,
  x: <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>,
  down: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14m-6-6 6 6 6-6"/></svg>,
};

/* -------------------------------------------------------------- helpers */
const relTime = (iso) => {
  const d = (Date.now() - new Date(iso + "Z")) / 1000;
  if (d < 60) return "now";
  if (d < 3600) return `${Math.floor(d / 60)}m`;
  if (d < 86400) return `${Math.floor(d / 3600)}h`;
  return `${Math.floor(d / 86400)}d`;
};

const groupOf = (iso) => {
  const d = (Date.now() - new Date(iso + "Z")) / 1000;
  if (d < 86400) return "Today";
  if (d < 172800) return "Yesterday";
  if (d < 604800) return "Last 7 days";
  return "Older";
};

/* --------------------------------------------------------------- sidebar */
export function Sidebar({
  sessions, agents, activeId, filter, setFilter,
  onOpen, onNew, onRename, onDelete, open, setOpen, theme, onToggleTheme,
}) {
  const [editing, setEditing] = useState(null);
  const [editTitle, setEditTitle] = useState("");
  const [confirmDel, setConfirmDel] = useState(null);
  const agentById = Object.fromEntries(agents.map((a) => [a.id, a]));

  const filtered = sessions.filter((s) =>
    (s.title || "").toLowerCase().includes(filter.toLowerCase())
  );

  // group by time bucket, preserving order
  const groups = [];
  for (const s of filtered) {
    const g = groupOf(s.updated_at);
    const last = groups[groups.length - 1];
    if (last && last.name === g) last.items.push(s);
    else groups.push({ name: g, items: [s] });
  }

  const commitRename = (s) => {
    if (editTitle.trim() && editTitle.trim() !== s.title) onRename(s.id, editTitle.trim());
    setEditing(null);
  };

  return (
    <>
      {open && (
        <div class="fixed inset-0 bg-black/60 backdrop-blur-sm z-30 md:hidden" onClick={() => setOpen(false)} />
      )}
      <aside
        class={`fixed md:static z-40 inset-y-0 left-0 w-[290px] flex flex-col surface
          md:translate-x-0 transition-transform duration-200
          ${open ? "translate-x-0" : "-translate-x-full"}`}
        style={{ background: "var(--bg-panel)", borderRight: "1px solid var(--border)" }}
      >
        {/* brand */}
        <div class="flex items-center gap-2.5 px-4 h-14 shrink-0" style={{ borderBottom: "1px solid var(--border-soft)" }}>
          <Logo size={21} />
          <span class="font-semibold tracking-tight text-[15px]" style={{ color: "var(--text-1)" }}>AgentDeck</span>
          <button class="ml-auto md:hidden p-1.5" style={{ color: "var(--text-2)" }} onClick={() => setOpen(false)} aria-label="fechar">{I.x}</button>
        </div>

        {/* actions */}
        <div class="p-3 space-y-3 shrink-0">
          <button
            onClick={onNew}
            class="w-full flex items-center justify-center gap-2 rounded-lg text-sm font-medium h-10 transition-all active:scale-[0.98] surface"
            style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}
          >
            {I.plus} New session
          </button>
          <div class="relative">
            <span style={{ color: "var(--text-3)" }} class="absolute left-3 top-1/2 -translate-y-1/2">{I.search}</span>
            <input
              value={filter}
              onInput={(e) => setFilter(e.target.value)}
              placeholder="Search sessions…"
              class="w-full h-9 rounded-lg pl-9 pr-3 text-sm focus:outline-none surface"
              style={{ color: "var(--text-1)", background: "var(--bg-card)", border: "1px solid var(--border-soft)" }}
              onfocus={(e) => (e.target.style.borderColor = "var(--border-strong)")}
              onblur={(e) => (e.target.style.borderColor = "var(--border-soft)")}
            />
          </div>
        </div>

        {/* list */}
        <nav class="flex-1 overflow-y-auto px-2 pb-3 relative">
          {filtered.length === 0 && (
            <p class="text-center text-xs text-zinc-500 mt-8 px-4">
              {sessions.length ? "no sessions found" : "your sessions will appear here"}
            </p>
          )}
          {groups.map((g) => (
            <section>
              <h3 class="px-3 pt-3 pb-1 text-[10.5px] font-medium uppercase tracking-wider" style={{ color: "var(--text-3)" }}>
                {g.name}
              </h3>
              {g.items.map((s) => {
                const ag = agentById[s.agent] || { color: "#888", label: s.agent };
                const active = s.id === activeId;
                const untitled = !s.title || s.title === "New session";
                return (
                  <div
                    key={s.id}
                    onClick={() => { onOpen(s.id); setOpen(false); }}
                    class={`group relative rounded-lg pl-3 pr-2 py-1.5 cursor-pointer surface min-h-[42px] flex flex-col justify-center ${
                      active ? "" : "hover:bg-white/[0.04]"
                    }`}
                    style={active ? { background: "var(--bg-raised)" } : {}}
                  >
                    {active && (
                      <span class="absolute left-0 top-1.5 bottom-1.5 w-[2px] rounded-full" style={{ background: "var(--accent)" }} />
                    )}
                    <div class="flex items-center gap-2 pr-2 group-hover:pr-14 h-[22px] transition-[padding]">
                      <AgentIcon id={s.agent} size={14} color={ag.color} />
                      {editing === s.id ? (
                        <input
                          autoFocus value={editTitle}
                          onInput={(e) => setEditTitle(e.target.value)}
                          onBlur={() => commitRename(s)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") commitRename(s);
                            if (e.key === "Escape") setEditing(null);
                          }}
                          onClick={(e) => e.stopPropagation()}
                          class="flex-1 min-w-0 bg-black/40 rounded px-1.5 py-0.5 text-sm text-zinc-100 focus:outline-none"
                          style={{ border: "1px solid var(--accent)" }}
                        />
                      ) : (
                        <span style={{ color: untitled ? "var(--text-3)" : active ? "var(--text-1)" : "var(--text-2)" }} class={`flex-1 min-w-0 truncate text-sm ${untitled ? "italic" : ""}`}>
                          {untitled ? "untitled" : s.title}
                        </span>
                      )}
                      <span style={{ color: "var(--text-2)" }} class="text-[11px] shrink-0">{relTime(s.updated_at)}</span>
                    </div>
                    <div class={`mt-0.5 pl-[15px] pr-3 text-[12px] text-zinc-500 truncate ${s.preview ? "" : "hidden"}`}>
                      {s.preview}
                    </div>

                    <div class="absolute right-2 top-1/2 -translate-y-1/2 hidden group-hover:flex gap-1" onClick={(e) => e.stopPropagation()}>
                      <button
                        title="rename"
                        onClick={() => { setEditing(s.id); setEditTitle(s.title); }}
                        class="h-7 w-7 grid place-items-center rounded-md surface"
                        style={{ background: "var(--bg-raised)", border: "1px solid var(--border)" }}
                      >{I.pencil}</button>
                      {confirmDel === s.id ? (
                        <button
                          onClick={() => { onDelete(s.id); setConfirmDel(null); }}
                          class="h-7 px-2 rounded-md text-[11px] font-medium"
                          style={{ background: "var(--err)", color: "#fff" }}
                        >delete?</button>
                      ) : (
                        <button
                          title="delete"
                          onClick={() => setConfirmDel(s.id)}
                          class="h-7 w-7 grid place-items-center rounded-md text-zinc-400 surface"
                          style={{ background: "var(--bg-raised)", border: "1px solid var(--border)" }}
                        >{I.trash}</button>
                      )}
                    </div>
                  </div>
                );
              })}
            </section>
          ))}
          {/* scroll fade */}
          {filtered.length > 4 && (
            <div class="sticky bottom-0 h-8 pointer-events-none -mx-2"
              style={{ background: "linear-gradient(to top, var(--bg-panel), transparent)" }} />
          )}
        </nav>

        {/* status footer */}
        <div
          class="flex items-center gap-2 px-4 h-11 text-[11px] shrink-0" style={{ color: "var(--text-3)" }}
          style={{ borderTop: "1px solid var(--border-soft)" }}
        >
          <span class="h-[6px] w-[6px] rounded-full" style={{ background: "var(--ok)" }} />
          local agents online
        </div>
      </aside>
    </>
  );
}

/* ------------------------------------------------------------------ chat */
export function ToolChip({ tool }) {
  const done = tool.state === "end";
  return (
    <span
      class="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-mono surface"
      style={{
        background: done ? "transparent" : "var(--warn-soft)",
        border: `1px solid ${done ? "var(--border)" : "var(--warn-border)"}`,
        color: done ? "var(--text-3)" : "var(--warn)",
      }}
    >
      <span class={done ? "text-emerald-500" : "animate-pulse text-amber-400"}>
        {done ? "✓" : "⟳"}
      </span>
      {tool.name}
    </span>
  );
}

export function Message({ m }) {
  const isUser = m.role === "user";
  const tools = m.meta?.tools || [];
  return (
    <div class={`flex ${isUser ? "justify-end" : "justify-start"} mb-5`}>
      <div class={`max-w-[90%] md:max-w-[75%] ${isUser ? "items-end" : "items-start"} flex flex-col gap-1.5`}>
        {isUser ? (
          <div class="flex flex-col items-end gap-1">
            <div class="rounded-2xl rounded-br-md px-4 py-2.5 text-[15px] leading-relaxed whitespace-pre-wrap"
              style={{ background: "var(--bubble-user-bg)", border: "1px solid var(--bubble-user-border)", color: "var(--text-1)" }}>
              {m.content}
            </div>
            {m.meta?.queued && (
              <span class="text-[11px] text-sky-300/80 pr-1">queued — will send when the turn ends</span>
            )}
          </div>
        ) : (
          <div class="rounded-2xl rounded-bl-md px-4 py-3 w-full"
            style={{
              background: m.meta?.error ? "rgba(127,29,29,0.12)" : "var(--bg-card)",
              border: `1px solid ${m.meta?.error ? "rgba(185,28,28,0.4)" : "var(--border-soft)"}`,
            }}>
            <Markdown text={m.content} />
          </div>
        )}
        {tools.length > 0 && (
          <div class="flex flex-wrap gap-1.5 px-1">
            {tools.slice(-10).map((t, i) => <ToolChip key={i} tool={t} />)}
          </div>
        )}
      </div>
    </div>
  );
}

export function Composer({ running, onSend, onStop, disabled, sessionId }) {
  const [text, setText] = useState("");
  const ref = useRef(null);

  // draft persistence (G2): per-session draft survives reloads and
  // session switches; cleared on send
  const draftKey = sessionId ? `agentdeck:draft:${sessionId}` : null;
  useEffect(() => {
    setText(draftKey ? localStorage.getItem(draftKey) || "" : "");
  }, [draftKey]);
  useEffect(() => {
    if (draftKey && text) localStorage.setItem(draftKey, text);
  }, [text, draftKey]);

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = "auto";
      ref.current.style.height = Math.min(ref.current.scrollHeight, 160) + "px";
    }
  }, [text]);

  const submit = () => {
    // no `running` guard: while a turn is in flight the message QUEUES
    // (steering, benchmark G3) — the server caps and 409s when full
    if (!text.trim() || disabled) return;
    onSend(text.trim());
    setText("");
    if (draftKey) localStorage.removeItem(draftKey);
  };

  return (
    <div
      class="shrink-0 surface"
      style={{
        background: "var(--bg-panel)",
        borderTop: "1px solid var(--border-soft)",
        paddingBottom: "calc(env(safe-area-inset-bottom, 0px) + 12px)",
        paddingTop: 12,
        paddingLeft: 12,
        paddingRight: 12,
      }}
    >
      <div class="max-w-3xl mx-auto flex items-end gap-2.5">
        <textarea
          ref={ref}
          rows="1"
          value={text}
          disabled={disabled}
          onInput={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); }
          }}
          placeholder={disabled ? "crie uma sessão para começar" : "mensagem para o agente…"}
          class="flex-1 resize-none rounded-xl px-4 py-3 text-[15px] focus:outline-none disabled:opacity-50 surface"
          style={{ color: "var(--text-1)", background: "var(--bg-card)", border: "1px solid var(--border)", maxHeight: 160 }}
          onfocus={(e) => (e.target.style.borderColor = "var(--border-strong)")}
          onblur={(e) => (e.target.style.borderColor = "var(--border)")}
        />
        {running ? (
          <button
            onClick={onStop}
            class="h-12 w-12 shrink-0 grid place-items-center rounded-xl transition-colors active:scale-95"
            style={{ background: "var(--err)", color: "#fff" }}
            title="parar agente"
          >{I.stop}</button>
        ) : (
          <button
            onClick={submit}
            disabled={disabled || !text.trim()}
            class="h-12 w-12 shrink-0 grid place-items-center rounded-xl transition-all active:scale-95 disabled:opacity-30 disabled:cursor-not-allowed disabled:active:scale-100"
            style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}
            title="enviar (Enter)"
          >{I.send}</button>
        )}
      </div>
      <p class="sr-only">Enter envia, Shift+Enter quebra linha</p>
    </div>
  );
}
