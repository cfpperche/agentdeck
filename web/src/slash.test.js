import { describe, it, expect } from "vitest";
import { filterSlash, parseAtToken, insertMention } from "./slash.js";

describe("filterSlash", () => {
  it("only when leading /", () => {
    expect(filterSlash("hello").length).toBe(0);
    expect(filterSlash("/").length).toBeGreaterThan(3);
  });
  it("filters by prefix", () => {
    const hits = filterSlash("/set");
    expect(hits[0].id).toBe("settings");
  });
});

describe("parseAtToken", () => {
  it("finds @ at start and after space", () => {
    expect(parseAtToken("@fi", 3)).toEqual({ start: 0, end: 3, query: "fi" });
    expect(parseAtToken("see @pkg", 8)).toEqual({ start: 4, end: 8, query: "pkg" });
  });
  it("ignores email-like and mid-word", () => {
    expect(parseAtToken("a@b", 3)).toBeNull();
    expect(parseAtToken("hello", 5)).toBeNull();
  });
});

describe("insertMention", () => {
  it("replaces the token and leaves a trailing space", () => {
    const next = insertMention("see @pkg", { start: 4, end: 8, query: "pkg" }, "src/pkg.go");
    expect(next.text).toBe("see @src/pkg.go ");
    expect(next.caret).toBe("see @src/pkg.go ".length);
  });
});
