import { useEffect, useRef, useState, useCallback } from "react";
import { api } from "./api.js";
import { Sidebar, Logo } from "./components.jsx";
import { AgentIcon } from "./icons.jsx";
import { Chat } from "./chat.jsx";
import { SettingsPanel } from "./components.jsx";
import { useTheme } from "./theme.js";
import { NewSessionPanel } from "./newsession.jsx";
import { ShareDrawer } from "./share.jsx";
import { DevicesPanel } from "./devices.jsx";

const NEW_TAB_ID = "__new__";

// App shell: sidebar + open-session TABS (editor-style). All open tabs
// stay mounted (hidden ones keep SSE/drafts/permissions alive).
export function App() {
  const theme = useTheme();
  const [agents, setAgents] = useState([]);
  const [sessions, setSessions] = useState([]);
  const [openTabs, setOpenTabs] = useState([]); // session objects
  const [activeId, setActiveId] = useState(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [devicesOpen, setDevicesOpen] = useState(false);
  const closeOverlays = () => { setSettingsOpen(false); setDevicesOpen(false); };
  const [shareOpen, setShareOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [isNarrow, setIsNarrow] = useState(() => matchMedia("(max-width: 767px)").matches);
  useEffect(() => {
    const mq = matchMedia("(max-width: 767px)");
    const on = () => setIsNarrow(mq.matches);
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);

  const agentById = Object.fromEntries(agents.map((a) => [a.id, a]));

  const refreshSessions = useCallback(() => {
    api.sessions().then(setSessions).catch(() => {});
  }, []);

  // ---- URL: ?tab=<active>&tabs=<a,b,c> (home = no tab) ----
  const parseURL = () => {
    const p = new URLSearchParams(location.search);
    const tabs = (p.get("tabs") || "").split(",").filter(Boolean);
    const tab = p.get("tab");
    if (p.get("s")) return { legacy: p.get("s") }; // pre-tabs deep-link
    // /s/<id> path deep-link (shared links, history entries)
    const m = location.pathname.match(/^\/s\/([A-Za-z0-9_-]+)\/?$/);
    if (m) return { legacy: m[1] };
    return { tabs, tab: tabs.includes(tab) ? tab : tabs[0] || null };
  };

  useEffect(() => {
    api.agents().then(setAgents).catch(() => {});
    refreshSessions();
    const { tabs, tab, legacy } = parseURL();
    if (legacy) {
      // normalize legacy /s/<id> deep-links into the tab model
      history.replaceState({}, "", `/?tabs=${legacy}&tab=${legacy}`);
      setOpenTabsAndActive([legacy], legacy, false);
    } else if (tabs.length) {
      setOpenTabsAndActive(tabs, tab, false);
    }
    if (location.pathname.startsWith("/settings")) setSettingsOpen(true);
    if (location.pathname.startsWith("/devices")) setDevicesOpen(true);
  }, []);

  useEffect(() => {
    const on = () => setShareOpen(true);
    addEventListener("agentdeck-share", on);
    return () => removeEventListener("agentdeck-share", on);
  }, []);

  useEffect(() => {
    const ping = () => {
      let id = localStorage.getItem("agentdeck-device-id");
      if (!id) { id = crypto.randomUUID(); localStorage.setItem("agentdeck-device-id", id); }
      const host = location.hostname === "localhost" || location.hostname === "127.0.0.1";
      fetch("/api/devices/ping", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id, host }),
      }).catch(() => {});
    };
    ping();
    const t = setInterval(ping, 15000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    const onPop = () => {
      setSettingsOpen(location.pathname.startsWith("/settings"));
      setDevicesOpen(location.pathname.startsWith("/devices"));
      const { tabs, tab } = parseURL();
      setOpenTabs((prev) => {
        // keep session objects we already have; fetch titles later
        const byId = Object.fromEntries(prev.map((t) => [t.id, t]));
        return tabs.map((id) => byId[id] || { id, title: "", agent: "" });
      });
      setActiveId(tab);
    };
    addEventListener("popstate", onPop);
    return () => removeEventListener("popstate", onPop);
  }, []);

  // resolve tab titles/agents from the sessions list as it loads
  useEffect(() => {
    setOpenTabs((prev) =>
      prev.map((t) => {
        const s = sessions.find((x) => x.id === t.id);
        return s ? s : t;
      })
    );
  }, [sessions]);

  const setOpenTabsAndActive = (ids, active, push = true) => {
    const byId = Object.fromEntries(
      [...openTabs, ...sessions].map((t) => [t.id, t])
    );
    const tabs = ids.map((id) => byId[id] || { id, title: "", agent: "" });
    setOpenTabs(tabs);
    setActiveId(active);
    if (push) {
      const q = new URLSearchParams();
      if (ids.length) {
        q.set("tabs", ids.join(","));
        if (active) q.set("tab", active);
      }
      history.pushState({}, "", `${location.pathname}?${q}`);
    }
  };

  // ---- actions ----
  const openSession = (id) => {
    closeOverlays();
    const ids = openTabs.map((t) => t.id);
    if (!ids.includes(id)) setOpenTabsAndActive([...ids, id], id);
    else setOpenTabsAndActive(ids, id);
  };

  // instant create (home chips) — config tab is the sidebar's New session
  const openNewSession = (agentId) => {
    const agent = agentId || agents[0]?.id;
    if (!agent) return;
    api.createSession(agent).then((s) => {
      refreshSessions();
      const ids = openTabs.map((t) => t.id).filter((x) => x !== NEW_TAB_ID);
      setOpenTabsAndActive([...ids, s.id], s.id);
    });
  };

  // "New session" opens a CONFIG TAB (runtime picker today; extensible
  // to post-runtime options). Home agent chips create immediately.
  const openNewSessionTab = () => {
    const ids = openTabs.map((t) => t.id);
    if (!ids.includes(NEW_TAB_ID)) setOpenTabsAndActive([...ids, NEW_TAB_ID], NEW_TAB_ID);
    else setOpenTabsAndActive(ids, NEW_TAB_ID);
  };

  const closeTab = (id) => {
    const ids = openTabs.map((t) => t.id).filter((x) => x !== id);
    const nextActive = activeId === id ? ids[ids.length - 1] || null : activeId;
    setOpenTabsAndActive(ids, nextActive);
    api.deleteSession; // no — closing a tab does NOT delete the session
  };

  const activeTab = openTabs.find((t) => t.id === activeId);

  return (
    <div class="flex h-full overflow-hidden">
      <Sidebar
        sessions={sessions}
        agents={agents}
        activeId={activeId}
        filter={filter}
        setFilter={setFilter}
        onOpen={openSession}
        onNew={() => { closeOverlays(); openNewSessionTab(); }}
        onRename={(id, t) => api.renameSession(id, t).then(refreshSessions)}
        onDelete={(id) => {
          api.deleteSession(id).then(() => {
            closeTab(id);
            refreshSessions();
          });
        }}
        open={sidebarOpen}
        setOpen={setSidebarOpen}
        theme={theme.current}
        onToggleTheme={theme.toggle}
        onOpenSettings={() => { closeOverlays(); setSettingsOpen(true); setActiveId(null); history.pushState({}, "", "/settings"); }}
        onOpenDevices={() => { setSettingsOpen(false); setDevicesOpen(true); setActiveId(null); history.pushState({}, "", "/devices"); }}
      />

      <main class="flex-1 flex flex-col min-w-0" style={{ background: "var(--bg-canvas)" }}>
        {/* tab bar */}
        <div class="flex items-stretch h-9 shrink-0 overflow-x-auto" style={{ background: "var(--bg-panel)", borderBottom: "1px solid var(--border-soft)" }}>

          {openTabs.map((t) => {
            const active = t.id === activeId;
            const ag = agentById[t.agent];
            const isNew = t.id === NEW_TAB_ID;
            return (
              <div
                key={t.id}
                onClick={() => { closeOverlays(); setOpenTabsAndActive(openTabs.map((x) => x.id), t.id); }}
                class={`group flex items-center gap-2 pl-3 pr-1.5 cursor-pointer text-[12.5px] shrink-0 max-w-[220px] surface ${active ? "" : "hover:bg-[color:var(--bg-hover)]"}`}
                style={{
                  borderRight: "1px solid var(--border-soft)",
                  background: active ? "var(--bg-canvas)" : "transparent",
                  color: active ? "var(--text-1)" : "var(--text-2)",
                  boxShadow: active ? "inset 0 2px 0 0 var(--accent)" : "none",
                }}
                title={t.title}
              >
                {isNew ? (
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
                ) : ag ? (
                  <AgentIcon id={t.agent} size={12} color={ag.color} />
                ) : null}
                <span class="truncate">{isNew ? "New session" : (t.title || "untitled")}</span>
                <button
                  onClick={(e) => { e.stopPropagation(); closeTab(t.id); }}
                  class="h-5 w-5 grid place-items-center rounded opacity-0 group-hover:opacity-100"
                  style={{ color: "var(--text-3)" }}
                  aria-label="close tab"
                >
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
                </button>
              </div>
            );
          })}
          {/* new tab = home (hidden when no tabs: the screen IS home) */}
          {openTabs.length > 0 && (
          <div
            onClick={() => { closeOverlays(); setOpenTabsAndActive(openTabs.map((x) => x.id), null); }}
            class="flex items-center px-3 cursor-pointer text-[12.5px] shrink-0 surface"
            style={{
              borderRight: "1px solid var(--border-soft)",
              background: activeId === null && !settingsOpen ? "var(--bg-canvas)" : "transparent",
              color: activeId === null && !settingsOpen ? "var(--text-1)" : "var(--text-3)",
              boxShadow: activeId === null && !settingsOpen ? "inset 0 2px 0 0 var(--accent)" : "none",
            }}
            title="home"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12h18M12 3v18"/></svg>
          </div>
          )}
        </div>

        {/* views: all tabs stay mounted; hidden ones keep their state */}
        {settingsOpen ? (
          <SettingsPanel themePref={theme.pref} currentTheme={theme.current} onSetTheme={theme.setPref} onClose={() => { closeOverlays(); history.pushState({}, "/"); }} />
        ) : devicesOpen ? (
          <DevicesPanel onClose={() => { closeOverlays(); history.pushState({}, "/"); }} />
        ) : activeId === NEW_TAB_ID ? (
          <NewSessionPanel
            agents={agents}
            onCreate={(agentId, cwd) => {
              api.createSession(agentId, cwd).then((s) => {
                refreshSessions();
                const ids = openTabs.map((t) => t.id).filter((x) => x !== NEW_TAB_ID);
                setOpenTabsAndActive([...ids, s.id], s.id);
              });
            }}
          />
        ) : activeTab ? (
          <>
            {openTabs.filter((t) => t.id !== NEW_TAB_ID).map((t) => (
              <div key={t.id} style={{ display: t.id === activeId ? "flex" : "none" }} class="flex-1 min-h-0">
                <Chat
                  session={t}
                  agentMeta={agentById[t.agent]}
                  onOpenSidebar={() => setSidebarOpen(true)}
                  onSessionUpdated={refreshSessions}
                  onStop={() => api.stop(t.id).then(refreshSessions)}
                />
              </div>
            ))}
          </>
        ) : (
          <Home agents={agents} onNew={openNewSession} onOpenSession={openSession} recent={sessions.slice(0, 6)} showRecent={sidebarOpen || isNarrow} />
        )}
      </main>
      <ShareDrawer open={shareOpen} onClose={() => setShareOpen(false)} />
    </div>
  );
}

// Home: hero + agent picker + recent sessions (opens as tabs)
function Home({ agents, onNew, onOpenSession, recent, showRecent = false }) {
  return (
    <div class="flex-1 overflow-y-auto flex flex-col">
      <div class="max-w-md mx-auto w-full px-6 py-16 text-center flex-1 flex flex-col justify-center">
        <div class="inline-grid place-items-center h-14 w-14 rounded-2xl mb-5"
          style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }}>
          <Logo size={26} />
        </div>
        <h2 class="text-2xl font-semibold tracking-tight mb-2" style={{ color: "var(--text-1)" }}>AgentDeck</h2>
        <p class="text-[15px] leading-relaxed balance mb-9" style={{ color: "var(--text-2)" }}>
          Talk to your local coding agents from the browser — from anywhere.
        </p>

        <p class="text-[11px] uppercase tracking-wider mb-3" style={{ color: "var(--text-3)" }}>start a session</p>
        <div class="flex flex-wrap justify-center gap-2 max-w-[360px] mx-auto">
          {agents.map((a) => (
            <button key={a.id} onClick={() => onNew(a.id)}
              class="chip flex flex-col items-center justify-center gap-2.5 h-[84px] w-[108px] rounded-xl text-sm active:scale-[0.97] transition-all"
              style={{ background: "var(--bg-card)", border: "1px solid var(--border)" }}>
              <AgentIcon id={a.id} size={22} color={a.color} />
              <span class="text-[13px]" style={{ color: "var(--text-2)" }}>{a.label}</span>
            </button>
          ))}
        </div>

        {recent.length > 0 && showRecent && (
          <>
            <p class="text-[11px] uppercase tracking-wider mt-12 mb-3" style={{ color: "var(--text-3)" }}>recent</p>
            <div class="text-left rounded-xl overflow-hidden" style={{ border: "1px solid var(--border)" }}>
              {recent.map((s, i) => (
                <button key={s.id} onClick={() => onOpenSession(s.id)}
                  class="w-full flex items-center gap-2.5 px-4 py-2.5 surface hover:bg-[color:var(--bg-hover)] text-left"
                  style={{ background: "var(--bg-card)", borderTop: i ? "1px solid var(--border-soft)" : "none" }}>
                  <span class="h-1.5 w-1.5 rounded-full shrink-0" style={{ background: s.agent === "claude" ? "#E07856" : "#888" }} />
                  <span class="flex-1 truncate text-[13px]" style={{ color: "var(--text-1)" }}>{s.title || "untitled"}</span>
                  <span class="text-[11px] shrink-0" style={{ color: "var(--text-3)" }}>{s.message_count} msg</span>
                </button>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
