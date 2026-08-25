import { describe, it, expect } from "vitest";
import { railAnchors } from "./rail.js";

describe("railAnchors", () => {
  it("skips empty and labels by runtime", () => {
    const a = railAnchors([
      { id: 1, role: "user", content: "Hello there" },
      { id: 2, role: "assistant", content: "  " },
      { id: 3, role: "assistant", content: "Answer" },
    ], "Claude");
    expect(a.length).toBe(2);
    expect(a[0]).toMatchObject({ id: "msg-1", actor: "You", cls: "user" });
    expect(a[1]).toMatchObject({ id: "msg-3", actor: "Claude", cls: "" });
  });

  it("truncates preview and appends the live stream", () => {
    const a = railAnchors(
      [{ id: 1, role: "user", content: "x".repeat(200) }],
      "Codex",
      "working on it",
    );
    expect(a[0].preview.endsWith("…")).toBe(true);
    expect(a[0].preview.length).toBeLessThanOrEqual(140);
    expect(a[1]).toMatchObject({ id: "msg-stream", actor: "Codex" });
  });
});
