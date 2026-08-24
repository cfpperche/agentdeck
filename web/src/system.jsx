import { useEffect, useState } from "react";

export function SystemPanel({ onClose }) {
  const [sys, setSys] = useState(null);
  useEffect(() => {
    fetch("/api/system").then((r) => r.json()).then(setSys).catch(() => setSys({}));
  }, []);

  return (
    <div class="flex-1 overflow-y-auto">
      <div class="max-w-xl mx-auto px-6 py-10">
        <div class="flex items-center gap-2 mb-6">
          <button
            onClick={onClose}
            class="h-8 w-8 grid place-items-center rounded-lg transition-colors hover:bg-[color:var(--bg-hover)]"
            style={{ color: "var(--text-2)" }}
            aria-label="back"
            title="back"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
          </button>
          <h1 class="text-lg font-semibold" style={{ color: "var(--text-1)" }}>System</h1>
        </div>

        <Section title="Host" rows={hostRows(sys)} />
        <Section title="Network" rows={netRows(sys)} />
        <Section title="Dependencies" rows={depRows(sys)} />
        <Section title="About" rows={aboutRows(sys)} />
      </div>
    </div>
  );
}

function Section({ title, rows }) {
  return (
    <section class="mb-8">
      <h2 class="text-[13px] font-semibold mb-2" style={{ color: "var(--text-2)" }}>{title}</h2>
      <div class="rounded-xl overflow-hidden" style={{ border: "1px solid var(--border)" }}>
        {rows.map(([k, v], i) => (
          <div
            key={k + i}
            class="flex items-start justify-between gap-4 px-4 py-2.5 text-[13px]"
            style={{ background: "var(--bg-card)", borderTop: i ? "1px solid var(--border-soft)" : "none" }}
          >
            <span class="shrink-0" style={{ color: "var(--text-2)" }}>{k}</span>
            <span class="text-right font-mono text-[12px] break-all" style={{ color: "var(--text-1)" }}>{v}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function hostRows(sys) {
  if (!sys?.host) return [["Status", "loading…"]];
  const h = sys.host;
  let os = h.os || "—";
  if (h.arch) os += " · " + h.arch;
  if (h.wsl) os += " (WSL)";
  return [
    ["Name", h.name || "—"],
    ["OS", os],
  ];
}

function netRows(sys) {
  if (!sys?.network) return [["Status", "loading…"]];
  const n = sys.network;
  const bind = n.port ? `${n.bind}:${n.port}` : (n.bind || "—");
  return [
    ["Bind", bind],
    ["HTTPS", n.https ? "on" : "off"],
    ["LAN", (n.lan && n.lan.length) ? n.lan.join(", ") : "—"],
    ["Tailscale", n.tailscale || "—"],
  ];
}

function depRows(sys) {
  if (!sys) return [["Status", "loading…"]];
  const rows = [
    ["tmux", sys.tmux?.installed ? (sys.tmux.version || "installed") : "not installed"],
    ["mkcert", sys.mkcert?.installed ? "installed" : "not installed · optional"],
    ["tailscale", sys.tailscale?.installed ? (sys.tailscale.ip || "installed") : "not installed · optional"],
  ];
  for (const a of sys.agents || []) {
    rows.push([a.id, a.installed ? (a.version || "installed") : "not installed"]);
  }
  for (const w of sys.warnings || []) rows.push(["note", w]);
  return rows;
}

function aboutRows(sys) {
  return [
    ["Version", sys?.version ? `v${sys.version}` : "—"],
    ["Repository", <a href="https://github.com/cfpperche/agentdeck" target="_blank" rel="noopener noreferrer" style={{ color: "var(--accent-fg)" }}>cfpperche/agentdeck ↗</a>],
    ["License", "MIT"],
  ];
}
