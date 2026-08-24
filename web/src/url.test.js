import { describe, it, expect } from "vitest";
import { parseAppURL, chatPath, overlayPath } from "./url.js";

describe("chatPath", () => {
  it("home has no query", () => {
    expect(chatPath([], null)).toBe("/");
    expect(chatPath(null, null)).toBe("/");
  });
  it("tabs + active tab", () => {
    expect(chatPath(["a", "b"], "b")).toBe("/?tabs=a%2Cb&tab=b");
  });
  it("drops a tab id that is not open", () => {
    expect(chatPath(["a"], "ghost")).toBe("/?tabs=a");
  });
});

describe("overlayPath", () => {
  it("keeps the tab query so close can restore it", () => {
    expect(overlayPath("devices", ["a"], "a")).toBe("/devices?tabs=a&tab=a");
    expect(overlayPath("settings", ["a", "b"], "b")).toBe("/settings?tabs=a%2Cb&tab=b");
    expect(overlayPath("system", [], null)).toBe("/system");
  });
  it("bare overlay when no tabs", () => {
    expect(overlayPath("devices", [], null)).toBe("/devices");
    expect(overlayPath("settings", [], null)).toBe("/settings");
  });
});

describe("parseAppURL", () => {
  const loc = (pathname, search = "") => ({ pathname, search });

  it("reads chat query", () => {
    expect(parseAppURL(loc("/", "?tabs=a,b&tab=b"))).toEqual({
      overlay: null, tabs: ["a", "b"], tab: "b", legacy: null,
    });
  });
  it("reads overlay + preserved tabs", () => {
    expect(parseAppURL(loc("/devices", "?tabs=a&tab=a"))).toEqual({
      overlay: "devices", tabs: ["a"], tab: "a", legacy: null,
    });
    expect(parseAppURL(loc("/settings"))).toEqual({
      overlay: "settings", tabs: [], tab: null, legacy: null,
    });
    expect(parseAppURL(loc("/system", "?tabs=a&tab=a")).overlay).toBe("system");
  });
  it("normalizes legacy /s/<id> and ?s=", () => {
    expect(parseAppURL(loc("/s/xyz")).legacy).toBe("xyz");
    expect(parseAppURL(loc("/", "?s=xyz")).legacy).toBe("xyz");
  });
  it("ignores tab ids that are not in tabs=", () => {
    expect(parseAppURL(loc("/", "?tabs=a&tab=zzz")).tab).toBe("a");
  });
});
