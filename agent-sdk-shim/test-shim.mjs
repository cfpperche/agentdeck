// Wire-contract test for the shim against the fake SDK (no tokens).
// Drives stdin and asserts stdout event shapes (ADR-0004 + control_request).
import { spawn } from "node:child_process";

const env = { ...process.env, AGENTDECK_SDK_FAKE: "1", FAKE_ASK: "1" };
const p = spawn("node", ["shim.mjs"], { env });
const out = [];
let buf = "";
p.stdout.on("data", (d) => {
  buf += d;
  let i;
  while ((i = buf.indexOf("\n")) >= 0) {
    const line = buf.slice(0, i); buf = buf.slice(i + 1);
    if (line.trim()) out.push(JSON.parse(line));
  }
});
const send = (o) => p.stdin.write(JSON.stringify(o) + "\n");
const waitFor = (pred, ms = 8000) =>
  new Promise((res, rej) => {
    const t0 = Date.now();
    const tick = setInterval(() => {
      if (out.some(pred)) { clearInterval(tick); res(); }
      else if (Date.now() - t0 > ms) { clearInterval(tick); rej(new Error("timeout: " + JSON.stringify(out))); }
    }, 50);
  });

const assert = (c, m) => { if (!c) { console.error("FAIL:", m, JSON.stringify(out)); process.exit(1); } };

// 1. memory across turns (same shim process)
send({ type: "user", message: { role: "user", content: [{ type: "text", text: "Remember: sdk rocks" }] } });
await waitFor((e) => e.type === "result");
send({ type: "user", message: { role: "user", content: [{ type: "text", text: "What do you remember?" }] } });
await waitFor((e) => e.type === "result" && e.result === "sdk rocks");
assert(true, "memory across turns");

// 2. permission round-trip: request arrives, we DENY, agent reports denial
send({ type: "user", message: { role: "user", content: [{ type: "text", text: "Write file now" }] } });
await waitFor((e) => e.type === "control_request");
const req = out.find((e) => e.type === "control_request");
assert(req.tool_name === "Bash" && req.input?.command, "control_request carries tool+input");
send({ type: "control_response", request_id: req.request_id, response: { behavior: "deny" } });
await waitFor((e) => e.type === "result" && e.result === "Permission denied by user");

// 3. allow path: file created
send({ type: "user", message: { role: "user", content: [{ type: "text", text: "Write file now" }] } });
await waitFor((e) => e.type === "control_request" && e.request_id !== req.request_id);
const req2 = out.find((e) => e.type === "control_request" && e.request_id !== req.request_id);
send({ type: "control_response", request_id: req2.request_id, response: { behavior: "allow" } });
await waitFor((e) => e.type === "result" && e.result === "File written");
const fs = await import("node:fs");
assert(fs.existsSync("/tmp/agentdeck-sdk-fake.txt"), "file created after allow");

console.log("SHIM CONTRACT TESTS PASSED ✓ (memory, deny, allow)");
p.kill();
process.exit(0);
