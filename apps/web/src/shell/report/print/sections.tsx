import type { EvaluationReport } from "@mind-imprint/contracts";
import { PrintSection, PrintTable } from "./parts";

export function SummarySection({ report }: { report: EvaluationReport }) {
  const c = report.basics.counters;
  const m = report.basics.milestones;
  const a = report.abstract;
  const milestoneRow = [
    ["立项", m.started], ["框架", m.frameworkFinished], ["提案", m.proposalFinished],
    ["初稿", m.writingFinished], ["完成", m.projectFinished],
  ]
    .filter(([, v]) => !!v)
    .map(([k, v]) => `${k} ${String(v).slice(0, 10)}`)
    .join(" · ");

  return (
    <PrintSection n="02" title="综述 · 学生画像" intro="以下为过程记录的确定性汇总，均由平台自动计数，非模型评分。">
      <p style={{ fontSize: 13, color: "var(--mk-ink)", lineHeight: 1.75, margin: "0 0 10px" }}>{a.overview}</p>
      <p style={{ fontSize: 12.5, color: "var(--mk-secondary)", lineHeight: 1.7, margin: "0 0 14px" }}>
        {a.materialSentence} {a.writingSentence} {a.aiSentence}
      </p>
      <PrintTable
        head={["AI 轮次", "材料阅读", "字数", "AI 批注", "编辑次数"]}
        rows={[[c.aiTurns, c.materialsRead, c.wordsWritten, c.aiCommentCount, c.editCount]]}
      />
      {milestoneRow ? (
        <div style={{ marginTop: 12, fontSize: 11.5, color: "var(--mk-muted)" }}>里程碑：{milestoneRow}</div>
      ) : null}
      {a.suggestionParagraph ? (
        <p style={{ marginTop: 14, fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7 }}>{a.suggestionParagraph}</p>
      ) : null}
    </PrintSection>
  );
}
