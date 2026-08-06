import { Check } from "lucide-react";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";
import { Badge, Card, Icon } from "@/ui";

// Shared visual language: `Card` (design-system object card — bg-mk-surface /
// shadow-mk-md / 10px radius) stacked with a 16px top gap, matching every
// other restyled shell surface (HomePage/ToolkitCards). This component
// renders the canonical assessment object (Spec B): D6 depth axis (L1–L4|NA,
// no subtotal) + A6 autonomy axis (0–5 behavior-count band,
// opportunity-gated) + 6-lens prompt telemetry + interaction evidence +
// guidance, with an optional project-surface superset (officialProjection +
// workAndProcess). RL-5: the two axes never compose into a total score —
// the only percentage anywhere is officialProjection.readiness.score, and its
// note always disclaims composition. 证据地图 is deferred (Spec D).
//
// COLOR ENCODING (data, not chrome — see `Chip` below): the two axes get two
// distinct macarons so they're never confusable at a glance — depth = peach,
// autonomy = mist (also reused for the AI-interaction prompt-lens telemetry,
// which reads as a process/autonomy-adjacent signal, not a third axis).
// Missed-opportunity is warning (amber); the generic cross-axis "signal" tag
// on interaction evidence is info (blue); the round-number marker is accent
// (vermilion) as the one "current/emphasis" highlight. Everything else
// (surfaces, borders, body text) is neutral chrome tokens.

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

type ChipTone = "depth" | "autonomy" | "warning" | "info" | "neutral";

const CHIP_TONES: Record<ChipTone, string> = {
  depth: "bg-mk-peach-bg text-mk-peach-fg",
  autonomy: "bg-mk-mist-bg text-mk-mist-fg",
  warning: "bg-mk-warning-bg text-mk-warning",
  info: "bg-mk-info-bg text-mk-info",
  neutral: "bg-mk-paper text-mk-muted",
};

function Chip({ tone, italic, children }: { tone: ChipTone; italic?: boolean; children: React.ReactNode }) {
  return (
    <span className={cx("rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold", CHIP_TONES[tone], italic && "italic")}>
      {children}
    </span>
  );
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <div className="mb-3.5 text-mk-h3 text-mk-ink">{children}</div>;
}

function SubTitle({ children }: { children: React.ReactNode }) {
  return <div className="mb-2.5 mt-3.5 text-mk-small font-bold text-mk-ink">{children}</div>;
}

function DepthLevelBadge({ level, levelRange }: { level: DualAxisReportT["depthAxis"][number]["level"]; levelRange?: string }) {
  if (level === "NA") {
    return <Chip tone="neutral" italic>暂无可计入的证据</Chip>;
  }
  return (
    <span className="inline-flex items-center gap-1.5">
      <Chip tone="depth">{level}</Chip>
      {levelRange ? <span className="text-mk-caption text-mk-muted">区间 {levelRange}</span> : null}
    </span>
  );
}

function DepthDimCard({ d }: { d: DualAxisReportT["depthAxis"][number] }) {
  return (
    <article className="border-b border-mk-border py-2.5">
      <header className="mb-1.5 flex flex-wrap items-center justify-between gap-2.5">
        <span className="text-mk-body font-bold text-mk-ink">{d.name}</span>
        <DepthLevelBadge level={d.level} levelRange={d.levelRange} />
      </header>
      <p className="m-0 text-mk-small text-mk-muted">{d.evidence}</p>
      {d.promptEvidence ? (
        <p className="m-0 mt-1 text-mk-small text-mk-secondary">提示词证据：{d.promptEvidence}</p>
      ) : null}
    </article>
  );
}

