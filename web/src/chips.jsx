import { useEffect, useRef, useState } from "react";

function filterOpts(options, q) {
  const n = (q || "").trim().toLowerCase();
  if (!n) return options || [];
  return (options || []).filter((o) => (o.label + " " + (o.id || "") + " " + (o.hint || "")).toLowerCase().includes(n));
}

export function Chip({ id, label, options, value, onChange, icon, searchable, title }) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const wrap = useRef(null);
  const hits = filterOpts(options, q);
  const cur = (options || []).find((o) => o.id === value);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e) => {
      if (wrap.current && !wrap.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  useEffect(() => { if (!open) setQ(""); }, [open]);

  if (!options || !options.length) return null;

  return (
    <div class="relative" ref={wrap}>
      <button
        type="button"
        id={id}
        title={title || label}
        onClick={() => setOpen((o) => !o)}
        class="flex items-center gap-1.5 h-7 pl-2 pr-1.5 rounded-lg text-[12px] font-medium transition-colors hover:opacity-80 max-w-[220px]"
        style={{ background: "transparent", border: "1px solid var(--border)", color: "var(--text-2)" }}
      >
        {icon}
        <span class="truncate">{cur?.label || label}</span>
        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="m6 9 6 6 6-6"/></svg>
      </button>
      {open && (
        <div
          class="absolute bottom-9 left-0 z-50 min-w-[180px] max-w-[280px] rounded-xl p-1.5 shadow-xl"
          style={{ background: "var(--bg-card)", border: "1px solid var(--border-strong)" }}
        >
          {searchable && options.length > 8 && (
            <input
              autoFocus
              value={q}
              onInput={(e) => setQ(e.target.value)}
              placeholder="search"
              class="w-full mb-1 h-7 px-2 rounded-md text-[12px] focus:outline-none"
              style={{ background: "var(--bg-raised)", color: "var(--text-1)", border: "1px solid var(--border)" }}
            />
          )}
          <div class="max-h-56 overflow-y-auto">
            {hits.map((o) => (
              <button
                key={o.id || "default"}
                type="button"
                onClick={() => { onChange(o.id); setOpen(false); }}
                class="w-full flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg text-[13px] text-left hover:bg-[color:var(--bg-hover)]"
                style={{ color: o.id === value ? "var(--text-1)" : "var(--text-2)" }}
              >
                <span class="truncate">{o.label}</span>
                {o.hint && <span class="text-[10px] shrink-0" style={{ color: "var(--text-3)" }}>{o.hint}</span>}
              </button>
            ))}
            {hits.length === 0 && (
              <div class="px-2.5 py-2 text-[12px]" style={{ color: "var(--text-3)" }}>no matches</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export const Ico = {
  provider: <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M7 18a5 5 0 0 1 1-9 6 6 0 0 1 11 2 4 4 0 0 1 1 7H7z"/></svg>,
  model: <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M12 3v3m0 12v3M3 12h3m12 0h3"/></svg>,
  think: <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M4 14c0-4 4-8 8-8s8 4 8 8"/><path d="M7 17h10M9 20h6"/></svg>,
  mode: <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M4 7h16M4 12h10M4 17h7"/></svg>,
  kind: <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M5 6h14v9H8l-3 3V6z"/></svg>,
};
