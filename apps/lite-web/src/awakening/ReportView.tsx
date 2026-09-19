import { useRef, useState } from "react";

import type { AwakeningReport } from "../api/awakening";
import { exportPoster } from "../reports/exportPoster";
import { REPORT, TALENT_CARDS } from "./content";

/**
 * ReportView —— 走完之后那份报告。
 *
 * # 它为什么是暖色的
 *
 * 房间是冷的，报告是暖的。这个温差是**出门那一下**：她从外面那套系统回到
 * 自己的树。房间里一个 mk token 都不用（见 awakening.css 头部），这一屏正好
 * 相反，整屏都用 mk —— 它已经在产品里了。
 *
 * 🚨 mk 是裸 CSS 变量，任何 Tailwind 透明度修饰（`bg-mk-accent/40`）都不会
 * 生成 CSS（memory: tailwind-mk-token-alpha-trap）。这里每一处半透明都用
 * `color-mix()` 写。
 *
 * # 哪些字是她的，哪些是模型的
 *
 * 「你的问题」「你在追什么」下面那些引文，全部是她自己敲的字，逐字。
 * 「可能的驱动力」是模型的推测，所以它自带一句说明，说清楚它是推测。
 * 这条界线在界面上必须看得见 —— 一段生成的话署上「你自己说的」是谎话
 * （memory: ai-errors-must-surface-never-fake）。
 */

const FIELD_ZH: Record<string, string> = {
  formal: "数学与形式",
  science: "科学与自然",
  making: "技术与创造",
  society: "社会与世界",
  humanities: "人文与写作",
  arts: "艺术与表达",
  self: "自我与成长",
};

function Section({
  title,
  children,
  note,
}: {
  title: string;
  children: React.ReactNode;
  note?: string;
}) {
  return (
    <section className="mt-9 first:mt-0">
      <h2 className="text-mk-h3 font-semibold text-mk-ink">{title}</h2>
      {note ? <p className="mt-1 text-mk-small text-mk-muted">{note}</p> : null}
      <div className="mt-3">{children}</div>
    </section>
  );
}

function Card({ children, accent }: { children: React.ReactNode; accent?: string }) {
  return (
    <div
      className="rounded-mk-lg border p-4"
      style={{
        borderColor: accent
          ? `color-mix(in srgb, ${accent} 38%, transparent)`
          : "var(--mk-line)",
        background: accent
          ? `color-mix(in srgb, ${accent} 7%, transparent)`
          : "var(--mk-surface)",
      }}
    >
      {children}
    </div>
  );
}

