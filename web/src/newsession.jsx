import { useEffect, useState } from "react";
import { AgentIcon } from "./icons.jsx";

// NewSessionPanel — session configuration shown in the "New session"
// tab. Today: runtime (agent) selection. This is the extension point
// for future options (model, workdir, permission mode, ...).
export function NewSessionPanel({ agents, onCreate }) {
  const [selected, setSelected] = useState(null);
  const [creating, setCreating] = useState(false);
  const [cwd, setCwd] = useState("");
  const [browsing, setBrowsing] = useState(false);
  const [browsePath, setBrowsePath] = useState("");
  const [browseDirs, setBrowseDirs] = useState(null);
  const [browseErr, setBrowseErr] = useState(null);

  const create = () => {
    if (!selected || creating) return;
    setCreating(true);
    onCreate(selected, cwd.trim() || null);
  };

  const openBrowser = async () => {
    setBrowsing(true);
    setBrowseErr(null);
    await loadDirs(browsePath || "");
  };

  const loadDirs = async (p) => {
    setBrowseDirs(null);
    setBrowseErr(null);
    try {
      const r = await fetch(`/api/fs/dirs${p ? `?path=${encodeURIComponent(p)}` : ""}`);
      const j = await r.json();
      if (!r.ok) throw new Error(j.detail || "cannot list");
      setBrowsePath(j.path);
      setBrowseDirs(j.dirs);
    } catch (e) {
      setBrowseErr(String(e.message || e));
      setBrowseDirs([]);
    }
  };

  return (
    <div class="flex-1 overflow-y-auto flex flex-col">
      <div class="max-w-lg mx-auto w-full px-6 py-10 flex-1 flex flex-col justify-center">
        <h2 class="text-lg font-semibold mb-1" style={{ color: "var(--text-1)" }}>New session</h2>
        <p class="text-sm mb-8" style={{ color: "var(--text-3)" }}>
          Pick the agent runtime for this session.
        </p>

        <div class="rounded-xl overflow-hidden" style={{ border: "1px solid var(--border)" }}>
          {agents.map((a, i) => {
            const active = selected === a.id;
            return (
              <button
                key={a.id}
                onClick={() => setSelected(a.id)}
                class="w-full flex items-center gap-3 px-4 py-3 surface text-left hover:bg-[color:var(--bg-hover)]"
                style={{
                  background: active ? "var(--bg-active)" : "var(--bg-card)",
                  borderTop: i ? "1px solid var(--border-soft)" : "none",
                  boxShadow: active ? "inset 2px 0 0 0 var(--accent)" : "none",
                }}
              >
                <AgentIcon id={a.id} size={18} color={a.color} />
                <span class="text-sm font-medium flex-1" style={{ color: active ? "var(--text-1)" : "var(--text-2)" }}>
                  {a.label}
                </span>
                {active && (
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--accent-fg)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
                )}
              </button>
            );
          })}
        </div>

        <h3 class="text-[11px] font-medium uppercase tracking-wider mt-8 mb-3" style={{ color: "var(--text-3)" }}>working directory</h3>
        <div class="flex gap-2">
          <input
            value={cwd}
            onInput={(e) => setCwd(e.target.value)}
            placeholder="/home/you/project — empty = isolated scratch workspace"
            spellCheck="false"
            class="flex-1 h-10 rounded-lg px-3 text-[13px] font-mono focus:outline-none surface"
            style={{ background: "var(--bg-card)", border: "1px solid var(--border-soft)", color: "var(--text-1)" }}
          />
          <button onClick={openBrowser}
            class="h-10 px-3.5 rounded-lg text-[12.5px] surface"
            style={{ border: "1px solid var(--border)", color: "var(--text-2)" }}>
            browse…
          </button>
        </div>
        {cwd.trim() ? (
          <p class="text-[11px] mt-2" style={{ color: "var(--text-3)" }}>
            session runs in <code style={{ color: "var(--accent-fg)" }}>{cwd.trim()}</code>
          </p>
        ) : (
          <p class="text-[11px] mt-2" style={{ color: "var(--text-3)" }}>
            no directory selected — session runs in an isolated scratch workspace
          </p>
        )}

        {browsing && (
          <div class="mt-3 rounded-xl overflow-hidden" style={{ border: "1px solid var(--border)" }}>
            <div class="flex items-center gap-2 px-3 h-10" style={{ background: "var(--bg-raised)" }}>
              <button onClick={() => { const up = browsePath.replace(/\/[^\/]+\/?$/, ""); loadDirs(up || "/"); }}
                class="text-[12px]" style={{ color: "var(--text-2)" }} title="parent">↑</button>
              <code class="flex-1 truncate text-[12px]" style={{ color: "var(--text-2)" }}>{browsePath}</code>
              <button onClick={() => { setCwd(browsePath); setBrowsing(false); }}
                class="text-[11px] px-2 h-6 rounded-md"
                style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}>use this</button>
              <button onClick={() => setBrowsing(false)} class="text-[11px] px-1" style={{ color: "var(--text-3)" }}>✕</button>
            </div>
            <div class="max-h-56 overflow-y-auto" style={{ background: "var(--bg-card)" }}>
              {browseDirs === null && <p class="px-3 py-3 text-[12px]" style={{ color: "var(--text-3)" }}>loading…</p>}
              {browseErr && <p class="px-3 py-3 text-[12px]" style={{ color: "var(--err)" }}>{browseErr}</p>}
              {browseDirs?.length === 0 && !browseErr && <p class="px-3 py-3 text-[12px]" style={{ color: "var(--text-3)" }}>no subdirectories</p>}
              {browseDirs?.map((d) => (
                <button key={d.path} onClick={() => loadDirs(d.path)}
                  class="w-full flex items-center gap-2 px-3 py-2 text-left surface hover:bg-[color:var(--bg-hover)]"
                  style={{ borderTop: "1px solid var(--border-soft)" }}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--text-3)" stroke-width="2"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/></svg>
                  <span class="text-[12.5px]" style={{ color: "var(--text-2)" }}>{d.name}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        <button
          onClick={create}
          disabled={!selected || creating}
          class="mt-6 h-11 rounded-xl text-sm font-medium transition-all active:scale-[0.99] disabled:opacity-30 disabled:cursor-not-allowed"
          style={{ background: "var(--btn-primary-bg)", color: "var(--btn-primary-fg)" }}
        >
          {creating ? "creating…" : "Start session"}
        </button>
      </div>
    </div>
  );
}
