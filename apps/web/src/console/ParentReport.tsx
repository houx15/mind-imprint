import { useEffect, useState } from "react";
import type { ParentReport as ParentReportDTO } from "@mind-imprint/contracts";
import { api, ApiError } from "../api";
import {
  PRINCIPLES, D_LEVELS, D_DIMS, A_STATES, A_SIGNALS, HOW_LIST, SUPPLY, GLOSSARY, FOOTER,
  D_BADGE_COLOR, A_STATE_COLOR,
} from "./parentReportContent";

// Full-screen printable overlay reproducing the binding design's 项目模式
// layout (docs/design/teacher end/project/家长报告.dc.html). Static
// front-matter comes from parentReportContent.ts; the per-student rows/prose
// come from the server DTO — badge/state colors are looked up ONLY from the
// label maps, never recomputed client-side, and no A-axis number is ever
// rendered (RL-5).

const CARD: React.CSSProperties = { border: "1px solid #E4E7EF", borderRadius: 14, padding: "16px 18px" };
const H2: React.CSSProperties = { fontSize: 19, fontWeight: 800, color: "#1C2333", margin: "30px 0 10px" };

const D_BADGE_FALLBACK = { color: "#8A92A3", bg: "#F1F2F6" };
const A_STATE_FALLBACK = { color: "#C4574D", bg: "#F7E6E4" };

function DBadge({ badge }: { badge: string }) {
  const c = D_BADGE_COLOR[badge] ?? D_BADGE_FALLBACK;
  return (
    <span style={{ flex: "none", padding: "4px 12px", borderRadius: 20, background: c.bg, color: c.color, fontSize: 12.5, fontWeight: 800 }}>
      {badge}
    </span>
  );
}

function AState({ state }: { state: string }) {
  const c = A_STATE_COLOR[state] ?? A_STATE_FALLBACK;
  return (
    <span style={{ flex: "none", padding: "4px 12px", borderRadius: 20, background: c.bg, color: c.color, fontSize: 12, fontWeight: 800 }}>
      {state}
    </span>
  );
}

