import { useState } from "react";
import { Harness } from "./Harness";
import { StorePanel } from "./StorePanel";

type Tab = "cards" | "store";

export function DevApp() {
  const [tab, setTab] = useState<Tab>("cards");
  const tabCls = (t: Tab) =>
    `rounded-full px-3 py-1.5 text-[13px] font-semibold ${tab === t ? "bg-mk-primary text-white" : "bg-white text-mk-muted-2"}`;
  return (
    <div className="min-h-screen bg-mk-bg font-sans text-mk-ink">
      <div className="flex gap-2 border-b border-mk-border bg-white px-6 py-3">
        <button type="button" onClick={() => setTab("cards")} className={tabCls("cards")}>卡片</button>
        <button type="button" onClick={() => setTab("store")} className={tabCls("store")}>存储</button>
      </div>
      {tab === "cards" && <Harness />}
      {tab === "store" && <StorePanel />}
    </div>
  );
}
