import { useState } from "react";
import { Check, Info } from "lucide-react";
import { useEco } from "../../store";
import { activeSteps } from "../../data/plan";
import { asRows, asText, cardById, filledRows } from "../../data/cards";
import { artifactById } from "../../data/artifacts";
import { METHODS, PBL_COVENANT } from "../../data/method";
import type { CardEntry, PlanStep, Project, StageRef } from "../../data/types";
import { Sys, cx } from "../../ui";

/**
 * 全景 — the right panel when there is nothing on the stage.
 *
 * This is the "working status" half of the Cowork shape: while she is talking,
 * the panel holds the whole picture of the project — where in the agreed plan
 * she is, what has been produced so far, and who does what. When something
 * needs doing or looking at, the stage takes the panel over and grows, and the
 * conversation shrinks to a column beside it.
 *
 * ## Why 材料 shows her own sentences
 * A rail of grey card titles is a table of contents. A rail of the sentences
 * she actually wrote is a body of work, and on week three that difference is
 * the whole reason she keeps going.
 */
export function Overview({
  project,
  onOpen,
}: {
  project: Project;
  onOpen: (ref: StageRef) => void;
}) {
  const [tab, setTab] = useState<"plan" | "stuff" | "who">("plan");
  const steps = activeSteps(project.plan);

  return (
    <div className="h-full overflow-y-auto px-5 py-5">
      <div className="mb-4 flex gap-1 rounded-mk-full p-1" style={{ background: "var(--mk-paper)" }}>
        {(
          [
            ["plan", "路线"],
            ["stuff", "材料"],
            ["who", "分工"],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            onClick={() => setTab(k)}
            className={cx(
              "flex-1 rounded-mk-full px-3 py-1.5 text-mk-small transition-colors duration-[120ms]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
              tab === k ? "bg-mk-surface font-semibold text-mk-ink shadow-mk-xs" : "text-mk-muted",
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "plan" ? (
        <PlanRail project={project} steps={steps} onOpen={onOpen} />
      ) : tab === "stuff" ? (
        <Stuff project={project} onOpen={onOpen} />
      ) : (
        <Who />
      )}
    </div>
  );
}

/** The agreed plan, vertical, with 当前 marked. Same data as the planner's
 *  graph — one source, so a step count here can never disagree with one
 *  there. */
function PlanRail({
  project,
  steps,
  onOpen,
}: {
  project: Project;
  steps: PlanStep[];
  onOpen: (ref: StageRef) => void;
}) {
  return (
    <>
      {project.decision ? <DecisionCard project={project} /> : null}

      <Sys className="mb-2 block">
        路线 · 第 {Math.min(project.at + 1, steps.length)} / {steps.length} 步
      </Sys>
      <ol className="relative space-y-1">
        {steps.map((s, i) => {
          const past = i < project.at;
          const now = i === project.at;
          return (
            <li key={s.id} className="relative">
              <button
                type="button"
                disabled={!s.opens || i > project.at}
                onClick={() => s.opens && onOpen(s.opens)}
                className={cx(
                  "w-full rounded-mk-md border p-3 text-left transition-colors duration-[120ms]",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                  now
                    ? "border-mk-accent bg-mk-accent-50"
                    : past
                      ? "border-mk-border bg-mk-surface hover:border-mk-accent-200"
                      : "cursor-default border-dashed border-mk-border bg-transparent opacity-55",
                )}
              >
                <span className="flex items-start gap-2.5">
                  <span
                    className={cx(
                      "mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full",
                      "font-mono text-[10px] font-bold",
                    )}
                    style={{
                      background: past
                        ? "var(--mk-success)"
                        : now
                          ? "var(--mk-accent)"
                          : "var(--mk-border)",
                      color: past || now ? "#fff" : "var(--mk-faint)",
                    }}
                  >
                    {past ? <Check size={11} strokeWidth={3} /> : i + 1}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span
                      className={cx(
                        "block text-mk-body leading-[1.5]",
                        now ? "font-semibold text-mk-ink" : "text-mk-secondary",
                      )}
                    >
                      {s.title}
                    </span>
                    <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-[11px] text-mk-faint">
                      {s.when ? <span>{s.when}</span> : null}
                      {now ? <span className="text-mk-accent-700">当前</span> : null}
                    </span>
                    {now && s.decide ? (
                      <span className="mt-1.5 block text-[11px] leading-[1.6] text-mk-accent-700">
                        这一步你判断：{s.decide}
                      </span>
                    ) : null}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ol>
    </>
  );
}

/** Her decision, pinned. It is quoted back for the rest of the project so
 *  that on week three she can read her own reason rather than re-argue it. */
function DecisionCard({ project }: { project: Project }) {
  const road = project.approaches.find((a) => a.id === project.decision?.approachId);
  if (!road || !project.decision) return null;
  return (
    <div
      className="mb-4 rounded-mk-md border p-3.5"
      style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
    >
      <Sys className="!text-[#8A6320]">你选的路 · {project.decision.at}</Sys>
      <p className="mt-1 text-mk-body font-semibold text-[#6B4D14]">{road.name}</p>
      {/* Label, not a sentence stem: her reason very often already starts with
          「因为」, and 「因为：因为它错了可以改」 reads as the product not having
          looked at what she wrote. */}
      <p className="mt-1 text-mk-small leading-[1.75] text-[#6B4D14]">
        <span className="eco-mono block text-[#8A6320]">你写的理由</span>
        {project.decision.why}
      </p>
      <p className="mt-1.5 border-t pt-1.5 text-[11px] leading-[1.7] text-[#8A6320]"
         style={{ borderColor: "color-mix(in srgb, var(--mk-butter) 60%, transparent)" }}>
        你知道放弃了：{project.decision.gaveUp}
      </p>
    </div>
  );
}

function Stuff({ project, onOpen }: { project: Project; onOpen: (ref: StageRef) => void }) {
  const cards = project.cards.filter((c) => c.status === "done");
  const makes = Object.entries(project.artifacts).filter(([, st]) => st.status === "settled");

  if (cards.length === 0 && makes.length === 0) {
    return (
      <p className="text-mk-body leading-[1.85] text-mk-muted">
        还没有材料。每填完一张卡、每验收一次我做的东西，它就会留在这儿。
      </p>
    );
  }

  return (
    <div className="space-y-4">
      {cards.length > 0 ? (
        <div>
          <Sys className="mb-2 block">你写的</Sys>
          <ul className="space-y-1.5">
            {cards.map((c) => {
              const spec = cardById(c.cardId);
              if (!spec) return null;
              return (
                <li key={c.cardId}>
                  <button
                    type="button"
                    onClick={() => onOpen({ kind: "card", cardId: c.cardId })}
                    className="w-full rounded-mk-md border border-mk-border bg-mk-surface p-3 text-left
                               transition-colors duration-[120ms] hover:border-mk-accent-200
                               focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  >
                    <span className="flex items-center gap-2">
                      <span className="text-[13px]">{spec.glyph}</span>
                      <span className="min-w-0 flex-1 truncate text-mk-body font-semibold text-mk-ink">
                        {spec.title}
                      </span>
                      <Check size={13} strokeWidth={2.6} color="var(--mk-success)" />
                    </span>
                    <span className="mt-1 block text-mk-small leading-[1.7] text-mk-muted">
                      {trace(c)}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}

      {makes.length > 0 ? (
        <div>
          <Sys className="mb-2 block">印记做的，你验收过的</Sys>
          <ul className="space-y-1.5">
            {makes.map(([id, st]) => {
              const spec = artifactById(id);
              if (!spec) return null;
              const opt = spec.options?.find((o) => o.id === st.choice);
              return (
                <li key={id}>
                  <button
                    type="button"
                    onClick={() => onOpen({ kind: "make", artifactId: id })}
                    className="w-full rounded-mk-md border border-mk-border bg-mk-surface p-3 text-left
                               transition-colors duration-[120ms] hover:border-mk-accent-200
                               focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  >
                    <span className="block text-mk-body font-semibold text-mk-ink">{spec.title}</span>
                    <span className="mt-1 block text-mk-small leading-[1.7] text-mk-muted">
                      {opt ? `你选了「${opt.name}」` : null}
                      {st.notes.length > 0 ? `你提了 ${st.notes.length} 条意见` : null}
                      {!opt && st.notes.length === 0 ? "已验收" : null}
                    </span>
                    {st.why ? (
                      <span className="mt-1 block border-l-2 border-mk-accent-200 pl-2 text-mk-small leading-[1.7] text-mk-secondary">
                        {st.why}
                      </span>
                    ) : null}
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

function Who() {
  return (
    <div className="space-y-4">
      <div
        className="rounded-mk-md border p-4"
        style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }}
      >
        <div className="flex items-center gap-1.5">
          <Info size={13} strokeWidth={2} className="text-mk-accent-700" />
          <Sys className="!text-mk-accent-700">{PBL_COVENANT.title}</Sys>
        </div>
        <p className="mt-2 text-mk-small font-semibold text-mk-ink">印记可以</p>
        <ul className="mt-1 space-y-1">
          {PBL_COVENANT.can.map((c) => (
            <li key={c} className="text-mk-small leading-[1.75] text-mk-secondary">
              · {c}
            </li>
          ))}
        </ul>
        <p className="mt-2.5 text-mk-small font-semibold text-mk-ink">这些必须是你的</p>
        <ul className="mt-1 space-y-1">
          {PBL_COVENANT.must.map((c) => (
            <li key={c} className="text-mk-small leading-[1.75] text-mk-secondary">
              · {c}
            </li>
          ))}
        </ul>
        <p className="mt-2.5 border-t border-mk-accent-200 pt-2.5 text-mk-small leading-[1.8] text-mk-ink">
          {PBL_COVENANT.line}
        </p>
      </div>

      {METHODS.map((m) => (
        <details key={m.id} className="rounded-mk-md border border-mk-border p-3.5">
          <summary className="cursor-pointer text-mk-body font-semibold text-mk-ink">
            {m.label}
          </summary>
          <p className="mt-1 text-mk-small text-mk-muted">{m.from}</p>
          <ul className="mt-2 space-y-1">
            {m.points.map((pt) => (
              <li key={pt} className="text-mk-small leading-[1.75] text-mk-secondary">
                · {pt}
              </li>
            ))}
          </ul>
          <p className="mt-2 text-mk-small leading-[1.75] text-mk-faint">{m.why}</p>
        </details>
      ))}
    </div>
  );
}

/**
 * One line of what she actually put in a finished card.
 *
 * Generic on purpose — it reads the entry, not the card id, so a new card
 * needs no change here. Rows win over prose because a row count is the most
 * honest single number a card can report.
 */
function trace(entry: CardEntry): string {
  for (const v of Object.values(entry.values)) {
    const rows = filledRows(asRows(v));
    if (rows.length) return `${rows.length} 条`;
  }
  for (const v of Object.values(entry.values)) {
    const t = asText(v).trim();
    if (t.length > 4) return t.length > 30 ? `${t.slice(0, 30)}…` : t;
  }
  return "已提交";
}
