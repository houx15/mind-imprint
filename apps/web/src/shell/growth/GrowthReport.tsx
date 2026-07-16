import { useEffect, useState } from "react";
import type { Assessment, DimensionScore, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";
import { api } from "../../api";

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
    L1: { fg: "#B0691F", bg: "#FBF0E3" },
    L2: { fg: "#8A6D1F", bg: "#FBF6E3" },
    L3: { fg: "#2E7D5B", bg: "#E7F5EF" },
    L4: { fg: "#2A5FA8", bg: "#E8F0FB" },
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

export function GrowthReport() {
  const [projectId, setProjectId] = useState<string | null>(null);
  const [noProject, setNoProject] = useState(false);
  const [assessment, setAssessment] = useState<Assessment | null | undefined>(undefined);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await api.listProjects();
        if (cancelled) return;
        if (list.length === 0) { setNoProject(true); return; }
        const pid = list[0]!.id;
        setProjectId(pid);
        const a = await api.getAssessment(pid);
        if (cancelled) return;
        setAssessment(a);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  async function handleGenerate() {
    if (!projectId || generating) return;
    setGenerating(true);
    setError(null);
    try {
      const a = await api.generateAssessment(projectId);
      setAssessment(a);
    } catch {
      setError("生成失败，请重试");
    } finally {
      setGenerating(false);
    }
  }

  if (noProject) {
    return (
      <div style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 10, color: "#6B7384" }}>
        <div style={{ fontSize: 15, fontWeight: 700, color: "#3A4256" }}>还没有任务</div>
        <div style={{ fontSize: 13.5, lineHeight: 1.7, maxWidth: 380, textAlign: "center" }}>先在工作室里开始一个任务，成长报告会在这里等你。</div>
      </div>
    );
  }

  if (assessment === undefined) {
    return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的成长报告…</div>;
  }

  const hasReport = assessment !== null;

  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        {/* hero */}
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2a5 5 0 0 0-5 5c0 2 1 3 1 5v2a2 2 0 0 0 2 2h4a2 2 0 0 0 2-2v-2c0-2 1-3 1-5a5 5 0 0 0-5-5z" /><path d="M9 21h6" /></svg>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>成长报告</div>
            <div style={{ fontSize: 22, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>你的思维印记</div>
            <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>按你在这次任务里的真实思考过程，逐维度给出诊断——不是分数，是证据。</div>
          </div>
        </div>

        {error && (
          <div style={{ marginTop: 14, fontSize: 13, color: "#B0432E", background: "#FBEDEA", borderRadius: 10, padding: "10px 14px" }}>{error}</div>
        )}

        {!hasReport ? (
          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "34px 24px", marginTop: 16, display: "flex", flexDirection: "column", alignItems: "center", gap: 14, textAlign: "center" }}>
            <div style={{ fontSize: 14.5, color: "#3A4256", fontWeight: 700 }}>还没有成长报告</div>
            <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.7, maxWidth: 420 }}>完成一些思考后，点下方生成——报告会读取你在工作室里留下的真实过程。</div>
            <button
              type="button"
              onClick={handleGenerate}
              disabled={generating || !projectId}
              style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: generating ? "#9CB6A9" : "#4C9A82", color: "#fff", border: "none", fontSize: 14.5, fontWeight: 700, padding: "13px 22px", borderRadius: 12, cursor: generating ? "default" : "pointer", fontFamily: "inherit" }}
            >
              {generating ? "生成中…" : "生成成长报告"}
            </button>
          </div>
        ) : (
          <>
            <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
              <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 6 }}>逐维度诊断</div>
              {assessment.dimensions.map((d) => <DimensionRow key={d.code} dim={d} />)}
            </div>

            <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
              <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 10 }}>你的思维印记</div>
              <div style={{ fontSize: 14, color: "#2B3346", lineHeight: 1.8 }}>{assessment.narrative}</div>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 18 }}>
              <button
                type="button"
                onClick={handleGenerate}
                disabled={generating}
                style={{ background: "#fff", border: "1px solid #E1E4ED", color: generating ? "#B7BCC8" : "#6B7384", fontSize: 13.5, fontWeight: 700, padding: "11px 18px", borderRadius: 12, cursor: generating ? "default" : "pointer", fontFamily: "inherit" }}
              >
                {generating ? "生成中…" : "重新生成"}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
