import type { EvaluationReport, EventEntry, MaterialEntry } from "@mind-imprint/contracts";
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

const EVENT_KIND_LABEL: Record<EventEntry["kind"], string> = {
  chat: "对话", reading: "阅读", graph: "探索", writing: "写作", review: "回顾", milestone: "里程碑",
};

export function TimelineSection({ events }: { events: EventEntry[] }) {
  return (
    <PrintSection n="03" title="过程时间线" intro="下表由平台的事件轨迹自动生成，按时间顺序还原学生这个项目的关键动作。">
      <PrintTable
        head={["时间", "阶段", "摘要", "AI 轮次"]}
        rows={events.map((e) => [e.ts.slice(0, 10), EVENT_KIND_LABEL[e.kind] ?? e.kind, e.summary, e.aiTurns])}
      />
    </PrintSection>
  );
}

export function MaterialsSection({ materials }: { materials: MaterialEntry[] }) {
  return (
    <PrintSection
      n="04"
      title="材料清单"
      intro="材料清单记录学生用过的每一份来源，以及对它的溯源体检：来源等级与可信度、最终判定（用 / 存疑 / 弃用）、这份材料用在了哪里、以及它能支持什么、不能支持什么。这是论证是否有据的地基。"
    >
      <PrintTable
        head={["来源", "加入时间", "最终判定", "用在哪", "备注", "局限"]}
        rows={materials.map((m) => [
          m.url ? `${m.source}（${m.url}）` : m.source,
          m.addedAt.slice(0, 10),
          m.finalStatus,
          m.usedIn?.label ?? "—",
          m.comment || "—",
          m.cannotSupport || "—",
        ])}
      />
    </PrintSection>
  );
}
