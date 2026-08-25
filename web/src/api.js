/// <reference path="./api-types.d.ts" />
const json = (r) => (r.ok ? r.json() : r.json().then((e) => Promise.reject(e)));

export const api = {
  agents: () => fetch("/api/agents").then(json),
  sessions: () => fetch("/api/sessions").then(json),
  createSession: (agent, cwd) =>
    fetch("/api/sessions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cwd ? { agent, cwd } : { agent }),
    }).then(json),
  renameSession: (id, title) =>
    fetch(`/api/sessions/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title }),
    }).then(json),
  deleteSession: (id) =>
    fetch(`/api/sessions/${id}`, { method: "DELETE" }).then(json),
  messages: (id) => fetch(`/api/sessions/${id}/messages`).then(json),
  send: (id, text, controls) =>
    fetch(`/api/sessions/${id}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(controls ? { text, controls } : { text }),
    }).then(json),
  control: (id, requestId, behavior, updatedInput) =>
    fetch(`/api/sessions/${id}/control`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(updatedInput
        ? { request_id: requestId, behavior, updatedInput }
        : { request_id: requestId, behavior }),
    }).then(json),
  stop: (id) => fetch(`/api/sessions/${id}/stop`, { method: "POST" }).then(json),
  clearQueue: (id) =>
    fetch(`/api/sessions/${id}/queue/cancel`, { method: "POST" }).then(json),
  terminal: (id) => fetch(`/api/sessions/${id}/terminal`).then(json),
  openTerminal: (id) =>
    fetch(`/api/sessions/${id}/terminal`, { method: "POST" }).then(json),
  closeTerminal: (id) =>
    fetch(`/api/sessions/${id}/terminal`, { method: "DELETE" }).then(json),
  status: (id) => fetch(`/api/sessions/${id}/status`).then(json),
  files: (path, q, signal) => {
    const u = new URL("/api/fs/files", location.origin);
    u.searchParams.set("path", path);
    if (q) u.searchParams.set("q", q);
    return fetch(u, { signal }).then(json);
  },
};

export function openEvents(sid, onEvent) {
  const es = new EventSource(`/api/sessions/${sid}/events`);
  es.onmessage = (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch {}
  };
  return es;
}
