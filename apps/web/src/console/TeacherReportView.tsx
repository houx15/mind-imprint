import { useEffect, useRef, useState } from "react";
import type { ApiClient, TeacherReport } from "../api";
import { ApiError } from "../api";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";
import { badgeColor } from "./badgeColor";
import { EvidenceMap } from "./EvidenceMap";

type Client = Pick<ApiClient, "getStudentReport">;

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "对话" };

const CARD_STYLE: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: 16 };
const H2_STYLE: React.CSSProperties = { fontSize: 20, fontWeight: 800, color: "#1C2333", margin: "38px 0 16px" };
const CAPTION_STYLE: React.CSSProperties = { fontSize: 12.5, color: "#8A92A3", lineHeight: 1.7, marginBottom: 16 };

function DepthBadge({ level, levelRange }: { level: DualAxisReportT["depthAxis"][number]["level"]; levelRange?: string }) {
  if (level === "NA") {
    return (
      <span style={{ fontSize: 12, fontWeight: 700, color: "#8A92A3", background: "#F1F2F6", padding: "2px 10px", borderRadius: 8, fontStyle: "italic" }}>
        暂无可计入的证据
      </span>
    );
  }
  const c = badgeColor(level);
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
      <span style={{ fontSize: 13, fontWeight: 800, color: c.fg, background: c.bg, padding: "2px 10px", borderRadius: 8 }}>{level}</span>
      {levelRange ? <span style={{ fontSize: 11.5, color: "#8A92A3" }}>区间 {levelRange}</span> : null}
    </span>
  );
}

