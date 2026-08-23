const json = (r) => (r.ok ? r.json() : r.json().then((e) => Promise.reject(e)));

export const api = {
  agents: () => fetch("/api/agents").then(json),
  sessions: () => fetch("/api/sessions").then(json),
  createSession: (agent, title) =>
    fetch("/api/sessions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ agent, title }),
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
  send: (id, text) =>
    fetch(`/api/sessions/${id}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    }).then(json),
  stop: (id) => fetch(`/api/sessions/${id}/stop`, { method: "POST" }).then(json),
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
