import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

function wsURL(path) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}${path}`;
}

// TerminalDock (ADR-0006 + 0008): closed by default, real header,
// × hides (detach). The tmux session keeps running.
export function TerminalDock({ open, sessionName, title, onClose, onShowChat }) {
  const hostRef = useRef(null);
  const termRef = useRef(null);

  useEffect(() => {
    if (!open || !sessionName || !hostRef.current) return;
    const host = hostRef.current;
    host.innerHTML = "";
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 12,
      fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace',
      theme: {
        background: "#0e0e11",
        foreground: "#ececf1",
        cursor: "#a1a1aa",
        selectionBackground: "#3f3f46",
      },
      scrollback: 10000,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    termRef.current = { term, fit, sock: null };

    const sock = new WebSocket(wsURL(`/ws/term?session=${encodeURIComponent(sessionName)}`));
    sock.binaryType = "arraybuffer";
    termRef.current.sock = sock;

    const sendResize = () => {
      if (sock.readyState === WebSocket.OPEN) {
        sock.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };
    sock.onopen = () => {
      fit.fit();
      sendResize();
      term.focus();
    };
    let sawBytes = false;
    sock.onmessage = (ev) => {
      if (typeof ev.data === "string") {
        try {
          const msg = JSON.parse(ev.data);
          if (msg.type === "error") term.writeln(`\r\n\x1b[31m${msg.message}\x1b[0m`);
        } catch { /* ignore */ }
        return;
      }
      sawBytes = true;
      term.write(new Uint8Array(ev.data));
    };
    sock.onclose = () => {
      if (!sawBytes) term.writeln("\r\n\x1b[31mcould not attach to the agent TUI\x1b[0m");
      else term.writeln("\r\n\x1b[90m— detached —\x1b[0m");
    };
    term.onData((data) => {
      if (sock.readyState === WebSocket.OPEN) sock.send(new TextEncoder().encode(data));
    });
    term.onResize(sendResize);
    const onWin = () => { fit.fit(); sendResize(); };
    window.addEventListener("resize", onWin);

    return () => {
      window.removeEventListener("resize", onWin);
      try { sock.close(); } catch {}
      term.dispose();
      termRef.current = null;
    };
  }, [open, sessionName]);

  useEffect(() => {
    if (open && termRef.current?.fit) {
      requestAnimationFrame(() => termRef.current.fit.fit());
    }
  }, [open]);

  if (!open) return null;
  return (
    <div class="flex-1 min-h-0 flex flex-col" style={{ background: "#0e0e11" }}>
      <div class="flex items-center h-8 px-3 shrink-0 gap-2"
        style={{ background: "var(--bg-panel)", borderBottom: "1px solid var(--border-soft)" }}>
        <span class="text-[12px] font-medium truncate" style={{ color: "var(--text-2)" }}>
          Terminal · {title || sessionName}
        </span>
        <div class="ml-auto flex items-center gap-1">
          <button
            onClick={onShowChat || onClose}
            class="h-6 px-2 rounded text-[11px] font-medium"
            style={{ color: "var(--text-2)", border: "1px solid var(--border)" }}
            title="show chat"
          >Chat</button>
          <button
            onClick={onClose}
            class="h-6 w-6 grid place-items-center rounded"
            style={{ color: "var(--text-3)" }}
            title="back to chat"
            aria-label="back to chat"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
      </div>
      <div ref={hostRef} class="flex-1 min-h-0 overflow-hidden" style={{ height: "100%" }} />
    </div>
  );
}
