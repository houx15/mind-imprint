import type { EvaluationReport, EvidenceItem } from "@mind-imprint/contracts";
import { DUALAXIS_MODEL } from "@mind-imprint/contracts";

export function Cover({ report }: { report: EvaluationReport }) {
  const b = report.basics;
  const range = [b.startDate?.slice(0, 10), b.endDate ? b.endDate.slice(0, 10) : "进行中"]
    .filter(Boolean)
    .join(" — ");
  return (
    <section className="print-section print-page" data-testid="print-cover" style={{ paddingTop: 96 }}>
      <div style={{ fontSize: 13, fontWeight: 700, color: "var(--mk-accent-600)", letterSpacing: ".08em" }}>
        思维印记 · 过程评估报告
      </div>
      <h1 style={{ fontSize: 30, fontWeight: 800, color: "var(--mk-ink)", margin: "18px 0 10px", lineHeight: 1.25 }}>
        {b.title}
      </h1>
      <div style={{ fontSize: 15, color: "var(--mk-secondary)", marginBottom: 40 }}>
        {report.student.name}
      </div>
      <dl style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "8px 20px", fontSize: 13, color: "var(--mk-secondary)", margin: 0 }}>
        <dt style={{ color: "var(--mk-muted)" }}>项目类型</dt><dd style={{ margin: 0 }}>{b.type}</dd>
        <dt style={{ color: "var(--mk-muted)" }}>起止日期</dt><dd style={{ margin: 0 }}>{range}</dd>
        <dt style={{ color: "var(--mk-muted)" }}>生成日期</dt><dd style={{ margin: 0 }}>{report.generatedAt.slice(0, 10)}</dd>
      </dl>
      <p style={{ marginTop: 56, fontSize: 12, color: "var(--mk-muted)", lineHeight: 1.7, borderTop: "1px solid var(--mk-border)", paddingTop: 16 }}>
        本报告记录学生与 AI 协作中的思考过程，是事件级证据的整理，不构成人级评分。两轴（认知深度、智识自主）永不合成总分。
      </p>
    </section>
  );
}

export function PrintSection({
  n, title, intro, breakBefore, children,
}: {
  n: string; title: string; intro?: React.ReactNode; breakBefore?: boolean; children: React.ReactNode;
}) {
  return (
    <section
      className={`print-section print-page${breakBefore ? " print-page-break" : ""}`}
      style={{ marginBottom: 30 }}
      data-testid={`print-section-${n}`}
    >
      <div style={{ display: "flex", alignItems: "baseline", gap: 10, marginBottom: intro ? 6 : 12 }}>
        <span style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-accent-600)" }}>{n}</span>
        <h2 style={{ fontSize: 19, fontWeight: 800, color: "var(--mk-ink)", margin: 0 }}>{title}</h2>
      </div>
      {intro ? (
        <p style={{ fontSize: 12.5, color: "var(--mk-muted)", lineHeight: 1.65, margin: "0 0 14px" }}>{intro}</p>
      ) : null}
      {children}
    </section>
  );
}

const thStyle: React.CSSProperties = {
  textAlign: "left", padding: "7px 9px", borderBottom: "1.5px solid var(--mk-border)",
  fontSize: 11.5, fontWeight: 700, color: "var(--mk-secondary)", background: "var(--mk-paper)",
};
const tdStyle: React.CSSProperties = {
  padding: "7px 9px", borderBottom: "1px solid var(--mk-border)", fontSize: 12,
  color: "var(--mk-ink)", verticalAlign: "top",
};

export function PrintTable({ head, rows }: { head: string[]; rows: React.ReactNode[][] }) {
  return (
    <table style={{ width: "100%", borderCollapse: "collapse" }} className="print-avoid-break">
      <thead>
        <tr>{head.map((h, i) => <th key={i} style={thStyle}>{h}</th>)}</tr>
      </thead>
      <tbody>
        {rows.map((r, ri) => (
          <tr key={ri}>{r.map((c, ci) => <td key={ci} style={tdStyle}>{c}</td>)}</tr>
        ))}
      </tbody>
    </table>
  );
}

export function EvidenceList({ evidence }: { evidence: EvidenceItem[] }) {
  if (evidence.length === 0) return null;
  return (
    <ul style={{ listStyle: "none", margin: "12px 0 0", padding: 0, display: "flex", flexDirection: "column", gap: 12 }}>
      {evidence.map((ev, i) => (
        <li key={i} className="print-avoid-break">
          <blockquote style={{ margin: 0, borderLeft: "2px solid var(--mk-lake)", paddingLeft: 10, fontSize: 12.5, color: "var(--mk-ink)" }}>
            {ev.quote}
          </blockquote>
          <div style={{ marginTop: 5, fontSize: 11.5, color: "var(--mk-muted)" }}>
            {ev.stage ? `${ev.stage} · ` : null}
            {ev.observation}
          </div>
          {ev.boundary ? (
            <div style={{ marginTop: 3, fontSize: 11.5, color: "var(--mk-danger)" }}>边界：{ev.boundary}</div>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

export function AboutReport({ report }: { report: import("@mind-imprint/contracts").EvaluationReport }) {
  void report;
  return (
    <PrintSection n="—" title="关于这份报告">
      <p style={{ fontSize: 13, color: "var(--mk-ink)", lineHeight: 1.7, margin: "0 0 12px" }}>
        {DUALAXIS_MODEL.axiom}
      </p>
      <p style={{ fontSize: 12.5, color: "var(--mk-secondary)", lineHeight: 1.7, margin: 0 }}>
        本报告用两个轴描述过程：<b>认知深度 D</b>——思考推进得有多深；<b>智识自主 A</b>——是否由学生自己驱动认知。
        两轴各自独立，永不合成一个总分；每个维度的具体读法写在对应章节的开头。
      </p>
    </PrintSection>
  );
}
