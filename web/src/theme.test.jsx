import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// theme.js: persisted preference, system resolution, live OS tracking.

const KEY = "agentdeck:theme";

function setMatchMedia(light) {
  const listeners = new Set();
  let current = light;
  global.matchMedia = vi.fn().mockImplementation((q) => ({
    get matches() { return q.includes("prefers-color-scheme") ? current : false; },
    addEventListener: (_e, cb) => listeners.add(cb),
    removeEventListener: (_e, cb) => listeners.delete(cb),
  }));
  return () => { current = true; listeners.forEach((cb) => cb({ matches: true })); };
}

describe("useTheme", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.dataset.theme = "";
  });

  it("dark default when OS prefers light? no — system resolves to light", async () => {
    setMatchMedia(true);
    const { useTheme } = await import("./theme.js");
    let out;
    function Probe() { out = useTheme(); return null; }
    render(<Probe />);
    expect(out.current).toBe("light");
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("toggle flips and persists", async () => {
    setMatchMedia(true);
    const { useTheme } = await import("./theme.js");
    let out;
    function Probe() { out = useTheme(); return null; }
    render(<Probe />);
    await act(async () => out.toggle());
    expect(out.current).toBe("dark");
    expect(localStorage.getItem(KEY)).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("setPref('system') follows OS changes", async () => {
    const fireOS = setMatchMedia(false);
    const { useTheme } = await import("./theme.js");
    let out;
    function Probe() { out = useTheme(); return null; }
    render(<Probe />);
    expect(out.current).toBe("dark");
    await act(async () => out.setPref("system"));
    // OS flips to light (listener fired with matches: true)
    await act(async () => fireOS());
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("restores a saved preference", async () => {
    localStorage.setItem(KEY, "dark");
    setMatchMedia(true);
    const { useTheme } = await import("./theme.js");
    let out;
    function Probe() { out = useTheme(); return null; }
    render(<Probe />);
    expect(out.pref).toBe("dark");
    expect(out.current).toBe("dark");
  });
});
