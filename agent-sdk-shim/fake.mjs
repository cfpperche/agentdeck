// Fake Agent SDK — deterministic test double for the shim (ADR-0005).
// Mirrors the subset of @anthropic-ai/claude-agent-sdk the shim uses:
// an async-iterable of messages with init/assistant/result, calling
// canUseTool for "sensitive" turns.
//
// Knobs via env:
//   FAKE_SESSION_ID        session id reported at init (default fake-sdk-42)
//   FAKE_ASK=1             emit a canUseTool for Bash before writing
//   FAKE_MEMORY=1          recall the last fact across turns in-memory
export async function query({ prompt, options }) {
  const sid = process.env.FAKE_SESSION_ID || "fake-sdk-42";
  const messages = [];
  globalThis.__fakeMemory ??= {};
  globalThis.__fakePermSeq ??= 0;
  const mem = globalThis.__fakeMemory;

  const low = prompt.toLowerCase();
  messages.push({ type: "system", subtype: "init", session_id: sid });
  messages.push({
    type: "capabilities",
    models: [
      { id: "sonnet", label: "Fake Sonnet", is_default: true,
        thinking_options: [{ id: "off", label: "Standard" }, { id: "on", label: "Thinking" }] },
      { id: "haiku", label: "Fake Haiku" },
    ],
    modes: [
      { id: "manual", label: "Ask before edits" },
      { id: "acceptEdits", label: "Auto-accept edits" },
    ],
  });

  let text;
  if (low.startsWith("remember:")) {
    mem.fact = prompt.slice("remember:".length).trim();
    text = "OK";
  } else if (low.includes("what do you remember")) {
    text = mem.fact || "nothing";
  } else if (low.startsWith("two permissions")) {
    // Two permission requests; serial by default (matches the real
    // SDK's turn semantics), PARALLEL=1 fires both before resolving.
    if (!options?.canUseTool) { text = "no canUseTool"; messages_done(); }
    else if (process.env.FAKE_PARALLEL === "1") {
      const p1 = options.canUseTool("Bash", { command: "first-cmd" }, { request_id: `perm-fake-${++globalThis.__fakePermSeq}` });
      const p2 = options.canUseTool("Write", { file_path: "/tmp/x" }, { request_id: `perm-fake-${++globalThis.__fakePermSeq}` });
      const [d1, d2] = await Promise.all([p1, p2]);
      text = await applyTwo(d1, d2);
    } else {
      const d1 = await options.canUseTool("Bash", { command: "first-cmd" }, { request_id: `perm-fake-${++globalThis.__fakePermSeq}` });
      const d2 = await options.canUseTool("Write", { file_path: "/tmp/x" }, { request_id: `perm-fake-${++globalThis.__fakePermSeq}` });
      text = await applyTwo(d1, d2);
    }
  } else if (low.startsWith("write file")) {
    if (process.env.FAKE_ASK === "1" && options?.canUseTool) {
      const decision = await options.canUseTool(
        "Bash",
        { command: "echo hi > /tmp/agentdeck-sdk-fake.txt" },
        { request_id: `perm-fake-${++globalThis.__fakePermSeq}` }
      );
      if (decision.behavior === "deny") {
        messages.push({
          type: "assistant",
          message: { role: "assistant", content: [{ type: "text", text: "Permission denied by user" }] },
        });
        messages.push({ type: "result", subtype: "success", result: "Permission denied by user", session_id: sid });
        return messages;
      }
    }
    const fs = await import("node:fs");
    fs.writeFileSync("/tmp/agentdeck-sdk-fake.txt", "hi");
    messages.push({
      type: "assistant",
      message: { role: "assistant", content: [{ type: "tool_use", name: "Bash", input: { command: "echo hi > /tmp/agentdeck-sdk-fake.txt" } }] },
    });
    text = "File written";
  } else if (low.includes("which model")) {
    // ADR-0006 proof: reflects the model option the shim actually passed
    text = options?.model ? "model: " + options.model : "model: default";
  } else {
    text = "echo: " + prompt;
  }


  async function applyTwo(d1, d2) {
    let out;
    if (d1.behavior === "deny") out = "Permission denied by user";
    else if (d1.updatedInput && d1.updatedInput.command !== "first-cmd") {
      const fs = await import("node:fs");
      fs.writeFileSync("/tmp/agentdeck-edited-input.txt", d1.updatedInput.command);
      out = "executed: " + d1.updatedInput.command;
    } else out = "executed: first-cmd";
    out += d2.behavior === "deny" ? " (+second denied)" : " (+second allowed)";
    return out;
  }

  messages.push({
    type: "assistant",
    message: { role: "assistant", content: [{ type: "text", text }] },
  });
  messages.push({
    type: "result", subtype: "success", result: text, session_id: sid,
    usage: { input_tokens: 8, output_tokens: 2, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 },
    total_cost_usd: 0,
  });
  return messages;
}
