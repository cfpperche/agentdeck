import { statusSegments } from "./statusbar.js";

export function ComposerStatus({ bar }) {
  const parts = statusSegments(bar);
  if (!parts.length) return null;
  return (
    <div class="flex flex-wrap items-center gap-x-0 gap-y-0.5 px-3 pb-2 pt-1 text-[11px] leading-snug"
      style={{ color: "var(--text-3)", borderTop: "1px solid var(--border-soft)" }}
      aria-label="session status"
    >
      {parts.map((p, i) => (
        <span key={p.key} class={p.tone === "warn" ? "text-[color:var(--warn)]" : p.tone === "bad" ? "text-[color:var(--err)]" : ""}>
          {i > 0 ? <span class="mx-1.5 opacity-40" aria-hidden="true">/</span> : null}
          {p.kind === "bar" ? <CtxBar p={p} /> : p.text}
        </span>
      ))}
    </div>
  );
}

function CtxBar({ p }) {
  const fill = p.tone === "bad" ? "var(--err)" : p.tone === "warn" ? "var(--warn)" : "var(--ok)";
  return (
    <span class="inline-flex items-center gap-1.5" title={p.text}>
      <span class="inline-block w-20 h-[5px] rounded-full overflow-hidden" style={{ background: "var(--border)" }} aria-hidden="true">
        <span class="block h-full rounded-full" style={{ width: (p.pct || 0) + "%", background: fill }} />
      </span>
      <span>{p.text}</span>
    </span>
  );
}
