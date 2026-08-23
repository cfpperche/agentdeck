// AgentDeck claude SDK shim — ADR-0005.
// Speaks the AgentDeck wire protocol (ADR-0004 shapes) on stdin/stdout
// while driving the real claude via the Agent SDK:
//
//   stdin:  {"type":"user",...} | {"type":"control_response",...}
//   stdout: init/assistant(tool_use|text)/control_request/result events
//
// Env knobs (test doubles):
//   AGENTDECK_SDK_FAKE=1  drive FakeSDK instead of the real agent
//   AGENTDECK_SDK_FAKE_*  behavior knobs (see fake.mjs)
import { createRequire } from "node:module";
import readline from "node:readline";
const require = createRequire(import.meta.url);

const emit = (o) => process.stdout.write(JSON.stringify(o) + "\n");
const onLine = async (raw, handlers) => {
  let msg;
  try { msg = JSON.parse(raw); } catch { return; }
  if (msg.type === "user" && handlers.onUser) await handlers.onUser(msg);
  if (msg.type === "control_response" && handlers.onControl)
    await handlers.onControl(msg);
};

const pendingPermissions = new Map(); // request_id -> resolve(result)

function normalize(message, ctx) {
  // ctx: { sessionIdRef } — updated from sdk-init
  const out = [];
  switch (message.type) {
    case "system":
      if (message.subtype === "init") {
        ctx.sessionId = message.session_id;
        out.push({ type: "system", subtype: "init", session_id: message.session_id });
      }
      break;
    case "assistant":
      for (const b of message.message?.content ?? []) {
        if (b.type === "tool_use")
          out.push({ type: "assistant", message: { role: "assistant", content: [b] } });
        if (b.type === "text" && b.text)
          out.push({ type: "assistant", message: { role: "assistant", content: [{ type: "text", text: b.text }] } });
      }
      break;
    case "result":
      out.push({
        type: "result",
        subtype: message.subtype || "success",
        result: message.result ?? "",
        session_id: ctx.sessionId ?? message.session_id,
      });
      break;
  }
  return out;
}

async function main() {
  const useFake = process.env.AGENTDECK_SDK_FAKE === "1";
  const { query } = useFake
    ? await import("./fake.mjs")
    : { query: require("@anthropic-ai/claude-agent-sdk").query };

  const ctx = { sessionId: null };
  let resume = process.env.AGENTDECK_SDK_RESUME || undefined;

  emit({ type: "shim-ready", mode: useFake ? "fake" : "sdk" });

  const runTurn = async (text) => {
    const q = await query({
      prompt: text,
      options: {
        cwd: process.cwd(),
        // manual = every tool call asks via canUseTool (AgentDeck always
        // answers through control_response); env can override per-deployment
        permissionMode: process.env.AGENTDECK_SDK_PERMISSION_MODE || "manual",
        // When AgentDeck is listening (it always is, via control_response),
        // 'ask' decisions must surface as canUseTool calls, never auto-deny.
        ...(resume ? { resume } : {}),
        canUseTool: async (toolName, input, { request_id }) => {
          emit({
            type: "control_request",
            request_id,
            tool_name: toolName,
            input,
          });
          return await new Promise((resolve) => {
            pendingPermissions.set(request_id, resolve);
          });
        },
      },
    });
    for await (const message of q) {
      if (message.type === "system" && message.subtype === "init") {
        ctx.sessionId = message.session_id;
        resume = message.session_id; // keep the native ref for next turns
      }
      for (const ev of normalize(message, ctx)) emit(ev);
    }
  };

  const handlers = {
    onUser: (msg) => {
      const text = (msg.message?.content ?? [])
        .filter((b) => b.type === "text")
        .map((b) => b.text)
        .join("\n");
      if (text) runTurn(text).catch((e) =>
        emit({ type: "result", subtype: "error_during_execution", result: String(e), session_id: ctx.sessionId }));
    },
    onControl: (msg) => {
      const resolve = pendingPermissions.get(msg.request_id);
      if (!resolve) return;
      pendingPermissions.delete(msg.request_id);
      const behavior = msg.response?.behavior === "deny" ? "deny" : "allow";
      const updated =
        behavior === "allow" && msg.response?.updatedInput
          ? msg.response.updatedInput
          : undefined;
      resolve({ behavior, updatedInput: updated });
    },
  };

  // CRITICAL: iterate stdin by LINE, not by chunk — a fast writer
  // (the Go runner) coalesces multiple JSON messages into one pipe
  // chunk, and JSON.parse of glued objects silently drops them all.
  const rl = readline.createInterface({ input: process.stdin });
  rl.on("line", (raw) => { onLine(String(raw).trim(), handlers); });
  await new Promise((resolve) => rl.on("close", resolve));
  // stdin closed: AgentDeck is shutting us down
  process.exit(0);
}

main().catch((e) => {
  emit({ type: "result", subtype: "error", result: String(e) });
  process.exit(1);
});
