import { useEffect, useRef, useState } from "react";
import QRCode from "qrcode";

export function ShareDrawer({ open, onClose }) {
  const [report, setReport] = useState(null);
  const [picked, setPicked] = useState("");
  const [err, setErr] = useState("");
  const canvasRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    setErr("");
    setPicked("");
    fetch("/api/share").then((r) => r.json()).then(setReport).catch((e) => setErr(e.message));
  }, [open]);

  const targets = report?.targets || [];
  const url = picked || report?.url || "";
  const chosen = targets.find((t) => t.url === url) || (url ? { url, onCert: true, addr: "" } : null);
  const appURL = withHash(url);
  const qrURL = trustPage(report?.trustPort, chosen?.addr, appURL);

  useEffect(() => {
    if (!open || !qrURL || !canvasRef.current) return;
    QRCode.toCanvas(canvasRef.current, qrURL, { width: 200, margin: 1, color: { dark: "#16181d", light: "#ffffff" } });
  }, [open, qrURL]);

  if (!open) return null;
  const misses = report ? report.checks.filter((c) => !c.ok) : [];

  return (
    <div class="fixed inset-0 z-[55] grid place-items-end md:place-items-center" style={{ background: "color-mix(in srgb, #000 40%, transparent)" }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div class="w-full max-w-[420px] max-h-[88vh] overflow-y-auto rounded-t-2xl md:rounded-xl p-4 pb-6"
        style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }} role="dialog" aria-label="Open on phone">
        <header class="flex items-center justify-between mb-3">
          <h2 class="text-[15px] font-semibold" style={{ color: "var(--text-1)" }}>Open on phone</h2>
          <button type="button" class="h-7 w-7 grid place-items-center" style={{ color: "var(--text-2)" }} onClick={onClose} aria-label="Close">×</button>
        </header>
        {err && <p class="text-sm text-red-400">{err}</p>}
        {!report && !err && <p class="text-sm" style={{ color: "var(--text-3)" }}>Checking…</p>}
        {report && (
          <>
            {qrURL && (
              <div class="grid place-items-center gap-2 py-2">
                <canvas ref={canvasRef} />
                <p class="text-[11px] font-mono break-all text-center" style={{ color: "var(--text-3)" }}>{qrURL}</p>
                {report.trustPort && (
                  <p class="text-[12px] text-center" style={{ color: "var(--text-3)" }}>
                    iPhone: Camera app (Safari). Chrome cannot install the profile. First Tailscale open can take 10–20s.
                  </p>
                )}
              </div>
            )}
            {misses.length > 0 && (
              <ul class="mb-3">
                {misses.map((c) => (
                  <li key={c.id} class="py-2" style={{ borderBottom: "1px solid var(--border-soft)" }}>
                    <strong class="block text-[13px]">{c.title}</strong>
                    <span class="text-[12px]" style={{ color: "var(--text-3)" }}>{c.action}</span>
                  </li>
                ))}
              </ul>
            )}
            {targets.length > 0 && (
              <ul class="space-y-1.5 mt-2">
                {targets.map((t) => (
                  <li key={t.url}>
                    <button type="button" disabled={!t.onCert}
                      class="w-full text-left rounded-lg px-2.5 py-2 flex flex-wrap gap-x-2.5 gap-y-1"
                      style={{
                        background: "var(--bg-canvas)",
                        border: `1px solid ${t.url === url ? "var(--accent)" : "var(--border)"}`,
                        opacity: t.onCert ? 1 : 0.55,
                      }}
                      onClick={() => setPicked(t.url)}
                    >
                      <span class="text-[10.5px] font-mono uppercase" style={{ color: "var(--text-3)" }}>{t.kind}</span>
                      <span class="text-[12px] font-mono">{t.addr}</span>
                      {!t.onCert && <span class="basis-full text-[11.5px] text-red-400">{t.reason}</span>}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <ul class="mt-3 space-y-1">
              {(report.checks || []).map((c) => (
                <li key={c.id} class="flex items-center gap-2 text-[12px]" style={{ color: c.ok ? "var(--ok)" : "var(--danger, #f7768e)" }}>
                  <span class="h-1.5 w-1.5 rounded-full" style={{ background: "currentColor" }} />
                  {c.title}
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </div>
  );
}

function withHash(url) {
  if (!url) return "";
  if (!location.hash || location.hash === "#") return url;
  return url.replace(/\/$/, "") + "/" + location.hash;
}

function trustPage(port, addr, app) {
  if (!addr || !port) return app || "";
  const u = new URL("http://" + addr + ":" + port + "/");
  if (app) u.searchParams.set("next", app);
  return u.toString();
}
