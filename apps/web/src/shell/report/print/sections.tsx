import type { EvaluationReport, EventEntry, MaterialEntry, PromptLens, RiskEntry, ToolUsageEntry } from "@mind-imprint/contracts";
import { DUALAXIS_MODEL } from "@mind-imprint/contracts";
import { EVENT_KIND, RISK_LABEL } from "../EvaluationReport/tokens";
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

export function TimelineSection({ events }: { events: EventEntry[] }) {
  return (
    <PrintSection n="03" title="过程时间线" intro="下表由平台的事件轨迹自动生成，按时间顺序还原学生这个项目的关键动作。">
      <PrintTable
        head={["时间", "阶段", "摘要", "AI 轮次"]}
        rows={events.map((e) => [e.ts.slice(0, 10), EVENT_KIND[e.kind]?.label ?? e.kind, e.summary, e.aiTurns])}
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

export function PromptLensSection({ promptLens }: { promptLens: PromptLens }) {
  const lensList = DUALAXIS_MODEL.lenses.map((l) => l.name).join("、");
  return (
    <PrintSection
      n="07"
      title="提问透镜"
      intro={`${DUALAXIS_MODEL.lensNote}它只读 AI 互动痕迹（如${lensList}），是给两轴补过程证据的旁证，不是第三根评分轴。`}
    >
      {promptLens.summary ? (
        <p style={{ fontSize: 12.5, color: "var(--mk-ink)", lineHeight: 1.7, margin: "0 0 12px" }}>{promptLens.summary}</p>
      ) : null}
      <PrintTable
        head={["阶段", "提问", "观察", "关联维度", "注意"]}
        rows={promptLens.prompts.map((p) => [
          p.stage, p.quote, p.observation, p.relatedDomains.join("、") || "—", p.attention ? "⚠" : "",
        ])}
      />
    </PrintSection>
  );
}

export function ToolUsageSection({ toolUsage }: { toolUsage: ToolUsageEntry[] }) {
  return (
    <PrintSection
      n="08"
      title="工具卡与子代理"
      intro="思维工具卡是把「思考」在对的时刻塞回给学生的交互卡片——触发是自动的，但打开由学生自己确认（我们不强迫、不操纵）。下表记录这个项目里被真正用到的工具卡与其用途。"
    >
      <PrintTable
        head={["工具", "阶段", "用途", "摘要"]}
        rows={toolUsage.map((t) => [t.name, t.stage, t.purpose, t.summary])}
      />
    </PrintSection>
  );
}

export function RisksSection({ risks }: { risks: RiskEntry[] }) {
  return (
    <PrintSection
      n="09"
      title="风险提示"
      intro="风险提示是过程中值得和学生聊一聊的信号，不是定论，也不影响任何评级。"
    >
      {risks.length === 0 ? (
        <p style={{ fontSize: 12.5, color: "var(--mk-secondary)", margin: 0 }}>本次评估未发现需要提示的风险行为。</p>
      ) : (
        <PrintTable
          head={["类型", "行为", "建议"]}
          rows={risks.map((r) => [RISK_LABEL[r.type] ?? r.type, r.behaviour, r.suggestion])}
        />
      )}
    </PrintSection>
  );
}
