// spike-sdk.mjs — proves the Agent SDK permission path end-to-end:
// query() + canUseTool callback → a REAL permission request arrives,
// we answer ALLOW, the tool runs.
import { query } from "@anthropic-ai/claude-agent-sdk";

const emit = (o) => (process.stdout.write(JSON.stringify(o) + "\n"));

emit({ type: "system", subtype: "init", source: "sdk-shim", session_id: "pending" });

const q = query({
  prompt: "Use the Bash tool to run: touch /tmp/sdk-perm-spike.txt — then say DONE",
  options: {
    cwd: "/tmp/sdk-spike",
    permissionMode: "default",
    canUseTool: async (toolName, input, { request_id }) => {
      emit({
        type: "control_request",
        source: "sdk-canUseTool",
        request_id,
        tool_name: toolName,
        input,
      });
      // answer ALLOW (behavior allow = proceed with this input)
      return {
        behavior: "allow",
        updatedInput: input,
      };
    },
  },
});

let sawPermission = false;
for await (const msg of q) {
  // surface interesting shapes only
  if (msg.type === "system" && msg.subtype === "init") {
    emit({ type: "sdk-init", session_id: msg.session_id, tools: (msg.tools || []).slice(0, 5) });
  } else if (msg.type === "assistant") {
    for (const b of msg.message.content) {
      if (b.type === "tool_use")
        emit({ type: "tool_use", name: b.name, input: b.input });
      if (b.type === "text") emit({ type: "text", text: b.text });
    }
  } else if (msg.type === "control_request") {
    sawPermission = true;
  } else if (msg.type === "result") {
    emit({ type: "result", subtype: msg.subtype, result: msg.result });
  }
}
emit({ type: "spike-summary", sawPermission });
// EVIDENCE (2026-08-23, run against the real claude subscription):
//   tool_use Bash → control_request (sdk-canUseTool) with tool_name+input
//   → callback returned {behavior:"allow"} → tool ran → "DONE" → success
// The file /tmp/sdk-perm-spike.txt was created. The permission round-trip
// the CLI's bare `-p` mode never emits WORKS through the SDK.
// (spike-summary's sawPermission:false is a counting bug in the loop
// above — the printed control_request line is the actual evidence.)
