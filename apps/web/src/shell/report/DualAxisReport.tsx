import { DUALAXIS_MODEL } from "@mind-imprint/contracts";
import type { DualAxisReport as DualAxisReportT, SoloLevel } from "@mind-imprint/contracts";

// SOLO level vocabulary comes from the DualAxis single-source config
// (DUALAXIS_MODEL.soloLevels), NOT a hand-rolled label map — 单点/多点/关联/
// 抽象扩展 is the canonical structural-level naming, distinct from the OLD CT
// dimension mastery-band vocabulary (萌芽/发展中/熟练/卓越 in SOLO_LABELS).
// Rendered as "code name" (e.g. "L3 关联") everywhere a SOLO level appears.
const SOLO_LEVEL_NAME: Record<string, string> = Object.fromEntries(
  DUALAXIS_MODEL.soloLevels.map((s) => [s.level, s.name])
);
function levelLabel(level: SoloLevel): string {
  if (level === "NA") return "未涉及";
  return `${level} ${SOLO_LEVEL_NAME[level] ?? ""}`.trim();
}

// Shared visual language: white card / #EAECF2 border / 16px radius / 22-24px
// padding, matching apps/web/src/shell/courses/CourseReport.tsx. This
// component is additive-only (Task 8) — no surface wires it in yet (Task 9).

const CARD_STYLE: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 };
const SECTION_TITLE_STYLE: React.CSSProperties = { fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 };
const SUB_TITLE_STYLE: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: "#1C2333", margin: "14px 0 10px" };

function ScoreBadge({ children, tone }: { children: React.ReactNode; tone: "score" | "obs" | "cross" }) {
  const palette: Record<typeof tone, { bg: string; color: string }> = {
    score: { bg: "#FBEEE7", color: "#D98263" },
    obs: { bg: "#EDEFF9", color: "#2A3B7A" },
    cross: { bg: "#E6F2ED", color: "#4C9A82" },
  } as const;
  const { bg, color } = palette[tone];
  return (
    <span style={{ fontSize: 12, fontWeight: 700, color, background: bg, padding: "2px 10px", borderRadius: 999 }}>{children}</span>
  );
}

