import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";

// Shared visual language: white card / #EAECF2 border / 16px radius / 22-24px
// padding, matching apps/web/src/shell/courses/CourseReport.tsx. This
// component renders the canonical assessment object (Spec B): D6 depth axis
// (L1–L4|NA, no subtotal) + A6 autonomy axis (0–5 behavior-count band,
// opportunity-gated) + 6-lens prompt telemetry + interaction evidence +
// guidance, with an optional project-surface superset (officialProjection +
// workAndProcess). RL-5: the two axes never compose into a total score —
// the only percentage anywhere is officialProjection.readiness.score, and its
// note always disclaims composition. 证据地图 is deferred (Spec D).

const CARD_STYLE: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 };
const SECTION_TITLE_STYLE: React.CSSProperties = { fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 };
const SUB_TITLE_STYLE: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: "#1C2333", margin: "14px 0 10px" };

function CheckIcon({ color }: { color: string }) {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
  );
}

function DepthLevelBadge({ level, levelRange }: { level: DualAxisReportT["depthAxis"][number]["level"]; levelRange?: string }) {
  if (level === "NA") {
    return (
      <span style={{ fontSize: 12, fontWeight: 700, color: "#8A92A3", background: "#F1F2F6", padding: "2px 10px", borderRadius: 999, fontStyle: "italic" }}>
        暂无可计入的证据
      </span>
    );
  }
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
      <span style={{ fontSize: 12, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 10px", borderRadius: 999 }}>{level}</span>
      {levelRange ? <span style={{ fontSize: 11.5, color: "#8A92A3" }}>区间 {levelRange}</span> : null}
    </span>
  );
}

function DepthDimCard({ d }: { d: DualAxisReportT["depthAxis"][number] }) {
  return (
    <article style={{ padding: "11px 0", borderBottom: "1px solid #F3F4F7" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6, gap: 10, flexWrap: "wrap" }}>
        <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{d.name}</span>
        <DepthLevelBadge level={d.level} levelRange={d.levelRange} />
      </header>
      <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0 }}>{d.evidence}</p>
      {d.promptEvidence ? (
        <p style={{ fontSize: 12, color: "#6B7384", margin: "4px 0 0" }}>提示词证据：{d.promptEvidence}</p>
      ) : null}
    </article>
  );
}

function AutonomySignalCard({ a }: { a: DualAxisReportT["autonomyAxis"][number] }) {
  const notSupplied = a.opportunity === "not_supplied";
  return (
    <article style={{ padding: "11px 0", borderBottom: "1px solid #F3F4F7" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6, gap: 10, flexWrap: "wrap" }}>
        <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{a.name}</span>
        {notSupplied ? (
          <span style={{ fontSize: 12, fontWeight: 700, color: "#8A92A3", background: "#F1F2F6", padding: "2px 10px", borderRadius: 999, fontStyle: "italic" }}>
            暂无·机会未提供
          </span>
        ) : (
          <span style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 10px", borderRadius: 999 }}>Lv {a.level}</span>
        )}
      </header>
      <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0 }}>{a.evidence}</p>
    </article>
  );
}

function LensStatCard({ s }: { s: DualAxisReportT["promptLens"]["stats"][number] }) {
  return (
    <div data-testid="lens-stat-card" style={{ fontSize: 12.5, color: "#2B3346", background: "#F3F4F7", padding: "10px 14px", borderRadius: 12, flex: "1 1 160px" }}>
      <div style={{ fontSize: 11.5, color: "#8A92A3", fontWeight: 700 }}>{s.label}</div>
      <div style={{ fontSize: 15, fontWeight: 800, marginTop: 2 }}>{s.value}</div>
    </div>
  );
}

function LensCard({ l }: { l: DualAxisReportT["promptLens"]["lenses"][number] }) {
  return (
    <article data-testid="lens-card" style={{ padding: "10px 0", borderBottom: "1px solid #F3F4F7" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4, gap: 10 }}>
        <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{l.name}</span>
        <span style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 10px", borderRadius: 999 }}>Lv {l.level}</span>
      </header>
      <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0 }}>{l.evidence}</p>
    </article>
  );
}