function AutonomySignalCard({ a }: { a: DualAxisReportT["autonomyAxis"][number] }) {
  const notSupplied = a.opportunity === "not_supplied";
  const missed = a.opportunity === "given_not_taken";
  return (
    <article className="border-b border-mk-border py-2.5">
      <header className="mb-1.5 flex flex-wrap items-center justify-between gap-2.5">
        <span className="text-mk-body font-bold text-mk-ink">{a.name}</span>
        {notSupplied ? (
          <Chip tone="neutral" italic>暂无·机会未提供</Chip>
        ) : (
          <span className="inline-flex items-center gap-1.5">
            <Chip tone="autonomy">Lv {a.level}</Chip>
            {missed ? <Chip tone="warning">机会已给·未接住</Chip> : null}
          </span>
        )}
      </header>
      <p className="m-0 text-mk-small text-mk-muted">{a.evidence}</p>
      {a.promptEvidence ? (
        <p className="m-0 mt-1 text-mk-small text-mk-secondary">提示词证据：{a.promptEvidence}</p>
      ) : null}
    </article>
  );
}

function LensStatCard({ s }: { s: DualAxisReportT["promptLens"]["stats"][number] }) {
  return (
    <div data-testid="lens-stat-card" className="flex-[1_1_160px] rounded-mk-md bg-mk-paper px-3.5 py-2.5 text-mk-small text-mk-ink">
      <div className="text-mk-caption text-mk-muted">{s.label}</div>
      <div className="mt-0.5 text-mk-h3 text-mk-ink">{s.value}</div>
    </div>
  );
}

function LensCard({ l }: { l: DualAxisReportT["promptLens"]["lenses"][number] }) {
  return (
    <article data-testid="lens-card" className="border-b border-mk-border py-2.5">
      <header className="mb-1 flex items-center justify-between gap-2.5">
        <span className="text-mk-body font-bold text-mk-ink">{l.name}</span>
        <Chip tone="autonomy">Lv {l.level}</Chip>
      </header>
      <p className="m-0 text-mk-small text-mk-muted">{l.evidence}</p>
    </article>
  );
}

