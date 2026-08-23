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

  let text;
  if (low.startsWith("remember:")) {
    mem.fact = prompt.slice("remember:".length).trim();
    text = "OK";
  } else if (low.includes("what do you remember")) {
    text = mem.fact || "nothing";
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
  } else {
    text = "echo: " + prompt;
  }

  messages.push({
    type: "assistant",
    message: { role: "assistant", content: [{ type: "text", text }] },
  });
  messages.push({ type: "result", subtype: "success", result: text, session_id: sid });
  return messages;
}
