// Conversation rail anchors (PiCode shape).
// Receipt: docs/benchmarks/2026-08-24-picode-conversation-rail.md

export function railAnchors(messages, agentLabel, streamText) {
  const out = [];
  (messages || []).forEach((m, i) => {
    if (!m || (m.role !== "user" && m.role !== "assistant")) return;
    const text = String(m.content || "").replace(/\s+/g, " ").trim();
    if (!text) return;
    const isUser = m.role === "user";
    out.push({
      id: "msg-" + (m.id != null ? m.id : i),
      actor: isUser ? "You" : (agentLabel || "Agent"),
      cls: isUser ? "user" : "",
      preview: text.length > 140 ? text.slice(0, 137) + "…" : text,
    });
  });
  const live = String(streamText || "").replace(/\s+/g, " ").trim();
  if (live) {
    out.push({
      id: "msg-stream",
      actor: agentLabel || "Agent",
      cls: "",
      preview: live.length > 140 ? live.slice(0, 137) + "…" : live,
    });
  }
  return out;
}
