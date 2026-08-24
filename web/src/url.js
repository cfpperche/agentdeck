// Tab-model URL helpers.
//
// Chat always lives at `/` with `?tabs=<ids>&tab=<active>`. Overlay
// routes (`/settings`, `/devices`) keep the same query so closing them
// restores the exact tab set. They must never leak into chat navigation
// — `setOpenTabsAndActive` used to push `${location.pathname}?…`, which
// glued /devices and /settings onto every subsequent click.

export function parseAppURL(loc = window.location) {
  const p = new URLSearchParams(loc.search);
  const tabs = (p.get("tabs") || "").split(",").filter(Boolean);
  const rawTab = p.get("tab");
  const tab = tabs.includes(rawTab) ? rawTab : tabs[0] || null;

  if (p.get("s")) {
    const id = p.get("s");
    return { overlay: null, tabs: [id], tab: id, legacy: id };
  }
  const m = (loc.pathname || "").match(/^\/s\/([A-Za-z0-9_-]+)\/?$/);
  if (m) return { overlay: null, tabs: [m[1]], tab: m[1], legacy: m[1] };

  let overlay = null;
  if ((loc.pathname || "").startsWith("/settings")) overlay = "settings";
  else if ((loc.pathname || "").startsWith("/devices")) overlay = "devices";
  return { overlay, tabs, tab, legacy: null };
}

export function chatPath(tabs, tab) {
  const ids = (tabs || []).filter(Boolean);
  const q = new URLSearchParams();
  if (ids.length) {
    q.set("tabs", ids.join(","));
    if (tab && ids.includes(tab)) q.set("tab", tab);
  }
  const qs = q.toString();
  return qs ? `/?${qs}` : "/";
}

export function overlayPath(kind, tabs, tab) {
  const chat = chatPath(tabs, tab);
  const qs = chat.startsWith("/?") ? chat.slice(1) : "";
  return `/${kind}${qs}`;
}