export function DualAxisReport({ report }: { report: DualAxisReportT }) {
  const { depthAxis, autonomyAxis, promptLens, interactionEvidence, guidance, narrative, axiom, officialProjection, workAndProcess } = report;

  return (
    <div className="mx-auto max-w-[760px]">
      {/* 总览 */}
      <div className="rounded-mk-md bg-mk-ink px-7 py-7 shadow-mk-md">
        <div className="text-mk-label text-white/70">思维印记 · 双轴成长报告</div>
        <p className="mt-3.5 text-mk-small leading-relaxed text-white/85">{narrative}</p>
        <p className="mt-2.5 border-t border-white/15 pt-2.5 text-mk-caption leading-relaxed text-white/70">{axiom}</p>
      </div>

      {/* D 轴 · 认知深度 */}
      <Card className="mt-4 p-6">
        <SectionTitle>认知深度</SectionTitle>
        {depthAxis.map((d) => <DepthDimCard key={d.code} d={d} />)}
      </Card>

      {/* A 轴 · 智识自主 */}
      <Card className="mt-4 p-6">
        <SectionTitle>智识自主</SectionTitle>
        {autonomyAxis.map((a) => <AutonomySignalCard key={a.code} a={a} />)}
      </Card>

      {/* 提示词透镜 */}
      <Card className="mt-4 p-6">
        <SectionTitle>提示词透镜</SectionTitle>
        <div className="mb-3.5 flex flex-wrap gap-2.5">
          {promptLens.stats.map((s, i) => <LensStatCard key={i} s={s} />)}
        </div>
        {promptLens.lenses.map((l) => <LensCard key={l.code} l={l} />)}
        <p className="m-0 mt-3 text-mk-caption leading-relaxed text-mk-muted">{promptLens.note}</p>
      </Card>

      {/* 交互证据 */}
      <Card className="mt-4 p-6">
        <SectionTitle>交互证据</SectionTitle>
        {interactionEvidence.map((row) => (
          <article key={row.round} className="border-b border-mk-border py-2.5">
            <Badge tone="progress" className="mb-1">R{row.round}</Badge>
            <blockquote className="m-0 mb-1.5 border-l-2 border-mk-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink">{row.student}</blockquote>
            <p className="m-0 mb-1 text-mk-small text-mk-secondary">{row.aiSummary}</p>
            <Chip tone="info">{row.signal}</Chip>
          </article>
        ))}
      </Card>

      {/* 下一步 */}
      <Card className="mt-4 p-6">
        <SectionTitle>下一步</SectionTitle>
        {guidance.nextSteps.map((n, i) => (
          <article key={i} className={cx("py-2", i > 0 && "border-t border-mk-border")}>
            <div className="text-mk-body font-bold text-mk-ink">{n.title}</div>
            <p className="m-0 mt-1 text-mk-body text-mk-secondary">{n.task}</p>
          </article>
        ))}
      </Card>

      {/* 官方投影 — project surface only */}
      {officialProjection ? (
        <Card className="mt-4 p-6">
          <SectionTitle>官方投影</SectionTitle>
          <div className="mb-2.5 text-mk-small text-mk-muted">对标 {officialProjection.standard.name}</div>

          <SubTitle>档位判定</SubTitle>
          {officialProjection.components.map((c, i) => (
            <article key={i} className="border-b border-mk-border py-2">
              <header className="flex flex-wrap items-center justify-between gap-2.5">
                <span className="text-mk-body font-bold text-mk-ink">{c.name}</span>
                <Chip tone="depth">{c.judgement}</Chip>
              </header>
              <p className="m-0 mt-1 text-mk-small text-mk-muted">{c.reason}</p>
            </article>
          ))}

          <SubTitle>对齐情况</SubTitle>
          {officialProjection.alignment.map((row, i) => (
            <article key={i} className="border-b border-mk-border py-2">
              <div className="text-mk-body font-bold text-mk-ink">{row.item}</div>
              <p className="m-0 mt-0.5 text-mk-small text-mk-secondary">要求：{row.standard}</p>
              <p className="m-0 mt-0.5 text-mk-small text-mk-ink">表现：{row.performance}</p>
              <p className="m-0 mt-0.5 text-mk-small text-mk-muted">影响：{row.impact}</p>
            </article>
          ))}

          <div className="mt-3.5 flex flex-wrap items-center gap-4 border-t border-mk-border pt-3.5">
            <div className="text-mk-h1 text-mk-ink">{officialProjection.readiness.score} / 100</div>
            <p className="m-0 flex-[1_1_240px] text-mk-small text-mk-muted">{officialProjection.readiness.note}</p>
          </div>
        </Card>
      ) : null}

      {/* 作品与过程 — project surface only. 证据地图 is deferred (Spec D). */}
      {workAndProcess ? (
        <Card className="mt-4 p-6">
          <SectionTitle>作品与过程</SectionTitle>

          <SubTitle>作品片段</SubTitle>
          {workAndProcess.workSamples.map((w, i) => (
            <article key={i} className="border-b border-mk-border py-2">
              <div className="text-mk-body font-bold text-mk-ink">{w.title}</div>
              <p className="m-0 mt-1 text-mk-body leading-relaxed text-mk-secondary">{w.text}</p>
            </article>
          ))}

          <SubTitle>过程材料</SubTitle>
          {workAndProcess.processMaterials.map((m, i) => (
            <article key={i} className="flex items-start gap-2.5 border-b border-mk-border py-2">
              {m.status === "完成" ? <Icon icon={Check} size={14} className="mt-0.5 shrink-0 text-mk-success" /> : <span className="w-3.5 shrink-0" />}
              <div className="flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-mk-body font-bold text-mk-ink">{m.name}</span>
                  <Chip tone="neutral">{m.status}</Chip>
                </div>
                <p className="m-0 mt-0.5 text-mk-small text-mk-muted">{m.diagnosis}</p>
              </div>
            </article>
          ))}
        </Card>
      ) : null}
    </div>
  );
}