export function DualAxisReport({ report }: { report: DualAxisReportT }) {
  const { depthAxis, autonomyAxis, promptLens, interactionEvidence, guidance, narrative, axiom, officialProjection, workAndProcess } = report;

  return (
    <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
      {/* 总览 */}
      <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
        <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>思维印记 · 双轴成长报告</div>
        <p style={{ fontSize: 13.5, color: "#DCE1F5", marginTop: 14, lineHeight: 1.6 }}>{narrative}</p>
        <p style={{ fontSize: 12, color: "#AEB8E4", marginTop: 10, lineHeight: 1.6, borderTop: "1px solid rgba(255,255,255,.16)", paddingTop: 10 }}>{axiom}</p>
      </div>

      {/* D 轴 · 认知深度 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>认知深度</div>
        {depthAxis.map((d) => <DepthDimCard key={d.code} d={d} />)}
      </div>

      {/* A 轴 · 智识自主 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>智识自主</div>
        {autonomyAxis.map((a) => <AutonomySignalCard key={a.code} a={a} />)}
      </div>

      {/* 提示词透镜 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>提示词透镜</div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap", marginBottom: 14 }}>
          {promptLens.stats.map((s, i) => <LensStatCard key={i} s={s} />)}
        </div>
        {promptLens.lenses.map((l) => <LensCard key={l.code} l={l} />)}
        <p style={{ fontSize: 12, color: "#8A92A3", margin: "12px 0 0", lineHeight: 1.6 }}>{promptLens.note}</p>
      </div>

      {/* 交互证据 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>交互证据</div>
        {interactionEvidence.map((row) => (
          <article key={row.round} style={{ padding: "10px 0", borderBottom: "1px solid #F3F4F7" }}>
            <div style={{ fontSize: 11.5, fontWeight: 700, color: "#D98263", marginBottom: 4 }}>R{row.round}</div>
            <blockquote style={{ fontSize: 13, color: "#2B3346", margin: "0 0 6px", padding: "8px 12px", background: "#F8F9FC", borderLeft: "3px solid #E1E4ED" }}>{row.student}</blockquote>
            <p style={{ fontSize: 12.5, color: "#6B7384", margin: "0 0 4px" }}>{row.aiSummary}</p>
            <span style={{ fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 8px", borderRadius: 999 }}>{row.signal}</span>
          </article>
        ))}
      </div>

      {/* 下一步 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>下一步</div>
        {guidance.nextSteps.map((n, i) => (
          <article key={i} style={{ padding: "8px 0", borderTop: i > 0 ? "1px solid #F3F4F7" : undefined }}>
            <div style={{ fontSize: 13, fontWeight: 700, color: "#1C2333" }}>{n.title}</div>
            <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0" }}>{n.task}</p>
          </article>
        ))}
      </div>

      {/* 官方投影 — project surface only */}
      {officialProjection ? (
        <div style={CARD_STYLE}>
          <div style={SECTION_TITLE_STYLE}>官方投影</div>
          <div style={{ fontSize: 12.5, color: "#8A92A3", marginBottom: 10 }}>对标 {officialProjection.standard.name}</div>

          <div style={SUB_TITLE_STYLE}>档位判定</div>
          {officialProjection.components.map((c, i) => (
            <article key={i} style={{ padding: "8px 0", borderBottom: "1px solid #F3F4F7" }}>
              <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{c.name}</span>
                <span style={{ fontSize: 12, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 10px", borderRadius: 999 }}>{c.judgement}</span>
              </header>
              <p style={{ fontSize: 12.5, color: "#8A92A3", margin: "4px 0 0" }}>{c.reason}</p>
            </article>
          ))}

          <div style={SUB_TITLE_STYLE}>对齐情况</div>
          {officialProjection.alignment.map((row, i) => (
            <article key={i} style={{ padding: "8px 0", borderBottom: "1px solid #F3F4F7" }}>
              <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{row.item}</div>
              <p style={{ fontSize: 12.5, color: "#6B7384", margin: "3px 0 0" }}>要求：{row.standard}</p>
              <p style={{ fontSize: 12.5, color: "#2B3346", margin: "3px 0 0" }}>表现：{row.performance}</p>
              <p style={{ fontSize: 12.5, color: "#8A92A3", margin: "3px 0 0" }}>影响：{row.impact}</p>
            </article>
          ))}

          <div style={{ marginTop: 14, paddingTop: 14, borderTop: "1px solid #F3F4F7", display: "flex", alignItems: "center", gap: 16, flexWrap: "wrap" }}>
            <div style={{ fontSize: 24, fontWeight: 800, color: "#1C2333" }}>{officialProjection.readiness.score} / 100</div>
            <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0, flex: "1 1 240px" }}>{officialProjection.readiness.note}</p>
          </div>
        </div>
      ) : null}

      {/* 作品与过程 — project surface only. 证据地图 is deferred (Spec D). */}
      {workAndProcess ? (
        <div style={CARD_STYLE}>
          <div style={SECTION_TITLE_STYLE}>作品与过程</div>

          <div style={SUB_TITLE_STYLE}>作品片段</div>
          {workAndProcess.workSamples.map((w, i) => (
            <article key={i} style={{ padding: "8px 0", borderBottom: "1px solid #F3F4F7" }}>
              <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{w.title}</div>
              <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0", lineHeight: 1.6 }}>{w.text}</p>
            </article>
          ))}

          <div style={SUB_TITLE_STYLE}>过程材料</div>
          {workAndProcess.processMaterials.map((m, i) => (
            <article key={i} style={{ display: "flex", alignItems: "flex-start", gap: 10, padding: "8px 0", borderBottom: "1px solid #F3F4F7" }}>
              {m.status === "完成" ? <CheckIcon color="#4C9A82" /> : <span style={{ width: 14 }} />}
              <div style={{ flex: 1 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{m.name}</span>
                  <span style={{ fontSize: 11.5, fontWeight: 700, color: "#6B7384", background: "#F1F2F6", padding: "1px 8px", borderRadius: 999 }}>{m.status}</span>
                </div>
                <p style={{ fontSize: 12.5, color: "#8A92A3", margin: "3px 0 0" }}>{m.diagnosis}</p>
              </div>
            </article>
          ))}
        </div>
      ) : null}
    </div>
  );
}
