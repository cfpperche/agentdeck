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
  onOpen, onNew, onRename, onDelete, open, setOpen, theme, onToggleTheme, onOpenSettings, onOpenDevices,
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
          <button type="button" id="btn-share" class="ml-auto p-1.5 rounded-md" style={{ color: "var(--text-2)" }} title="Open on phone" onClick={() => window.dispatchEvent(new CustomEvent("agentdeck-share"))} aria-label="Open on phone">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><path d="M14 14h3v3h-3zM20 14h1v1M14 20h1v1M17 17h4v4h-4z"/></svg>
          </button>
          <button class="md:hidden p-1.5" style={{ color: "var(--text-2)" }} onClick={() => setOpen(false)} aria-label="fechar">{I.x}</button>
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
                      <span style={{ color: "var(--text-2)" }} class="ts text-[11px] shrink-0">{relTime(s.updated_at)}</span>
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

        {/* user menu (Vercel-style; benchmark t3code SidebarChrome) */}
        <UserMenu theme={theme} onToggleTheme={onToggleTheme} onOpenSettings={onOpenSettings} onOpenDevices={onOpenDevices} />
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
      {tool.detail && <span style={{ color: "var(--text-3)" }}>· {String(tool.detail).slice(0, 60)}</span>}
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

export function Composer({ running, onSend, onStop, disabled, sessionId, agentId, caps }) {
  const [text, setText] = useState("");
  const ref = useRef(null);
  const [openMenu, setOpenMenu] = useState(null); // 'model' | 'think' | null

  // composer controls (ADR-0006): model/thinking/mode selected per agent,
  // persisted so the next session starts where you left off.
  const ctrlKey = agentId ? `agentdeck:controls:${agentId}` : null;
  const [ctrl, setCtrl] = useState({ model: null, thinking: null, mode: null });
  useEffect(() => {
    if (!ctrlKey || !caps) return;
    let saved = {};
    try { saved = JSON.parse(localStorage.getItem(ctrlKey) || "{}"); } catch {}
    const defModel = caps.models?.find((m) => m.is_default) || caps.models?.[0];
    const model = caps.models?.some((m) => m.id === saved.model)
      ? saved.model
      : defModel?.id || null;
    const m = caps.models?.find((x) => x.id === model);
    const think = m?.thinking_options?.some((t) => t.id === saved.thinking)
      ? saved.thinking
      : m?.default_thinking_option_id || m?.thinking_options?.find((t) => t.is_default)?.id || m?.thinking_options?.[0]?.id || null;
    const mode = caps.modes?.some((x) => x.id === saved.mode)
      ? saved.mode
      : caps.modes?.find((x) => x.id === "manual")?.id || caps.modes?.[0]?.id || null;
    setCtrl({ model, thinking: think, mode });
  }, [ctrlKey, caps]);
  const updCtrl = (patch) => {
    setCtrl((c) => {
      // switching model resets an invalid thinking choice
      const next = { ...c, ...patch };
      const m = caps?.models?.find((x) => x.id === next.model);
      if (next.thinking && !m?.thinking_options?.some((t) => t.id === next.thinking))
        next.thinking = m?.default_thinking_option_id || null;
      if (ctrlKey) localStorage.setItem(ctrlKey, JSON.stringify(next));
      return next;
    });
  };

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
    const controls = {
      ...(ctrl.model ? { model: ctrl.model } : {}),
      ...(ctrl.thinking ? { thinking: ctrl.thinking } : {}),
      ...(ctrl.mode ? { mode: ctrl.mode } : {}),
    };
    onSend(text.trim(), Object.keys(controls).length ? controls : undefined);
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
      <div class="max-w-3xl mx-auto">
        {/* one bordered block owns everything (Cursor shape): textarea on
            top, control strip inside the same box */}
        <div
          class="rounded-xl flex flex-col"
          style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }}
          onFocus={(e) => (e.currentTarget.style.borderColor = "var(--border-strong)")}
          onBlur={(e) => (e.currentTarget.style.borderColor = "var(--border)")}
        >
          <textarea
            ref={ref}
            rows="1"
            value={text}
            disabled={disabled}
            onInput={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); }
            }}
            placeholder={disabled ? "open a session to start" : "message the agent\u2026"}
            class="flex-1 resize-none px-4 pt-3 pb-1 text-[15px] focus:outline-none disabled:opacity-50 bg-transparent"
            style={{ color: "var(--text-1)", maxHeight: 160, border: "none" }}
          />
          {/* control strip INSIDE the box: chips left, send right */}
          <div class="flex items-center justify-between gap-2 pl-2 pr-2 pb-2 pt-0.5">
            <div class="flex items-center gap-1.5 min-w-0">
              {caps?.models?.filter((m) => m.id).length > 0 && ctrl.model && (() => {
                const m = caps.models.find((x) => x.id === ctrl.model);
                return (
                  <div class="relative">
                    <button
                      onClick={() => setOpenMenu((o) => o === "model" ? null : "model")}
                      class="flex items-center gap-1.5 h-7 pl-2 pr-1.5 rounded-lg text-[12px] font-medium transition-colors hover:opacity-80"
                      style={{ background: "transparent", border: "1px solid var(--border)", color: "var(--text-2)" }}
                      title="model"
                    >
                      {m?.label || ctrl.model}
                      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="m6 9 6 6 6-6"/></svg>
                    </button>
                    {openMenu === "model" && (
                      <>
                        <div class="fixed inset-0 z-40" onClick={() => setOpenMenu(null)} />
                        <div
                          class="absolute bottom-9 left-0 z-50 w-64 rounded-xl p-1.5 shadow-xl"
                          style={{ background: "var(--bg-card)", border: "1px solid var(--border-strong)" }}
                        >
                          {caps.models.filter((mo) => mo.id).map((mo) => (
                            <button
                              key={mo.id}
                              onClick={() => { updCtrl({ model: mo.id }); setOpenMenu(null); }}
                              class="w-full flex items-center justify-between px-2.5 py-2 rounded-lg text-[13px] transition-colors text-left hover:bg-[color:var(--bg-hover)]"
                              style={{ color: mo.id === ctrl.model ? "var(--text-1)" : "var(--text-2)" }}
                            >
                              <span>{mo.label}</span>
                              {mo.is_default && <span class="text-[10px] uppercase tracking-wide" style={{ color: "var(--text-3)" }}>default</span>}
                            </button>
                          ))}
                        </div>
                      </>
                    )}
                  </div>
                );
              })()}
              {(() => {
                // thinking is a first-class strip control (not buried in the
                // model menu) — label in front of a compact selector
                const m = caps?.models?.find((x) => x.id === ctrl.model)
                  || caps?.models?.find((x) => !x.id && x.thinking_options?.length)
                  || null;
                const opts = m?.thinking_options || [];
                if (!opts.length) return null;
                const cur = opts.find((t) => t.id === ctrl.thinking) || opts.find((t) => t.is_default) || opts[0];
                return (
                  <div class="relative flex items-center gap-1.5">
                    <span class="text-[11px] font-medium shrink-0" style={{ color: "var(--text-3)" }}>Thinking</span>
                    <button
                      onClick={() => setOpenMenu((o) => o === "think" ? null : "think")}
                      class="flex items-center gap-1.5 h-7 pl-2 pr-1.5 rounded-lg text-[12px] font-medium transition-colors hover:opacity-80"
                      style={{ background: "transparent", border: "1px solid var(--border)", color: "var(--text-2)" }}
                      title="thinking"
                    >
                      {cur?.label || "off"}
                      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="m6 9 6 6 6-6"/></svg>
                    </button>
                    {openMenu === "think" && (
                      <>
                        <div class="fixed inset-0 z-40" onClick={() => setOpenMenu(null)} />
                        <div
                          class="absolute bottom-9 left-0 z-50 min-w-[140px] rounded-xl p-1.5 shadow-xl"
                          style={{ background: "var(--bg-card)", border: "1px solid var(--border-strong)" }}
                        >
                          {opts.map((t) => (
                            <button
                              key={t.id}
                              onClick={() => { updCtrl({ thinking: t.id }); setOpenMenu(null); }}
                              class="w-full flex items-center px-2.5 py-1.5 rounded-lg text-[13px] transition-colors text-left hover:bg-[color:var(--bg-hover)]"
                              style={{ color: t.id === ctrl.thinking ? "var(--text-1)" : "var(--text-2)" }}
                            >{t.label}</button>
                          ))}
                        </div>
                      </>
                    )}
                  </div>
                );
              })()}
              {caps?.modes?.length > 1 && ctrl.mode && (() => {
                // paseo pattern: click cycles; colored dot carries the tier
                const idx = caps.modes.findIndex((x) => x.id === ctrl.mode);
                const cur = caps.modes[idx];
                return (
                  <button
                    onClick={() => updCtrl({ mode: caps.modes[(idx + 1) % caps.modes.length].id })}
                    class="flex items-center gap-1.5 h-7 px-2 rounded-lg text-[12px] font-medium transition-colors hover:opacity-80 truncate max-w-[220px]"
                    style={{ background: "transparent", border: "1px solid var(--border)", color: "var(--text-2)" }}
                    title={`mode: ${cur.description || cur.label} (click to change)`}
                  >
                    <span class="h-1.5 w-1.5 rounded-full shrink-0" style={{ background: idx === 0 ? "var(--ok)" : "var(--accent)" }} />
                    <span class="truncate">{cur.label}</span>
                  </button>
                );
              })()}
            </div>
            {running ? (
              <button
                onClick={onStop}
                class="h-8 w-8 shrink-0 grid place-items-center rounded-lg transition-colors active:scale-95"
                style={{ background: "var(--err)", color: "#fff" }}
                title="stop agent"
              >{I.stop}</button>
            ) : (
              <button
                onClick={submit}
                disabled={disabled || !text.trim()}
                class="h-8 w-8 shrink-0 grid place-items-center rounded-lg transition-all active:scale-95 disabled:opacity-30 disabled:cursor-not-allowed disabled:active:scale-100"
                style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}
                title="send (Enter)"
              >{I.send}</button>
            )}
          </div>
        </div>
      </div>
      <p class="sr-only">Enter envia, Shift+Enter quebra linha</p>
    </div>
  );
}