function DepthDimCard({ d }: { d: DualAxisReportT["depthAxis"]["dims"][number] }) {
  return (
    <article style={{ padding: "11px 0", borderBottom: "1px solid #F3F4F7" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
        <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{d.name}</span>
        <ScoreBadge tone="score">{d.score}</ScoreBadge>
      </header>
      <p style={{ fontSize: 12.5, color: "#8A92A3", margin: "0 0 4px" }}>{d.evidence}</p>
      {d.promptEvidence ? (
        <p style={{ fontSize: 12, color: "#6B7384", margin: 0 }}>提示词证据：{d.promptEvidence}</p>
      ) : (
        <p style={{ fontSize: 12, color: "#B7BCC9", margin: 0, fontStyle: "italic" }}>提示词证据：空</p>
      )}
    </article>
  );
}

export function DualAxisReport({ report }: { report: DualAxisReportT }) {
  const { depthAxis, autonomyAxis, crossAxis, solo, promptLens, timeline, keyEvidence, guidance, narrative, axiom } = report;

  return (
    <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
      {/* 总览 */}
      <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
        <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>思维印记 · 双轴成长报告</div>
        <div style={{ display: "flex", gap: 24, marginTop: 12, flexWrap: "wrap" }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 800, color: "#fff" }}>{depthAxis.subtotal} / 12</div>
            <div style={{ fontSize: 12, color: "#C3CBEC", marginTop: 2 }}>深度小计（单轴，不与自主轴合并）</div>
          </div>
          <div>
            <div style={{ fontSize: 14, fontWeight: 700, color: "#fff" }}>边界设定 ×{promptLens.boundarySettings}</div>
            <div style={{ fontSize: 12, color: "#C3CBEC", marginTop: 2 }}>自主侧不计分，详见提示词透镜</div>
          </div>
        </div>
        <p style={{ fontSize: 13.5, color: "#DCE1F5", marginTop: 14, lineHeight: 1.6 }}>{narrative}</p>
        <p style={{ fontSize: 12, color: "#AEB8E4", marginTop: 10, lineHeight: 1.6, borderTop: "1px solid rgba(255,255,255,.16)", paddingTop: 10 }}>{axiom}</p>
      </div>

      {/* 双轴读数 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>双轴读数</div>

        <div style={SUB_TITLE_STYLE}>第一轴 · 认知深度</div>
        {depthAxis.dims.map((d) => <DepthDimCard key={d.code} d={d} />)}

        <div style={SUB_TITLE_STYLE}>第二轴 · 智识自主（不计分）</div>
        <article style={{ padding: "11px 0", borderBottom: "1px solid #F3F4F7" }}>
          <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{autonomyAxis.name}</span>
            <ScoreBadge tone="obs">观察</ScoreBadge>
          </header>
          <p style={{ fontSize: 12.5, color: "#8A92A3", margin: "0 0 6px" }}>{autonomyAxis.observation}</p>
          <p style={{ fontSize: 12, color: "#6B7384", margin: "0 0 3px" }}>能力锚定：{autonomyAxis.anchoredSignals.join("、") || "无"}</p>
          <p style={{ fontSize: 12, color: "#6B7384", margin: "0 0 3px" }}>引导后：{autonomyAxis.promptedSignals.join("、") || "无"}</p>
          <p style={{ fontSize: 12, color: "#6B7384", margin: 0 }}>对手邀请：{autonomyAxis.adversaryInvites} 次</p>
        </article>

        <div style={SUB_TITLE_STYLE}>跨轴 · 元认知</div>
        <article style={{ padding: "11px 0" }}>
          <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6 }}>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{crossAxis.name}</span>
            <ScoreBadge tone="cross">跨轴</ScoreBadge>
          </header>
          <p style={{ fontSize: 12.5, color: "#8A92A3", margin: "0 0 4px" }}>深度面 {levelLabel(crossAxis.depthLevel)}／自主面 {crossAxis.initiative}</p>
          <p style={{ fontSize: 13, color: "#2B3346", margin: 0, lineHeight: 1.6 }}>{crossAxis.prose}</p>
        </article>
      </div>

      {/* SOLO 判层 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>SOLO 判层</div>
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
            <thead>
              <tr style={{ textAlign: "left", color: "#8A92A3", fontSize: 12, fontWeight: 700 }}>
                <th style={{ padding: "6px 8px" }}>轮次</th>
                <th style={{ padding: "6px 8px" }}>回应</th>
                <th style={{ padding: "6px 8px" }}>判层</th>
                <th style={{ padding: "6px 8px" }}>判据</th>
                <th style={{ padding: "6px 8px" }}>发起方</th>
              </tr>
            </thead>
            <tbody>
              {solo.map((s) => (
                <tr key={s.round} style={{ borderTop: "1px solid #F3F4F7" }}>
                  <td style={{ padding: "8px" }}>R{s.round}</td>
                  <td style={{ padding: "8px", color: "#2B3346" }}>{s.excerpt}</td>
                  <td style={{ padding: "8px" }}><ScoreBadge tone="score">{levelLabel(s.level)}</ScoreBadge></td>
                  <td style={{ padding: "8px", color: "#6B7384" }}>{s.rationale}</td>
                  <td style={{ padding: "8px", color: "#6B7384" }}>{s.initiative}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* 提示词透镜 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>提示词透镜</div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap", marginBottom: 14 }}>
          <span style={{ fontSize: 12.5, color: "#2B3346", background: "#F3F4F7", padding: "6px 12px", borderRadius: 10 }}>主动指令轮 {promptLens.directiveRounds} / {promptLens.totalRounds}</span>
          <span style={{ fontSize: 12.5, color: "#2B3346", background: "#F3F4F7", padding: "6px 12px", borderRadius: 10 }}>边界设定 {promptLens.boundarySettings} 次</span>
          <span style={{ fontSize: 12.5, color: "#2B3346", background: "#F3F4F7", padding: "6px 12px", borderRadius: 10 }}>对手邀请 {promptLens.adversaryInvites} 次</span>
        </div>
        {promptLens.questions.map((q, i) => (
          <div key={i} style={{ marginBottom: 10 }}>
            <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{q.title}</div>
            <p style={{ fontSize: 12.5, color: "#6B7384", margin: "3px 0 0" }}>{q.body}</p>
          </div>
        ))}
        <div style={{ marginTop: 14, paddingTop: 14, borderTop: "1px solid #F3F4F7" }}>
          <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>本次最佳提示词（R{promptLens.bestPrompt.round}）</div>
          <blockquote style={{ fontSize: 13, color: "#2B3346", background: "#F8F9FC", borderLeft: "3px solid #4C9A82", margin: "6px 0", padding: "8px 12px" }}>{promptLens.bestPrompt.quote}</blockquote>
          <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0 }}>{promptLens.bestPrompt.annotation}</p>
        </div>
        <div style={{ marginTop: 14, paddingTop: 14, borderTop: "1px solid #F3F4F7" }}>
          <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>带走的一条升级提示</div>
          <blockquote style={{ fontSize: 13, color: "#2B3346", background: "#F8F9FC", borderLeft: "3px solid #D98263", margin: "6px 0", padding: "8px 12px" }}>{promptLens.takeaway.quote}</blockquote>
          <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0 }}>{promptLens.takeaway.annotation}</p>
        </div>
      </div>

      {/* 交互证据 timeline */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>交互证据</div>
        {timeline.map((t) => (
          <details key={t.round} style={{ padding: "10px 0", borderBottom: "1px solid #F3F4F7" }}>
            <summary style={{ cursor: "pointer", fontSize: 13.5, color: "#1C2333", display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
              <span style={{ fontWeight: 700 }}>R{t.round} · {t.task}</span>
              <span style={{ fontSize: 11, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 8px", borderRadius: 999 }}>{t.pTag}</span>
              {t.dimTags.map((dt) => (
                <span key={dt} style={{ fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 8px", borderRadius: 999 }}>{dt}</span>
              ))}
            </summary>
            <blockquote style={{ fontSize: 12.5, color: "#6B7384", margin: "8px 0 0", padding: "8px 12px", background: "#F8F9FC", borderLeft: "3px solid #E1E4ED" }}>{t.prompt}</blockquote>
          </details>
        ))}
      </div>

      {/* 关键原话 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>关键思维证据</div>
        {keyEvidence.map((k, i) => (
          <article key={i} style={{ padding: "10px 0", borderBottom: "1px solid #F3F4F7" }}>
            <div style={{ fontSize: 13, fontWeight: 700, color: "#1C2333" }}>{k.label}</div>
            <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0", lineHeight: 1.6 }}>{k.quote}</p>
          </article>
        ))}
      </div>

      {/* 建议 */}
      <div style={CARD_STYLE}>
        <div style={SECTION_TITLE_STYLE}>参与度与下一步</div>
        <article style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: "#4C9A82" }}>A 类 · 自发完成</div>
          <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0" }}>{guidance.anchored}</p>
        </article>
        <article style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: "#2A3B7A" }}>引导后完成</div>
          <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0" }}>{guidance.prompted}</p>
        </article>
        <article style={{ padding: "8px 0" }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: "#D98263" }}>主要风险</div>
          <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0" }}>{guidance.risk}</p>
        </article>
        {guidance.nextSteps.map((n, i) => (
          <article key={i} style={{ padding: "8px 0", borderTop: "1px solid #F3F4F7" }}>
            <div style={{ fontSize: 13, fontWeight: 700, color: "#1C2333" }}>{n.title}</div>
            <p style={{ fontSize: 13, color: "#2B3346", margin: "4px 0 0" }}>{n.body}</p>
          </article>
        ))}
      </div>
    </div>
  );
}