export function ParentReport({
  classId,
  studentId,
  surface,
  scopeId,
  studentName,
  onClose,
}: {
  classId: string;
  studentId: string;
  surface: string;
  scopeId: string;
  studentName?: string;
  onClose: () => void;
}) {
  const [data, setData] = useState<ParentReportDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  useEffect(() => {
    setError(null);
    api.getParentReport(classId, studentId, surface, scopeId).then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }, [classId, studentId, surface, scopeId]);

  async function handleGenerate() {
    setGenerating(true);
    try {
      const next = await api.generateParentReportProse(classId, studentId, surface, scopeId);
      setData(next);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "生成失败");
    } finally {
      setGenerating(false);
    }
  }

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 100, background: "#F3F4F8", overflowY: "auto" }}>
      <style>{"@media print { .no-print { display:none !important; } }"}</style>

      {/* control bar (screen only) */}
      <div className="no-print" style={{ position: "sticky", top: 0, zIndex: 10, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, background: "#fff", borderBottom: "1px solid #E4E7EF", padding: "12px 24px" }}>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>家长版报告{studentName ? ` · ${studentName}` : ""}</div>
        <div style={{ display: "flex", gap: 10 }}>
          <button
            onClick={() => window.print()}
            style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "#2A3B7A", color: "#fff", fontSize: 13, fontWeight: 700, padding: "9px 16px", borderRadius: 10, border: "none", cursor: "pointer", fontFamily: "inherit" }}
          >
            下载 PDF
          </button>
          <button
            onClick={onClose}
            style={{ background: "#F1F2F6", color: "#4A5060", fontSize: 13, fontWeight: 700, padding: "9px 16px", borderRadius: 10, border: "none", cursor: "pointer", fontFamily: "inherit" }}
          >
            关闭
          </button>
        </div>
      </div>

      {error && (
        <div style={{ margin: "20px 24px", color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error}</div>
      )}

      {data && (
        <div style={{ maxWidth: 780, margin: "0 auto", padding: "26px 30px 80px" }}>
          {/* cover */}
          <div style={{ border: "1px solid #E4E7EF", borderRadius: 16, padding: "26px 28px", background: "linear-gradient(180deg,#F7F8FC 0%,#FFFFFF 70%)" }}>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#2A3B7A", letterSpacing: ".02em" }}>思维印记 · 学生能力成长报告</div>
            <div style={{ marginTop: 20, fontSize: 30, fontWeight: 900, color: "#1C2333", lineHeight: 1.15 }}>{data.cover.name} 的能力成长报告</div>
            {data.cover.warmLine ? (
              <div style={{ marginTop: 10, fontSize: 14, color: "#5A6377", lineHeight: 1.7 }}>{data.cover.warmLine}</div>
            ) : null}
            <div style={{ marginTop: 18, display: "flex", flexWrap: "wrap", gap: 8 }}>
              <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, background: "#EDEFF9", color: "#2A3B7A", fontSize: 12.5, fontWeight: 700 }}>{data.cover.typeLabel}</span>
              <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, border: "1px solid #E4E7EF", color: "#5A6377", fontSize: 12.5 }}>{data.cover.subject}</span>
              <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, border: "1px solid #E4E7EF", color: "#5A6377", fontSize: 12.5 }}>{data.cover.klass}</span>
              <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, border: "1px solid #E4E7EF", color: "#5A6377", fontSize: 12.5 }}>生成日期 {data.cover.dateStr}</span>
            </div>
          </div>

          {/* 这份报告怎么读 */}
          <h2 style={H2}>这份报告怎么读</h2>
          <p style={{ fontSize: 15, lineHeight: 1.9, color: "#3A4256", margin: "0 0 14px" }}>思维印记陪伴孩子做研究和思考，记录的是过程，不是一张成绩单。在看具体内容之前，有四条我们始终遵守的原则，也希望和您达成共识。</p>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            {PRINCIPLES.map((p) => (
              <div key={p.title} style={CARD}>
                <div style={{ fontSize: 14.5, fontWeight: 800, color: "#2A3B7A" }}>{p.title}</div>
                <div style={{ marginTop: 8, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{p.text}</div>
              </div>
            ))}
          </div>

          {/* 两条轴 */}
          <h2 style={H2}>我们怎么看"能力"：两条轴</h2>
          <p style={{ fontSize: 15, lineHeight: 1.9, color: "#3A4256", margin: "0 0 16px" }}>我们用两条彼此独立的线索来看孩子。它们回答的是两个不同的问题，所以不会被合成一个总分。</p>

          <div style={CARD}>
            <div style={{ fontSize: 16, fontWeight: 800, color: "#2A3B7A" }}>D 轴 · 认知深度</div>
            <div style={{ marginTop: 6, fontSize: 14, lineHeight: 1.8, color: "#3A4256" }}>回答"想得有多深"。我们把它分成四个台阶，逐条对孩子的表现判断处在哪个台阶，再看他最稳定停留在哪里。</div>
            <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 8 }}>
              {D_LEVELS.map((d) => (
                // The label span deliberately includes the trailing "：" so
                // this static explainer text never exact-text-collides with
                // a dRow's D_BADGE_COLOR badge (dRows render the SAME four
                // labels verbatim, unadorned, per the server DTO).
                <div key={d.label} style={{ display: "flex", gap: 12, alignItems: "flex-start", borderLeft: `4px solid ${d.color}`, background: "#FAFBFD", borderRadius: "0 10px 10px 0", padding: "10px 14px" }}>
                  <span style={{ flex: "none", minWidth: 44, fontSize: 13.5, fontWeight: 800, color: d.color }}>{d.label}：</span>
                  <span style={{ fontSize: 13.5, lineHeight: 1.7, color: "#3A4256" }}>{d.desc}</span>
                </div>
              ))}
            </div>
            <div style={{ marginTop: 14, fontSize: 13.5, fontWeight: 700, color: "#4A5060" }}>D 轴看的六个维度：</div>
            <div style={{ marginTop: 8, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              {D_DIMS.map((x) => (
                <div key={x.name} style={{ fontSize: 13, lineHeight: 1.65, color: "#3A4256" }}>
                  <b style={{ color: "#20263A" }}>{x.name}</b> — {x.meaning}
                </div>
              ))}
            </div>
          </div>

          <div style={{ ...CARD, marginTop: 14 }}>
            <div style={{ fontSize: 16, fontWeight: 800, color: "#3E8A6E" }}>A 轴 · 智识自主</div>
            <div style={{ marginTop: 6, fontSize: 14, lineHeight: 1.8, color: "#3A4256" }}>
              回答"有多愿意自己想"。这条轴<b>只记录信号、不打分</b>——所以整份报告里，您不会看到 A 轴的任何数字。我们记录"主动信号"出现的次数和方式，用三种状态来呈现。
            </div>
            <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 8 }}>
              {A_STATES.map((s) => (
                // Same "：" disambiguation as D_LEVELS above — aRows render
                // these SAME three state labels verbatim via A_STATE_COLOR.
                <div key={s.label} style={{ display: "flex", gap: 12, alignItems: "flex-start", borderLeft: `4px solid ${s.color}`, background: "#FAFBFD", borderRadius: "0 10px 10px 0", padding: "10px 14px" }}>
                  <span style={{ flex: "none", minWidth: 96, fontSize: 13, fontWeight: 800, color: s.color }}>{s.label}：</span>
                  <span style={{ fontSize: 13.5, lineHeight: 1.7, color: "#3A4256" }}>{s.desc}</span>
                </div>
              ))}
            </div>
            <div style={{ marginTop: 14, fontSize: 13.5, fontWeight: 700, color: "#4A5060" }}>A 轴看的六个信号：</div>
            <div style={{ marginTop: 8, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              {A_SIGNALS.map((x) => (
                <div key={x.name} style={{ fontSize: 13, lineHeight: 1.65, color: "#3A4256" }}>
                  <b style={{ color: "#20263A" }}>{x.name}</b> — {x.meaning}
                </div>
              ))}
            </div>
          </div>

          {/* 判断是怎么得出来的 */}
          <h2 style={H2}>判断是怎么得出来的</h2>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {HOW_LIST.map((h, i) => (
              <div key={h.title} style={{ display: "flex", gap: 12, alignItems: "flex-start" }}>
                <span style={{ flex: "none", width: 22, height: 22, borderRadius: "50%", background: "#EDEFF9", color: "#2A3B7A", display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800, fontSize: 12, marginTop: 2 }}>{i + 1}</span>
                <div style={{ fontSize: 14, lineHeight: 1.8, color: "#3A4256" }}>
                  <b style={{ color: "#20263A" }}>{h.title}</b>{h.text}
                </div>
              </div>
            ))}
          </div>

          {/* {name} 这次的表现 */}
          <h2 style={H2}>{data.cover.name} 这次的表现</h2>
          {data.glance ? (
            <div style={{ border: "1px solid #E4E7EF", background: "#F7F8FB", borderRadius: 14, padding: "16px 18px" }}>
              <div style={{ fontSize: 13, fontWeight: 800, color: "#2A3B7A" }}>一眼判断</div>
              <div style={{ marginTop: 7, fontSize: 14.5, lineHeight: 1.75, color: "#20263A", fontWeight: 600 }}>{data.glance}</div>
            </div>
          ) : null}

          <div style={{ fontSize: 16, fontWeight: 800, color: "#2A3B7A", margin: "22px 0 4px" }}>D 轴 · 认知深度（逐维）</div>
          {data.dOverview ? <div style={{ fontSize: 13, color: "#7A8296", marginBottom: 12 }}>{data.dOverview}</div> : null}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {data.dRows.map((r) => (
              <div key={r.code} style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 800, color: "#1C2333" }}>{r.name}</div>
                  <DBadge badge={r.badge} />
                </div>
                {r.reading ? <div style={{ marginTop: 9, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{r.reading}</div> : null}
              </div>
            ))}
          </div>

          <div style={{ fontSize: 16, fontWeight: 800, color: "#3E8A6E", margin: "24px 0 4px" }}>A 轴 · 智识自主（信号，不打分）</div>
          {data.aOverview ? <div style={{ fontSize: 13, color: "#7A8296", marginBottom: 12 }}>{data.aOverview}</div> : null}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {data.aRows.map((r) => (
              <div key={r.code} style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 800, color: "#1C2333" }}>{r.name}</div>
                  <AState state={r.state} />
                </div>
                {r.reading ? <div style={{ marginTop: 9, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{r.reading}</div> : null}
              </div>
            ))}
          </div>

          {data.opportunity ? (
            <div style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px", marginTop: 16 }}>
              <div style={{ fontSize: 13, fontWeight: 800, color: "#B0863A" }}>关于"机会"与真实性</div>
              <div style={{ marginTop: 7, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{data.opportunity}</div>
            </div>
          ) : null}

          {data.prose === null && (
            <div className="no-print" style={{ marginTop: 16 }}>
              <button
                onClick={handleGenerate}
                disabled={generating}
                style={{ background: "#2A3B7A", color: "#fff", fontSize: 13.5, fontWeight: 700, padding: "11px 18px", borderRadius: 11, border: "none", cursor: generating ? "default" : "pointer", opacity: generating ? 0.7 : 1, fontFamily: "inherit" }}
              >
                {generating ? "生成中…" : "生成家长版正文"}
              </button>
            </div>
          )}

          {/* 下一步 */}
          <h2 style={H2}>下一步</h2>
          {data.advice.length > 0 && (
            <>
              <div style={{ fontSize: 14.5, fontWeight: 800, color: "#2A3B7A", margin: "6px 0 10px" }}>在家可以怎么帮</div>
              <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                {data.advice.map((a, i) => (
                  <div key={a.title} style={{ display: "flex", gap: 12, alignItems: "flex-start", border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 16px" }}>
                    <span style={{ flex: "none", width: 26, height: 26, borderRadius: 8, background: "#EDEFF9", color: "#2A3B7A", display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 800, fontSize: 13 }}>{i + 1}</span>
                    <div>
                      <div style={{ fontSize: 14.5, fontWeight: 800, color: "#1C2333" }}>{a.title}</div>
                      <div style={{ marginTop: 5, fontSize: 14, lineHeight: 1.75, color: "#3A4256" }}>{a.text}</div>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
          <div style={{ fontSize: 14.5, fontWeight: 800, color: "#3E8A6E", margin: "18px 0 10px" }}>平台接下来会做</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {SUPPLY.map((s) => (
              <div key={s} style={{ display: "flex", gap: 10, alignItems: "flex-start" }}>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#3E8A6E" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 2 }}><path d="M20 6 9 17l-5-5" /></svg>
                <span style={{ fontSize: 14, lineHeight: 1.75, color: "#3A4256" }}>{s}</span>
              </div>
            ))}
          </div>

          {/* 名词解释 */}
          <h2 style={H2}>名词解释</h2>
          <div style={{ display: "flex", flexDirection: "column", gap: 9 }}>
            {GLOSSARY.map((g) => (
              <div key={g.term} style={{ display: "flex", gap: 12, alignItems: "flex-start" }}>
                <span style={{ flex: "none", minWidth: 118, fontSize: 13.5, fontWeight: 800, color: "#2A3B7A" }}>{g.term}</span>
                <span style={{ fontSize: 13.5, lineHeight: 1.7, color: "#3A4256" }}>{g.def}</span>
              </div>
            ))}
          </div>

          <div style={{ marginTop: 28, paddingTop: 14, borderTop: "1px solid #E4E7EF", fontSize: 12, color: "#9198A8", lineHeight: 1.7 }}>{FOOTER}</div>
        </div>
      )}
    </div>
  );
}