/* ---------------------------------------------------------------- settings */
const ThemeIcon = ({ kind }) =>
  kind === "system" ? (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="2" y="4" width="20" height="14" rx="2"/><path d="M12 9v4m-2-2h4M2 20h20"/></svg>
  ) : kind === "dark" ? (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"/></svg>
  ) : (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>
  );

export function SettingsPanel({ themePref, onSetTheme, currentTheme, onClose }) {
  const options = [
    { id: "system", label: "System", desc: "Follows your OS" },
    { id: "dark", label: "Dark", desc: "Always dark" },
    { id: "light", label: "Light", desc: "Always light" },
  ];
  return (
    <div class="max-w-xl mx-auto w-full px-4 md:px-6 py-8">
      <div class="flex items-center gap-2 mb-4">
        <button
          onClick={onClose}
          class="h-8 w-8 grid place-items-center rounded-lg transition-colors hover:bg-[color:var(--bg-hover)]"
          style={{ color: "var(--text-2)" }}
          aria-label="back"
          title="back"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
        </button>
        <h2 class="text-lg font-semibold" style={{ color: "var(--text-1)" }}>Settings</h2>
      </div>
      <p class="text-sm mb-8" style={{ color: "var(--text-3)" }}>Preferences are stored in this browser.</p>

      <section>
        <h3 class="text-[11px] font-medium uppercase tracking-wider mb-3" style={{ color: "var(--text-3)" }}>Appearance</h3>
        <div class="grid grid-cols-3 gap-2.5">
          {options.map((o) => {
            const active = themePref === o.id;
            return (
              <button
                key={o.id}
                onClick={() => onSetTheme(o.id)}
                class="flex flex-col items-start gap-2 rounded-xl p-3.5 surface active:scale-[0.98] transition-all"
                style={{
                  background: "var(--bg-card)",
                  border: `1px solid ${active ? "var(--accent)" : "var(--border)"}`,
                }}
              >
                <span style={{ color: active ? "var(--accent-fg)" : "var(--text-2)" }}><ThemeIcon kind={o.id} /></span>
                <span class="text-sm font-medium" style={{ color: "var(--text-1)" }}>{o.label}</span>
                <span class="text-[11px]" style={{ color: "var(--text-3)" }}>
                  {o.desc}{o.id === "system" && currentTheme ? ` · ${currentTheme}` : ""}
                </span>
              </button>
            );
          })}
        </div>
      </section>

      <PortSection />
      <AboutSection />
    </div>
  );
}

