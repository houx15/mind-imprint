import { useEffect, useState } from "react";
import type { GrowthHistoryEntry } from "@mind-imprint/contracts";
import { api } from "../../api";
import { DualAxisReport } from "../report/DualAxisReport";
import { AbilityModel } from "./AbilityModel";
import { ToolkitCards } from "./ToolkitCards";

const SURFACE_LABEL: Record<GrowthHistoryEntry["surface"], string> = {
  project: "项目", course: "课程", chat: "聊天",
};

function HistoryRow({ entry, open, onToggle }: { entry: GrowthHistoryEntry; open: boolean; onToggle: () => void }) {
  const date = entry.createdAt.slice(0, 10);
  return (
    <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, marginTop: 12, overflow: "hidden" }}>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        style={{ width: "100%", display: "flex", alignItems: "center", gap: 12, padding: "16px 20px", background: "none", border: "none", cursor: "pointer", fontFamily: "inherit", textAlign: "left" }}
      >
        <span style={{ fontSize: 11.5, fontWeight: 800, color: "#5B6474", background: "#F1F2F6", borderRadius: 8, padding: "3px 9px", flex: "none" }}>
          {SURFACE_LABEL[entry.surface]}
        </span>
        <span style={{ flex: 1, minWidth: 0, fontSize: 15, fontWeight: 700, color: "#1C2333" }}>
          {entry.label}{entry.sublabel ? <span style={{ color: "#8A92A3", fontWeight: 600 }}> · {entry.sublabel}</span> : null}
        </span>
        <span style={{ fontSize: 12.5, color: "#9AA1B0", flex: "none" }}>{date}</span>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", transform: open ? "rotate(180deg)" : "none", transition: "transform .15s" }}>
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>
      {open && (
        <div style={{ padding: "0 20px 20px" }}>
          <DualAxisReport report={entry.report} />
        </div>
      )}
    </div>
  );
}

function LearningRecord({ initialScopeId }: { initialScopeId?: string | null }) {
  const [entries, setEntries] = useState<GrowthHistoryEntry[] | undefined>(undefined);
  const [openId, setOpenId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await api.getGrowthHistory();
        if (cancelled) return;
        setEntries(list);
        // Deep-link: if arriving from a project's "查看评估报告", open that
        // entry; otherwise expand the newest.
        const focused = initialScopeId && list.find((e) => e.scopeId === initialScopeId);
        if (focused) setOpenId(`${focused.surface}:${focused.scopeId}`);
        else if (list.length > 0) setOpenId(`${list[0]!.surface}:${list[0]!.scopeId}`);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, [initialScopeId]);

  if (entries === undefined) {
    return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的成长报告…</div>;
  }

  return (
    <>
      <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
        <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2a5 5 0 0 0-5 5c0 2 1 3 1 5v2a2 2 0 0 0 2 2h4a2 2 0 0 0 2-2v-2c0-2 1-3 1-5a5 5 0 0 0-5-5z" /><path d="M9 21h6" /></svg>
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>成长报告</div>
          <div style={{ fontSize: 22, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>你的思维印记</div>
          <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>每完成一个任务、一节课，或在聊天里留下一次思维印记，都会汇集到这里——按真实过程给出的诊断，不是分数。</div>
        </div>
      </div>

      {error && (
        <div style={{ marginTop: 14, fontSize: 13, color: "#B0432E", background: "#FBEDEA", borderRadius: 10, padding: "10px 14px" }}>{error}</div>
      )}

      {entries.length === 0 ? (
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "34px 24px", marginTop: 16, textAlign: "center" }}>
          <div style={{ fontSize: 14.5, color: "#3A4256", fontWeight: 700 }}>还没有报告</div>
          <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.7, maxWidth: 420, margin: "8px auto 0" }}>完成一个任务、一节课，或在聊天里留下一次思维印记，报告会在这里汇集。</div>
        </div>
      ) : (
        entries.map((e) => {
          const id = `${e.surface}:${e.scopeId}`;
          return <HistoryRow key={id} entry={e} open={openId === id} onToggle={() => setOpenId(openId === id ? null : id)} />;
        })
      )}
    </>
  );
}

export function GrowthReport({ initialScopeId }: { initialScopeId?: string | null } = {}) {
  const [tab, setTab] = useState<"learning" | "cards" | "ability">("learning");
  const tabStyle = (active: boolean) => ({
    padding: "8px 16px", borderRadius: 999, border: "none", cursor: "pointer", fontFamily: "inherit",
    fontSize: 13.5, fontWeight: 700,
    background: active ? "#2A3B7A" : "transparent", color: active ? "#fff" : "#6B7384",
  });
  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        <div style={{ display: "flex", gap: 8, marginBottom: 18 }}>
          <button type="button" style={tabStyle(tab === "learning")} onClick={() => setTab("learning")}>学习记录</button>
          <button type="button" style={tabStyle(tab === "cards")} onClick={() => setTab("cards")}>工具卡</button>
          <button type="button" style={tabStyle(tab === "ability")} onClick={() => setTab("ability")}>能力素养</button>
        </div>
        {tab === "learning" ? <LearningRecord initialScopeId={initialScopeId} /> : tab === "cards" ? <ToolkitCards /> : <AbilityModel />}
      </div>
    </div>
  );
}
