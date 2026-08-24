import { useEffect, useState } from "react";

export function DevicesPanel() {
  const [list, setList] = useState([]);
  useEffect(() => {
    let on = true;
    const load = () => fetch("/api/devices").then((r) => r.json()).then((d) => { if (on) setList(d || []); }).catch(() => {});
    load();
    const t = setInterval(load, 4000);
    return () => { on = false; clearInterval(t); };
  }, []);
  const host = list.filter((d) => d.host);
  const remote = list.filter((d) => !d.host);
  return (
    <div class="flex-1 overflow-y-auto">
      <div class="max-w-xl mx-auto px-6 py-10">
        <h1 class="text-lg font-semibold mb-6" style={{ color: "var(--text-1)" }}>Devices</h1>
        <h2 class="text-[13px] font-semibold mb-2" style={{ color: "var(--text-2)" }}>This machine</h2>
        <DeviceList items={host} empty="This browser is not pinging yet." />
        <h2 class="text-[13px] font-semibold mt-8 mb-2" style={{ color: "var(--text-2)" }}>Other devices</h2>
        <DeviceList items={remote} empty="No phones or other machines connected." />
      </div>
    </div>
  );
}

function DeviceList({ items, empty }) {
  if (!items.length) return <p class="text-[13px] mb-4" style={{ color: "var(--text-3)" }}>{empty}</p>;
  return (
    <ul class="mb-4">
      {items.map((d) => (
        <li key={d.id} class="flex items-center gap-2.5 py-2 text-[13px]" style={{ borderBottom: "1px solid var(--border-soft)", opacity: d.online ? 1 : 0.55 }}>
          <span class="h-1.5 w-1.5 rounded-full shrink-0" style={{ background: d.online ? "var(--ok)" : "var(--text-3)" }} />
          <span class="font-medium" style={{ color: "var(--text-1)" }}>{d.name}</span>
          <span class="font-mono text-[11.5px] ml-auto" style={{ color: "var(--text-3)" }}>{d.ip}</span>
          <span class="font-mono text-[11px]" style={{ color: "var(--text-3)" }}>{d.online ? "online" : "away"}</span>
        </li>
      ))}
    </ul>
  );
}
