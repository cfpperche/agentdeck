import { useState } from "react";
import { AgentIcon } from "./icons.jsx";

// NewSessionPanel — session configuration shown in the "New session"
// tab. Today: runtime (agent) selection. This is the extension point
// for future options (model, workdir, permission mode, ...).
export function NewSessionPanel({ agents, onCreate }) {
  const [selected, setSelected] = useState(null);
  const [creating, setCreating] = useState(false);

  const create = () => {
    if (!selected || creating) return;
    setCreating(true);
    onCreate(selected);
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