function DimCard({ name, badge, evidence }: { name: string; badge: React.ReactNode; evidence: string }) {
  return (
    <div style={{ ...CARD_STYLE, borderTop: "4px solid #EAECF2" }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
        <div style={{ fontSize: 13.5, fontWeight: 800, color: "#1C2333", lineHeight: 1.35 }}>{name}</div>
        {badge}
      </div>
      <div style={{ marginTop: 10, fontSize: 12.5, color: "#4A5060", lineHeight: 1.7 }}>{evidence}</div>
    </div>
  );
}

export function TeacherReportView({
  client,
  classId,
  userId,
  surface,
  scopeId,
  studentName,
  onBack,
}: {
  client: Client;
  classId: string;
  userId: string;
  surface: string;
  scopeId: string;
  studentName?: string;
  onBack: () => void;
}) {
  const [data, setData] = useState<TeacherReport | null>(null);
  const [error, setError] = useState<string | null>(null);

  const overviewRef = useRef<HTMLDivElement | null>(null);
  const officialRef = useRef<HTMLDivElement | null>(null);
  const dualRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<HTMLDivElement | null>(null);
  const timelineRef = useRef<HTMLDivElement | null>(null);
  const promptRef = useRef<HTMLDivElement | null>(null);
  const workRef = useRef<HTMLDivElement | null>(null);
  const nextRef = useRef<HTMLDivElement | null>(null);

  function load() {
    setError(null);
    client.getStudentReport(classId, userId, surface, scopeId).then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client, classId, userId, surface, scopeId]);

  if (error) {
    return (
      <div style={{ flex: 1, padding: 40 }}>
        <button onClick={onBack} style={{ background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 }}>← 全部学生</button>
        <div style={{ marginTop: 20, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
      </div>
    );
  }
  if (!data) {
    return <div style={{ flex: 1 }} />;
  }

  const { report, context } = data;
  const { depthAxis, autonomyAxis, promptLens, interactionEvidence, guidance, narrative, axiom, officialProjection, workAndProcess } = report;

  const scrollTo = (ref: React.RefObject<HTMLDivElement | null>) => {
    ref.current?.scrollIntoView?.({ behavior: "smooth", block: "start" });
  };

  const title = context.projectTitle ?? `${SURFACE_LABEL[surface] ?? surface}报告`;
  const chips = [
    SURFACE_LABEL[surface] ?? surface,
    officialProjection ? officialProjection.standard.name : null,
    report.generatedAt ? report.generatedAt.slice(0, 10) : null,
  ].filter((c): c is string => !!c);

  const catalog: { key: string; label: string; ref: React.RefObject<HTMLDivElement | null> }[] = [
    { key: "overview", label: "总览", ref: overviewRef },
    ...(officialProjection ? [{ key: "official", label: "官方作品投影", ref: officialRef }] : []),
    { key: "dual", label: "D / A 双轴读数", ref: dualRef },
    { key: "map", label: "证据地图", ref: mapRef },
    { key: "timeline", label: "学生 · AI 交互证据", ref: timelineRef },
    { key: "prompt", label: "提示词透镜", ref: promptRef },
    ...(workAndProcess ? [{ key: "work", label: "作品与过程", ref: workRef }] : []),
    { key: "next", label: "下一步脚手架", ref: nextRef },
  ];

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", position: "relative" }}>
      {/* catalog nav */}
      <div style={{ position: "sticky", float: "right", top: 24, marginRight: 24, width: 196, background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: 10, boxShadow: "0 10px 28px rgba(20,30,60,.10)" }}>
        <div style={{ fontSize: 11, fontWeight: 700, color: "#9198A8", letterSpacing: ".04em", padding: "4px 10px 8px" }}>报告目录</div>
        {catalog.map((c) => (
          <div key={c.key} onClick={() => scrollTo(c.ref)} style={{ fontSize: 12.5, color: "#4A5060", padding: "7px 10px", borderRadius: 8, cursor: "pointer" }}>
            {c.label}
          </div>
        ))}
      </div>

      <div style={{ maxWidth: 1080, margin: "0 auto", padding: "22px 236px 72px 30px" }}>
        {/* header */}
        <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 14, fontSize: 13, fontWeight: 600 }}>
          <span onClick={onBack} style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "#6C7488", cursor: "pointer" }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round"><path d="M19 12H5M11 18l-6-6 6-6" /></svg>
            全部学生
          </span>
          <span style={{ color: "#C6CBD8" }}>/</span>
          <span onClick={onBack} style={{ color: "#6C7488", cursor: "pointer" }}>{studentName ?? title}</span>
        </div>
        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", letterSpacing: ".03em" }}>能力报告 · 教师视图</div>
            <div style={{ fontSize: 22, fontWeight: 800, color: "#1C2333", marginTop: 6, lineHeight: 1.25 }}>{studentName ? `${studentName} · ${title}` : title}</div>
            <div style={{ marginTop: 10, display: "flex", flexWrap: "wrap", gap: 7 }}>
              {chips.map((chip) => (
                <span key={chip} style={{ display: "inline-flex", alignItems: "center", minHeight: 26, padding: "4px 11px", border: "1px solid #E4E7EF", borderRadius: 20, background: "#FAFBFD", fontSize: 12, color: "#4A5060" }}>{chip}</span>
              ))}
            </div>
          </div>
          <span
            title="家长版报告即将上线"
            aria-disabled="true"
            style={{ flex: "none", display: "inline-flex", alignItems: "center", gap: 7, background: "#F5F6FA", border: "1px solid #E1E4ED", color: "#B4BAC8", fontSize: 13, fontWeight: 700, padding: "10px 16px", borderRadius: 11, cursor: "not-allowed" }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
            导出家长版 PDF
          </span>
        </div>

        {/* 1. 总览 */}
        <div ref={overviewRef} />
        <h2 style={H2_STYLE}>总览</h2>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1.4fr) minmax(220px,.6fr)", gap: 18, alignItems: "stretch" }}>
          <div style={CARD_STYLE}>
            {context.researchQuestion ? (
              <>
                <div style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A" }}>Research Question</div>
                <div style={{ marginTop: 8, fontSize: 16, fontWeight: 700, color: "#1C2333", lineHeight: 1.5 }}>{context.researchQuestion}</div>
              </>
            ) : null}
            {narrative ? (
              <>
                <div style={{ marginTop: 18, fontSize: 12, fontWeight: 700, color: "#6C7488" }}>研究概况</div>
                <div style={{ marginTop: 8, fontSize: 14, color: "#4A5060", lineHeight: 1.8 }}>{narrative}</div>
              </>
            ) : null}
            <div style={{ marginTop: 18, padding: "14px 15px", background: "#F7F8FB", borderRadius: 12, fontSize: 12.5, color: "#7A8296", lineHeight: 1.7 }}>{axiom}</div>
          </div>
          {officialProjection ? (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <div style={CARD_STYLE}>
                <div style={{ fontSize: 12.5, color: "#6C7488", fontWeight: 600 }}>训练用作品就绪度折算</div>
                <div style={{ marginTop: 8, fontSize: 44, fontWeight: 800, color: "#2A3B7A", lineHeight: 1 }}>{officialProjection.readiness.score}</div>
                <div style={{ marginTop: 12, height: 9, borderRadius: 6, background: "#F1F2F6", overflow: "hidden" }}>
                  <div style={{ height: "100%", width: `${officialProjection.readiness.score}%`, borderRadius: 6, background: "linear-gradient(90deg,#2A3B7A,#5C74C4)" }} />
                </div>
                <div style={{ marginTop: 12, fontSize: 12.5, color: "#7A8296", lineHeight: 1.6 }}>{officialProjection.readiness.note}</div>
              </div>
            </div>
          ) : null}
        </div>

        {/* 2. 官方作品投影 — project only */}
        {officialProjection ? (
          <>
            <div ref={officialRef} />
            <h2 style={H2_STYLE}>官方作品投影</h2>
            <div style={CAPTION_STYLE}>先按 {officialProjection.standard.name} 官方标准（Academic Paper 档位、POD、PREP/真实性）定位作品，再解释双轴证据。</div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 14 }}>
              {officialProjection.components.map((c, i) => (
                <div key={i} style={CARD_STYLE}>
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                    <span style={{ fontSize: 14, fontWeight: 800, color: "#2A3B7A" }}>{c.name}</span>
                    <span style={{ fontSize: 12, fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 10px", borderRadius: 8 }}>{c.judgement}</span>
                  </div>
                  <div style={{ marginTop: 8, fontSize: 13.5, color: "#4A5060", lineHeight: 1.75 }}>{c.reason}</div>
                </div>
              ))}
            </div>
            <div style={{ marginTop: 22, fontSize: 15, fontWeight: 800, color: "#1C2333" }}>官方标准逐条对齐</div>
            <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 10 }}>
              {officialProjection.alignment.map((a, i) => (
                <div key={i} style={CARD_STYLE}>
                  <div style={{ fontSize: 13.5, fontWeight: 800, color: "#1C2333" }}>{a.item}</div>
                  <div style={{ marginTop: 10, display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
                    <div>
                      <div style={{ fontSize: 11, fontWeight: 700, color: "#8A92A3" }}>官方要求</div>
                      <div style={{ marginTop: 4, fontSize: 12.5, color: "#4A5060", lineHeight: 1.65 }}>{a.standard}</div>
                    </div>
                    <div>
                      <div style={{ fontSize: 11, fontWeight: 700, color: "#8A92A3" }}>本作品表现</div>
                      <div style={{ marginTop: 4, fontSize: 12.5, color: "#4A5060", lineHeight: 1.65 }}>{a.performance}</div>
                    </div>
                    <div>
                      <div style={{ fontSize: 11, fontWeight: 700, color: "#8A92A3" }}>影响</div>
                      <div style={{ marginTop: 4, fontSize: 12.5, color: "#4A5060", lineHeight: 1.65 }}>{a.impact}</div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {/* 3. D / A 双轴读数 */}
        <div ref={dualRef} />
        <h2 style={H2_STYLE}>D / A 双轴读数</h2>
        <div style={CAPTION_STYLE}>D 轴看研究理解、证据和论证能走多深；A 轴看学生是否真正拥有这些研究判断。两者不合成总分。</div>
        <div style={{ fontSize: 13, fontWeight: 800, color: "#2A3B7A", background: "#EDEFF9", borderRadius: 10, padding: "9px 14px", marginBottom: 12 }}>D 轴 · 认知深度</div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
          {depthAxis.map((d) => (
            <DimCard key={d.code} name={d.name} evidence={d.evidence} badge={<DepthBadge level={d.level} levelRange={d.levelRange} />} />
          ))}
        </div>
        <div style={{ fontSize: 13, fontWeight: 800, color: "#3E8A6E", background: "#EAF3EE", borderRadius: 10, padding: "9px 14px", margin: "22px 0 12px" }}>A 轴 · 智识自主</div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
          {autonomyAxis.map((a) => {
            const notSupplied = a.opportunity === "not_supplied";
            const c = badgeColor(String(a.level));
            return (
              <DimCard
                key={a.code}
                name={a.name}
                evidence={a.evidence}
                badge={
                  notSupplied ? (
                    <span style={{ fontSize: 12, fontWeight: 700, color: "#8A92A3", background: "#F1F2F6", padding: "2px 10px", borderRadius: 8, fontStyle: "italic" }}>暂无·机会未提供</span>
                  ) : (
                    <span style={{ fontSize: 13, fontWeight: 800, color: c.fg, background: c.bg, padding: "2px 10px", borderRadius: 8 }}>Lv {a.level}</span>
                  )
                }
              />
            );
          })}
        </div>

        {/* 4. 证据地图 — Task 11 fills this slot */}
        <div ref={mapRef} />
        <h2 style={H2_STYLE}>证据地图</h2>
        <div style={CAPTION_STYLE}>点击节点，看 RQ、官方投影、双轴、AI 互动如何串起这个项目的证据。</div>
        <div data-testid="evidence-map-slot">
          <EvidenceMap report={report} context={context} studentName={studentName} />
        </div>

        {/* 5. 学生 · AI 交互证据 */}
        <div ref={timelineRef} />
        <h2 style={H2_STYLE}>学生 · AI 交互证据</h2>
        <div style={CAPTION_STYLE}>学生输入、AI 回应摘要与可读出的评估信号，用来支撑 A 轴与提示词透镜判断。</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {interactionEvidence.map((row) => (
            <div key={row.round} style={CARD_STYLE}>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span style={{ width: 28, height: 28, flex: "none", borderRadius: "50%", background: "#2A3B7A", color: "#fff", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 13, fontWeight: 800 }}>{row.round}</span>
                <div style={{ fontSize: 13, fontWeight: 800, color: "#1C2333" }}>第 {row.round} 轮交互</div>
              </div>
              <div style={{ marginTop: 12, padding: "12px 14px", borderLeft: "3px solid #2A3B7A", background: "#F7F8FB", borderRadius: "0 10px 10px 0", fontSize: 13, color: "#2A3040", lineHeight: 1.7 }}>{row.student}</div>
              <div style={{ marginTop: 10, fontSize: 12.5, color: "#6C7488", lineHeight: 1.7 }}>{row.aiSummary}</div>
              <div style={{ marginTop: 10, paddingTop: 10, borderTop: "1px dashed #EAECF2", fontSize: 12.5, color: "#B0863A", lineHeight: 1.7, fontWeight: 600 }}>评估信号：{row.signal}</div>
            </div>
          ))}
        </div>

        {/* 6. 提示词透镜 */}
        <div ref={promptRef} />
        <h2 style={H2_STYLE}>提示词透镜</h2>
        <div style={CAPTION_STYLE}>提示词透镜只读 AI 互动痕迹，为双轴补过程证据；它不是第三根评分轴。</div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14, marginBottom: 16 }}>
          {promptLens.stats.map((s, i) => (
            <div key={i} style={{ ...CARD_STYLE, textAlign: "center" }}>
              <div style={{ fontSize: 22, fontWeight: 800, color: "#2A3B7A" }}>{s.value}</div>
              <div style={{ fontSize: 12, color: "#8A92A3", marginTop: 6 }}>{s.label}</div>
            </div>
          ))}
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
          {promptLens.lenses.map((l) => {
            const c = badgeColor(String(l.level));
            return (
              <DimCard
                key={l.code}
                name={l.name}
                evidence={l.evidence}
                badge={<span style={{ fontSize: 13, fontWeight: 800, color: c.fg, background: c.bg, padding: "2px 10px", borderRadius: 8 }}>Lv {l.level}</span>}
              />
            );
          })}
        </div>
        <div style={{ marginTop: 16, fontSize: 12, color: "#8A92A3", lineHeight: 1.7 }}>{promptLens.note}</div>

        {/* 7. 作品与过程 — project only */}
        {workAndProcess ? (
          <>
            <div ref={workRef} />
            <h2 style={H2_STYLE}>作品与过程</h2>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 12 }}>作品片段</div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 14 }}>
              {workAndProcess.workSamples.map((w, i) => (
                <div key={i} style={CARD_STYLE}>
                  <div style={{ fontSize: 13, fontWeight: 800, color: "#2A3B7A" }}>{w.title}</div>
                  <div style={{ marginTop: 10, fontSize: 13, color: "#4A5060", lineHeight: 1.8 }}>{w.text}</div>
                </div>
              ))}
            </div>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", margin: "22px 0 12px" }}>过程材料</div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 14 }}>
              {workAndProcess.processMaterials.map((m, i) => (
                <div key={i} style={CARD_STYLE}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <span style={{ fontSize: 13.5, fontWeight: 800, color: "#2A3B7A" }}>{m.name}</span>
                    <span style={{ fontSize: 11.5, fontWeight: 700, color: "#6B7384", background: "#F1F2F6", padding: "1px 8px", borderRadius: 999 }}>{m.status}</span>
                  </div>
                  <div style={{ marginTop: 9, fontSize: 12.5, color: "#4A5060", lineHeight: 1.7 }}>{m.diagnosis}</div>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {/* 8. 下一步脚手架 */}
        <div ref={nextRef} />
        <h2 style={H2_STYLE}>下一步脚手架</h2>
        <div style={CAPTION_STYLE}>只保留能推进官方表现和双轴弱点的动作，不做泛泛润色。</div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 14 }}>
          {guidance.nextSteps.map((n, i) => (
            <div key={i} style={{ ...CARD_STYLE, display: "flex", gap: 14 }}>
              <span style={{ width: 30, height: 30, flex: "none", borderRadius: 9, background: "#EDEFF9", color: "#2A3B7A", display: "flex", alignItems: "center", justifyContent: "center" }}>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round"><path d="M20 6 9 17l-5-5" /></svg>
              </span>
              <div>
                <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>{n.title}</div>
                <div style={{ marginTop: 7, fontSize: 13, color: "#4A5060", lineHeight: 1.75 }}>{n.task}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
