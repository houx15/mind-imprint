import type { EvaluationReport } from "@mind-imprint/contracts";

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