export function ReportView({
  report,
  onBackToTree,
  onOpenReading,
}: {
  report: AwakeningReport;
  onBackToTree: () => void;
  /** 点一篇推荐 —— 这是「报告 → 阅读室」那条闭环的落点。tier 是给她的难度档。 */
  onOpenReading: (slug: string, tier: number) => void;
}) {
  const posterRef = useRef<HTMLDivElement>(null);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState("");

  const doExport = async () => {
    setExporting(true);
    setExportError("");
    const err = await exportPoster(posterRef.current, "兴趣印记.png");
    if (err) setExportError(err);
    setExporting(false);
  };

  return (
    <div className="mx-auto w-full max-w-3xl px-5 py-10 sm:px-8">
      <div ref={posterRef}>
        <p className="font-mono text-mk-small uppercase tracking-[0.18em] text-mk-accent-500">
          {REPORT.eyebrow}
        </p>
        <h1 className="mt-2 text-mk-h1 text-mk-ink">{REPORT.title}</h1>
        {report.summary ? (
          <p className="mt-3 text-mk-body leading-[1.9] text-mk-secondary">{report.summary}</p>
        ) : null}

        {/* 1 · 她在追什么 —— 这次动了的词 */}
        <Section title={REPORT.sections.pursuing}>
          {report.pursuing.length === 0 ? (
            <p className="text-mk-body leading-[1.9] text-mk-secondary">{REPORT.emptyPursuing}</p>
          ) : (
            <div className="grid gap-3">
              {report.pursuing.map((w) => (
                <Card key={w.interestId}>
                  <div className="flex flex-wrap items-baseline gap-2">
                    <span className="text-mk-h3 font-semibold text-mk-ink">{w.zh}</span>
                    <span className="text-mk-small text-mk-muted">{FIELD_ZH[w.field] ?? w.field}</span>
                    <span
                      className="rounded-mk-full px-2 py-0.5 text-mk-small"
                      style={{
                        background:
                          w.verdict === "confirm"
                            ? "color-mix(in srgb, var(--mk-lake) 16%, transparent)"
                            : "color-mix(in srgb, var(--mk-accent-400) 18%, transparent)",
                        color:
                          w.verdict === "confirm" ? "var(--mk-lake)" : "var(--mk-accent-600)",
                      }}
                    >
                      {w.verdict === "confirm" ? REPORT.confirmTag : REPORT.growTag}
                    </span>
                    {w.strength > 0 ? (
                      <span className="ml-auto font-mono text-mk-small tabular-nums text-mk-muted">
                        强度 {w.strength}
                      </span>
                    ) : null}
                  </div>
                  {w.note ? (
                    <p className="mt-2 text-mk-small leading-relaxed text-mk-secondary">{w.note}</p>
                  ) : null}
                  {/* 她自己的那句话，逐字。它是这个词的全部说服力。 */}
                  <p className="mt-2 border-l-2 pl-3 text-mk-small leading-[1.9] text-mk-ink"
                     style={{ borderColor: "var(--mk-accent-300)" }}>
                    {w.evidence}
                  </p>
                </Card>
              ))}
            </div>
          )}
        </Section>

        {/* 2 · 可能的驱动力 —— 模型的推测，标明它是推测 */}
        <Section title={REPORT.sections.drivers} note={REPORT.driversNote}>
          {report.drivers.length === 0 ? (
            <p className="text-mk-body text-mk-secondary">{REPORT.emptyDrivers}</p>
          ) : (
            <div className="grid gap-3">
              {report.drivers.map((d) => (
                <Card key={d.label}>
                  <div className="flex items-baseline justify-between gap-3">
                    <span className="text-mk-body font-semibold text-mk-ink">{d.label}</span>
                    <span className="font-mono text-mk-small tabular-nums text-mk-muted">
                      {Math.round(d.confidence * 100)}%
                    </span>
                  </div>
                  <p className="mt-2 border-l-2 pl-3 text-mk-small leading-[1.9] text-mk-secondary"
                     style={{ borderColor: "var(--mk-line)" }}>
                    {d.evidence}
                  </p>
                </Card>
              ))}
            </div>
          )}
        </Section>

        {/* 3 · 她的问题 —— 原样 */}
        {report.question ? (
          <Section title={REPORT.sections.question}>
            <p className="text-mk-h3 leading-[1.8] text-mk-ink">{report.question}</p>
            {report.workConcept ? (
              <p className="mt-3 text-mk-body leading-[1.9] text-mk-secondary">
                {report.workConcept}
              </p>
            ) : null}
          </Section>
        ) : null}

        {/* 4 · 怎么靠近 —— 她自己的分堆 */}
        {report.talent.some((p) => p.cards.length > 0) ? (
          <Section title={REPORT.sections.talent}>
            <div className="grid gap-3 sm:grid-cols-3">
              {report.talent.map((pile) => (
                <Card key={pile.key}>
                  <div className="text-mk-small font-semibold text-mk-ink">{pile.label}</div>
                  <ul className="mt-2 grid gap-1">
                    {pile.cards.map((id) => {
                      const c = TALENT_CARDS.find((x) => x.id === id);
                      return (
                        <li key={id} className="text-mk-small text-mk-secondary">
                          {c ? `${c.mark} ${c.title}` : id}
                        </li>
                      );
                    })}
                    {pile.cards.length === 0 ? (
                      <li className="text-mk-small text-mk-faint">无</li>
                    ) : null}
                  </ul>
                </Card>
              ))}
            </div>
          </Section>
        ) : null}

        {/* 6 · 和上次比 —— 第二趟起才有 */}
        {report.diff ? (
          <Section title={REPORT.sections.diff}>
            <Card>
              <p className="text-mk-body leading-[1.9] text-mk-ink">
                距上一次 {report.diff.daysBetween} 天。
                {report.diff.stronger.length > 0
                  ? `「${report.diff.stronger.join("」「")}」又出现了一次，强度上升。`
                  : ""}
                {report.diff.new.length > 0
                  ? `「${report.diff.new.join("」「")}」是这次新长出来的。`
                  : ""}
                {report.diff.stronger.length === 0 && report.diff.new.length === 0
                  ? "这次没有词发生变化。"
                  : ""}
              </p>
              {report.diff.previousQuestion ? (
                <p className="mt-3 text-mk-small leading-[1.9] text-mk-secondary">
                  上次你写下的问题：{report.diff.previousQuestion}
                </p>
              ) : null}
            </Card>
          </Section>
        ) : null}
      </div>

      {/* 5 · 下一步 —— 闭环的落点。不进导出的图片：它是按钮，不是内容。 */}
      <Section title={REPORT.sections.next}>
        {report.readings.length === 0 ? (
          <p className="text-mk-body text-mk-secondary">{REPORT.emptyReadings}</p>
        ) : (
          <div className="grid gap-3">
            {report.readings.map((a) => (
              <button
                key={a.slug}
                type="button"
                onClick={() => onOpenReading(a.slug, a.tier)}
                className="rounded-mk-lg border p-4 text-left transition-colors duration-[140ms] hover:bg-[rgba(51,48,46,.04)]"
                style={{ borderColor: "var(--mk-line)" }}
              >
                <div className="text-mk-body font-semibold text-mk-ink">
                  {a.zhTitle || a.title}
                </div>
                <div className="mt-1 text-mk-small text-mk-muted">
                  {FIELD_ZH[a.field] ?? a.field} · 第 {a.tier} 档
                </div>
              </button>
            ))}
          </div>
        )}

        {report.openFields.length > 0 ? (
          <p className="mt-4 text-mk-small text-mk-muted">
            {REPORT.openFieldsLead}
            {report.openFields.map((f) => FIELD_ZH[f] ?? f).join("、")}
          </p>
        ) : null}
      </Section>

      <div className="mt-10 flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={onBackToTree}
          className="rounded-mk-full px-5 py-2 text-mk-body font-semibold text-white transition hover:opacity-90"
          style={{ background: "linear-gradient(135deg,var(--mk-accent-400),var(--mk-accent-600))" }}
        >
          {REPORT.backToTree}
        </button>
        <button
          type="button"
          onClick={() => void doExport()}
          disabled={exporting}
          className="rounded-mk-full border px-5 py-2 text-mk-body transition-colors duration-[120ms] hover:bg-[rgba(51,48,46,.05)]"
          style={{ borderColor: "var(--mk-accent-300)", color: "var(--mk-accent-500)" }}
        >
          {exporting ? REPORT.exporting : REPORT.exportImage}
        </button>
      </div>
      {exportError ? (
        <p className="mt-3 text-mk-small" style={{ color: "var(--mk-danger, #c0392b)" }}>
          导出失败：{exportError}
        </p>
      ) : null}
    </div>
  );
}