function AboutSection() {
  const [info, setInfo] = useState(null);
  useEffect(() => {
    fetch("/api/server-info")
      .then((r) => r.json())
      .then(setInfo)
      .catch(() => {});
  }, []);
  const rows = info
    ? [
        ["Version", info.version || "—"],
        ["Execution mode", info.mode || "—"],
      ]
    : [];
  return (
    <section class="mt-10">
      <h3 class="text-[11px] font-medium uppercase tracking-wider mb-3" style={{ color: "var(--text-3)" }}>About</h3>
      <div class="rounded-xl overflow-hidden" style={{ border: "1px solid var(--border)" }}>
        {rows.length === 0 && (
          <div class="px-4 py-3 text-sm" style={{ color: "var(--text-3)" }}>loading…</div>
        )}
        {rows.map(([k, v], i) => (
          <div
            key={k}
            class="flex items-center justify-between px-4 py-3"
            style={{ background: "var(--bg-card)", borderTop: i ? "1px solid var(--border-soft)" : "none" }}
          >
            <span class="text-sm" style={{ color: "var(--text-2)" }}>{k}</span>
            <span class="text-sm font-mono" style={{ color: "var(--text-1)" }}>{v}</span>
          </div>
        ))}
      </div>
      <p class="text-[11px] mt-3" style={{ color: "var(--text-3)" }}>
        AgentDeck — the web cockpit for local AI agents. github.com/cfpperche/agentdeck
      </p>
    </section>
  );
}


