import { describe, it, expect, vi, beforeEach } from "vitest";

// api.js contract tests — mock fetch; these encode the HTTP surface
// the Go server must keep (drift breaks here before it breaks users).

function jsonResponse(data, ok = true) {
  return { ok, json: () => Promise.resolve(data) };
}

describe("api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    global.fetch = vi.fn();
  });

  it("agents() GETs /api/agents", async () => {
    global.fetch.mockResolvedValue(jsonResponse([{ id: "claude" }]));
    const { api } = await import("./api.js");
    const out = await api.agents();
    expect(global.fetch).toHaveBeenCalledWith("/api/agents");
    expect(out).toEqual([{ id: "claude" }]);
  });

  it("createSession sends agent only when no cwd", async () => {
    global.fetch.mockResolvedValue(jsonResponse({ id: "s1" }));
    const { api } = await import("./api.js");
    await api.createSession("claude");
    const [url, init] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/sessions");
    expect(JSON.parse(init.body)).toEqual({ agent: "claude" });
  });

  it("createSession includes cwd when provided", async () => {
    global.fetch.mockResolvedValue(jsonResponse({ id: "s1" }));
    const { api } = await import("./api.js");
    await api.createSession("claude", "/tmp/x");
    const body = JSON.parse(global.fetch.mock.calls[0][1].body);
    expect(body).toEqual({ agent: "claude", cwd: "/tmp/x" });
  });

  it("control sends updatedInput only when present", async () => {
    global.fetch.mockResolvedValue(jsonResponse({ ok: true }));
    const { api } = await import("./api.js");
    await api.control("s1", "r1", "allow");
    expect(JSON.parse(global.fetch.mock.calls[0][1].body)).toEqual({
      request_id: "r1", behavior: "allow",
    });
    await api.control("s1", "r2", "allow", { command: "safe" });
    expect(JSON.parse(global.fetch.mock.calls[1][1].body)).toEqual({
      request_id: "r2", behavior: "allow", updatedInput: { command: "safe" },
    });
  });

  it("stop POSTs the stop route", async () => {
    global.fetch.mockResolvedValue(jsonResponse({ ok: true }));
    const { api } = await import("./api.js");
    await api.stop("s1");
    const [url, init] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/sessions/s1/stop");
    expect(init.method).toBe("POST");
  });
});

describe("openEvents", () => {
  it("parses SSE messages and closes on demand", async () => {
    const close = vi.fn();
    const inst = { onmessage: null, close };
    global.EventSource = vi.fn(() => inst);
    const { openEvents } = await import("./api.js");
    const seen = [];
    const es = openEvents("s1", (ev) => seen.push(ev));
    inst.onmessage({ data: JSON.stringify({ type: "state", running: true }) });
    expect(seen).toEqual([{ type: "state", running: true }]);
    // malformed JSON is ignored, not thrown
    inst.onmessage({ data: "{oops" });
    expect(seen.length).toBe(1);
    // close passthrough
    es.close();
    expect(close).toHaveBeenCalled();
  });
});
