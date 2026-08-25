// AgentDeck-native slash catalog (PiCode shape, our commands).
// Receipt: docs/benchmarks/2026-08-24-composer-mentions-slash.md

export const SLASH = [
  { id: "new", label: "/new", hint: "New session", run: "new" },
  { id: "term", label: "/term", hint: "Toggle terminal dock", run: "term" },
  { id: "stop", label: "/stop", hint: "Stop the agent", run: "stop" },
  { id: "settings", label: "/settings", hint: "Open settings", run: "settings" },
  { id: "devices", label: "/devices", hint: "Paired devices", run: "devices" },
  { id: "system", label: "/system", hint: "Host and dependencies", run: "system" },
];

export function filterSlash(q) {
  const s = (q || "").trim();
  if (!s.startsWith("/")) return [];
  const needle = s.slice(1).toLowerCase();
  return SLASH.filter((c) => c.label.slice(1).startsWith(needle) || c.id.startsWith(needle));
}

// @token immediately left of the caret: "@foo" after whitespace or start.
export function parseAtToken(text, caret) {
  const t = text || "";
  const end = caret == null ? t.length : caret;
  const left = t.slice(0, end);
  const m = left.match(/(^|[\s])@([^\s]*)$/);
  if (!m) return null;
  const query = m[2];
  const start = left.length - query.length - 1;
  return { start, end, query };
}

export function insertMention(text, token, rel) {
  const t = text || "";
  const before = t.slice(0, token.start);
  const after = t.slice(token.end);
  const ins = "@" + rel + " ";
  return { text: before + ins + after, caret: before.length + ins.length };
}
