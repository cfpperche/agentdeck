import { describe, it, expect } from "vitest";
import { statusSegments } from "./statusbar.js";

describe("statusSegments", () => {
  it("hides empty segments", () => {
    expect(statusSegments({})).toEqual([]);
    expect(statusSegments({ cwd: "~/agentdeck" })[0].text).toBe("~/agentdeck");
  });

  it("git worktree dirty and extras", () => {
    const parts = statusSegments({
      cwd: "~/w",
      branch: "main",
      worktree: "hotfix",
      dirty: true,
      contextWindow: 200000,
      contextPercent: 81,
      autoCompact: true,
      input: 12000,
      output: 3000,
      cacheRead: 8000,
      cacheHit: 40,
      cost: 0.12,
      sessionName: "refactor-auth",
    });
    expect(parts.find((p) => p.key === "git").text).toBe("main@hotfix*");
    const ctx = parts.find((p) => p.key === "ctx");
    expect(ctx.tone).toBe("warn");
    expect(ctx.text).toContain("(auto)");
    expect(parts.find((p) => p.key === "io").text).toBe("↑12k ↓3k");
    expect(parts.find((p) => p.key === "ch").text).toBe("CH40%");
    expect(parts.find((p) => p.key === "cost").text).toBe("$0.12");
    expect(parts.find((p) => p.key === "name").text).toBe("refactor-auth");
  });
});
