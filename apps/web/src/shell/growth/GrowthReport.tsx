import { useEffect, useState } from "react";
import type { DimensionScore, GrowthHistoryEntry, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";
import { api } from "../../api";

const SURFACE_LABEL: Record<GrowthHistoryEntry["surface"], string> = {
  project: "项目", course: "课程", chat: "聊天",
};

// Per-dimension diagnostic chip: SOLO_LABELS for a scored level, or a muted
// "insufficient evidence" note for NA — RL-5 (no total/rank/aggregate score
// anywhere, ever): this always renders a label + its evidence, never a
// number or grade.
function LevelChip({ level }: { level: DimensionScore["level"] }) {
  if (level === "NA") {
    return (
      <span style={{ fontSize: 12, fontWeight: 700, color: "#9AA1B0", background: "#F1F2F6", borderRadius: 999, padding: "3px 10px" }}>
        证据不足 · NA
      </span>
    );
  }
  const colors: Record<ScoredLevel, { fg: string; bg: string }> = {
    L1: { fg: "#B0691F", bg: "#FBF0E3" }, L2: { fg: "#8A6D1F", bg: "#FBF6E3" },
    L3: { fg: "#2E7D5B", bg: "#E7F5EF" }, L4: { fg: "#2A5FA8", bg: "#E8F0FB" },
  };
  const c = colors[level];
  return (
    <span style={{ fontSize: 12, fontWeight: 700, color: c.fg, background: c.bg, borderRadius: 999, padding: "3px 10px" }}>
      {SOLO_LABELS[level]}
    </span>
  );
}

function DimensionRow({ dim }: { dim: DimensionScore }) {
  return (
    <div style={{ padding: "14px 0", borderBottom: "1px solid #F3F4F7" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <div style={{ fontSize: 14.5, fontWeight: 700, color: "#1C2333" }}>{dim.name}</div>
        <LevelChip level={dim.level} />
      </div>
      {dim.evidence && (
        <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.6, marginTop: 6 }}>{dim.evidence}</div>
      )}
    </div>
  );
}

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
          <div style={{ borderTop: "1px solid #F0F1F5", paddingTop: 8 }}>
            {entry.report.dimensions.map((d) => <DimensionRow key={d.code} dim={d} />)}
          </div>
          <div style={{ marginTop: 16, fontSize: 14, color: "#2B3346", lineHeight: 1.8 }}>{entry.report.narrative}</div>
        </div>
      )}
    </div>
  );
}

export function GrowthReport() {
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
        if (list.length > 0) setOpenId(`${list[0]!.surface}:${list[0]!.scopeId}`); // newest expanded
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (entries === undefined) {
    return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的成长报告…</div>;
  }

  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
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
      </div>
    </div>
  );
}
