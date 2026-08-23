import { useEffect, useState } from "react";

// Theme: "dark" | "light" | "system", persisted; anti-FOUC applied in
// index.html before the bundle loads.
const KEY = "agentdeck:theme";

const resolved = (pref) =>
  pref === "system"
    ? matchMedia("(prefers-color-scheme: light)").matches
      ? "light"
      : "dark"
    : pref;

export function useTheme() {
  const [pref, setPref] = useState(() => localStorage.getItem(KEY) || "system");

  useEffect(() => {
    const apply = () =>
      document.documentElement.dataset.theme = resolved(pref);
    apply();
    localStorage.setItem(KEY, pref);
    const mq = matchMedia("(prefers-color-scheme: light)");
    const onChange = () => pref === "system" && apply();
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [pref]);

  const current = resolved(pref);
  const toggle = () => setPref(current === "dark" ? "light" : "dark");
  return { pref, current, setPref, toggle };
}