/* ---------------------------------------------------------------- user menu */
export function UserMenu({ theme, onToggleTheme, onOpenSettings, onOpenDevices }) {
  const [open, setOpen] = useState(false);
  const [info, setInfo] = useState(null);
  const ref = useRef(null);

  useEffect(() => {
    fetch("/api/server-info").then((r) => r.json()).then(setInfo).catch(() => {});
  }, []);

  useEffect(() => {
    if (!open) return;
    const close = (e) => { if (!ref.current?.contains(e.target)) setOpen(false); };
    const esc = (e) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", esc);
    };
  }, [open]);

  const user = info?.user || "you";
  const mode = info?.mode || "personal";
  const item = "w-full flex items-center gap-2.5 px-3 py-2.5 text-left text-[13px] surface hover:bg-[color:var(--bg-hover)]";

  return (
    <div ref={ref} class="relative shrink-0" style={{ borderTop: "1px solid var(--border-soft)" }}>
      <button
        onClick={() => setOpen((o) => !o)}
        class="w-full flex items-center gap-2.5 px-3 h-12 mb-1 surface hover:bg-[color:var(--bg-hover)]"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <span class="h-7 w-7 rounded-full grid place-items-center text-[11px] font-semibold shrink-0"
          style={{ background: "var(--bg-raised)", color: "var(--text-2)", border: "1px solid var(--border)" }}>
          {user.slice(0, 2).toUpperCase()}
        </span>
        <span class="flex-1 min-w-0 text-left">
          <span class="block text-[12.5px] truncate" style={{ color: "var(--text-1)" }}>{user}</span>
          <span class="block text-[10.5px]" style={{ color: "var(--text-3)" }}>localhost · {mode}</span>
        </span>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--text-3)" stroke-width="2.2" stroke-linecap="round" style={{ transform: open ? "rotate(180deg)" : "", transition: "transform 150ms" }}><path d="m6 9 6 6 6-6"/></svg>
      </button>

      {open && (
        <div role="menu"
          class="absolute bottom-[calc(100%+8px)] left-2 right-2 rounded-xl overflow-hidden z-50"
          style={{ background: "var(--bg-card)", border: "1px solid var(--border)", boxShadow: "var(--shadow)" }}>
          <button role="menuitem" class={item} style={{ color: "var(--text-2)" }} onClick={() => { setOpen(false); onOpenSettings(); }}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1 1.55V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1-1.55 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.55-1H3a2 2 0 1 1 0-4h.09a1.7 1.7 0 0 0 1.55-1 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34h.09a1.7 1.7 0 0 0 1-1.55V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1 1.55h.09a1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87v.09a1.7 1.7 0 0 0 1.55 1H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.55 1Z"/></svg>
            Settings
          </button>
          <button role="menuitem" class={item} style={{ color: "var(--text-2)" }} onClick={() => { setOpen(false); onOpenDevices && onOpenDevices(); }}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="2" width="14" height="20" rx="2"/><path d="M12 18h.01"/></svg>
            Devices
          </button>
          <button role="menuitem" class={item} style={{ color: "var(--text-2)" }} onClick={() => onToggleTheme()}>
            {theme === "dark" ? (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>
            ) : (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"/></svg>
            )}
            {theme === "dark" ? "Light mode" : "Dark mode"}
          </button>
          <div style={{ borderTop: "1px solid var(--border)" }} class="bg-[color:var(--bg-panel)]">
            <div class="flex items-center gap-2 px-3 py-2">
              <span class="h-1.5 w-1.5 rounded-full" style={{ background: "var(--ok)" }} />
              <span class="text-[11px]" style={{ color: "var(--text-3)" }}>
                AgentDeck {info?.version || ""} · agents online
              </span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}


/* ------------------------------------------------------- port (Server) */
function PortSection() {
  const [serving, setServing] = useState(null);
  const [configured, setConfigured] = useState("");
  const [port, setPort] = useState("");
  const [busy, setBusy] = useState(false);
  const [moving, setMoving] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetch("/api/server/port").then((r) => r.json()).then((j) => {
      setServing(j.serving);
      setPort(String(j.serving));
      setConfigured(j.configured || String(j.serving));
    }).catch(() => {});
  }, []);

  const apply = () => {
    const target = port.trim();
    if (!target || busy || moving) return;
    setError(null);
    setBusy(true);
    fetch("/api/server/port", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port: target }),
    }).then(async (r) => {
      const j = await r.json().catch(() => ({}));
      if (r.status === 202) {
        setMoving(j.port);
        setTimeout(() => {
          location.replace(`${location.protocol}//${location.hostname}:${j.port}/settings`);
        }, 1500);
      } else {
        setError(j.detail || "cannot change port");
        setBusy(false);
      }
    }).catch((e) => { setError(String(e)); setBusy(false); });
  };

  const dirty = port.trim() !== String(serving);

  return (
    <section class="mt-10">
      <h3 class="text-[11px] font-medium uppercase tracking-wider mb-3" style={{ color: "var(--text-3)" }}>Server</h3>
      <p class="text-sm mb-4" style={{ color: "var(--text-2)" }}>
        Port AgentDeck serves on. The server moves immediately and this page reconnects.
      </p>
      <div class="flex gap-2 items-start">
        <label class="flex flex-col gap-1.5">
          <span class="text-[11px] font-medium" style={{ color: "var(--text-3)" }}>Port</span>
          <input
          value={port}
          onInput={(e) => setPort(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && apply()}
          disabled={!!moving}
          spellCheck="false"
          class="w-28 h-10 rounded-lg px-3 text-[13px] font-mono focus:outline-none surface"
          style={{ background: "var(--bg-card)", border: "1px solid var(--border-soft)", color: "var(--text-1)" }}
          />
        </label>
        <button
          onClick={apply}
          disabled={!dirty || busy || !!moving}
          class="h-10 px-5 rounded-lg text-[13px] font-medium transition-all disabled:opacity-30 disabled:cursor-not-allowed"
          style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}
        >
          {moving ? "Moving…" : "Apply"}
        </button>
      </div>
      {moving && (
        <p class="text-[12px] mt-2.5" style={{ color: "var(--accent-fg)" }}>
          Moving to port {moving} — reconnecting…
        </p>
      )}
      {!moving && !error && (
        <p class="flex items-center gap-1.5 text-[12px] mt-2.5" style={{ color: "var(--text-3)" }}>
          <span class="h-1.5 w-1.5 rounded-full" style={{ background: "var(--ok)" }} />
          Serving on port {serving ?? "…"}{configured && configured !== String(serving) ? ` (configured: ${configured})` : ""}.
        </p>
      )}
      {error && (
        <p class="text-[12px] mt-2.5" style={{ color: "var(--err)" }}>{error}</p>
      )}
    </section>
  );
}
