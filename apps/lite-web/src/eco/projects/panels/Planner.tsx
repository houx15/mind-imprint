import { useState } from "react";
import { ArrowDown, ArrowUp, Check, Plus, RotateCcw, X } from "lucide-react";
import { useEco } from "../../store";
import { activeSteps } from "../../data/plan";
import { cardById } from "../../data/cards";
import { artifactById } from "../../data/artifacts";
import type { PlanStep, Project } from "../../data/types";
import { Btn, Sys, cx } from "../../ui";

/**
 * 路线 — the plan, before anything runs.
 *
 * ## Why a graph and not a list
 * A list of seven bullets reads as instructions. A timeline with big nodes
 * reads as a *route*, which is the thing she is being asked to agree with —
 * and it makes two facts visible that a list hides: how long the whole thing
 * is, and where the weight sits. On this journey one step (印记 开工) eats
 * most of the calendar and none of her attention, and four short ones eat all
 * of her attention and almost no calendar. Seeing that shape is most of what
 * reviewing a plan is for.
 *
 * ## What she can actually do here
 * Change any step's words, turn one off, reorder, add one of her own — and
 * write the timing. Nothing runs until she presses 开工.
 *
 * 🚨 **印记 does not pre-fill `when`.** The quick chips are a keyboard, not a
 * suggestion: they type a word she picked. A schedule she did not write is a
 * schedule she will not keep, and — the part that matters more — writing it
 * is the moment a student discovers her plan has seven steps and two free
 * afternoons in it. Taking that discovery away to save her thirty seconds of
 * typing would be the worst trade in this file.
 */
export function Planner({ project }: { project: Project }) {
  const { planEdit, planToggle, planMove, planAdd, approvePlan } = useEco();
  const [editing, setEditing] = useState<string | null>(null);

  const steps = activeSteps(project.plan);
  const timed = steps.filter((s) => s.when.trim().length > 0).length;
  const ready = steps.length > 0 && timed === steps.length;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="border-b border-mk-border px-6 py-4">
        <Sys>路线 · 印记提的，你说了算</Sys>
        <h2 className="mt-1 text-mk-h1 text-mk-ink">这条路我打算这么走</h2>
        <p className="mt-1.5 max-w-[70ch] text-mk-body leading-[1.85] text-mk-secondary">
          每一步都写了三件事：<span className="text-mk-ink">你带来什么</span>、
          <span className="text-mk-ink">我做什么</span>、
          <span className="text-mk-ink">哪个判断是你的</span>。
          不同意就改，多余的关掉，缺的加一步。
        </p>
      </div>

      {/* ── the journey ─────────────────────────────────────────────── */}
      <div className="min-h-0 flex-1 overflow-auto px-6 py-6">
        <div className="relative flex min-w-max gap-4 pb-2">
          {/* the connector the nodes sit on */}
          <span
            aria-hidden
            className="absolute left-0 right-0 top-[34px] h-[2px]"
            style={{
              background:
                "repeating-linear-gradient(90deg,var(--mk-border) 0 10px,transparent 10px 16px)",
            }}
          />
          {project.plan.map((step, i) => (
            <Node
              key={step.id}
              step={step}
              index={project.plan.filter((s, k) => !s.off && k < i).length}
              editing={editing === step.id}
              onEdit={() => setEditing(editing === step.id ? null : step.id)}
              onPatch={(p) => planEdit(project.id, step.id, p)}
              onToggle={() => planToggle(project.id, step.id)}
              onMove={(d) => planMove(project.id, step.id, d)}
            />
          ))}

          <button
            type="button"
            onClick={() => planAdd(project.id)}
            className="mt-[22px] flex h-[190px] w-[130px] shrink-0 flex-col items-center justify-center gap-2
                       rounded-mk-lg border border-dashed border-mk-border text-mk-muted
                       transition-colors duration-[140ms] hover:border-mk-accent hover:text-mk-accent-700
                       focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            <Plus size={20} strokeWidth={1.8} />
            <span className="text-mk-small">加一步</span>
            <span className="px-3 text-center text-[11px] leading-[1.6] text-mk-faint">
              我想到的不一定全
            </span>
          </button>
        </div>
      </div>

      {/* ── approve ─────────────────────────────────────────────────── */}
      <div className="border-t border-mk-border bg-mk-surface px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <p className="text-mk-body text-mk-ink">
              <span className="font-mono tabular-nums">{steps.length}</span> 步 ·
              时间已排{" "}
              <span
                className={cx(
                  "font-mono tabular-nums",
                  ready ? "text-mk-success" : "text-mk-accent-700",
                )}
              >
                {timed}/{steps.length}
              </span>
            </p>
            <p className="mt-0.5 text-mk-small leading-[1.7] text-mk-muted">
              {ready
                ? "时间是你自己排的，所以它算数。"
                : "每一步都要有时间。我不替你排——你排的你才会当真。"}
            </p>
          </div>
          <Btn
            disabled={!ready}
            iconStart={<Check size={16} strokeWidth={2.4} />}
            onClick={() => approvePlan(project.id)}
          >
            这个计划我同意，开工
          </Btn>
        </div>
      </div>
    </div>
  );
}

/** Quick words for the `when` field. A keyboard, not a suggestion — see the
 *  file header on why 印记 never fills this in. */
const WHENS = ["本周", "下周", "第三周", "两天", "一个下午", "一个月"];

function Node({
  step,
  index,
  editing,
  onEdit,
  onPatch,
  onToggle,
  onMove,
}: {
  step: PlanStep;
  index: number;
  editing: boolean;
  onEdit: () => void;
  onPatch: (p: Partial<PlanStep>) => void;
  onToggle: () => void;
  onMove: (d: -1 | 1) => void;
}) {
  const opens = step.opens;
  const what =
    opens?.kind === "card"
      ? cardById(opens.cardId)?.title
      : opens?.kind === "make"
        ? artifactById(opens.artifactId)?.title
        : null;
  const mineOnly = !opens || opens.kind === "card";

  return (
    <div className="relative w-[262px] shrink-0">
      {/* the node on the line */}
      <div className="flex h-[68px] items-start justify-center">
        <span
          className={cx(
            "flex h-[68px] w-[68px] items-center justify-center rounded-mk-full font-mono text-[19px] font-bold",
            step.off ? "text-mk-faint" : "text-white",
          )}
          style={{
            background: step.off
              ? "var(--mk-paper)"
              : mineOnly
                ? "linear-gradient(150deg,var(--mk-accent-400),var(--mk-accent-600))"
                : "linear-gradient(150deg,#7FA8C9,#4E7EA6)",
            border: step.off ? "2px dashed var(--mk-border)" : "3px solid var(--mk-surface)",
            boxShadow: step.off ? "none" : "0 2px 10px rgba(0,0,0,.12)",
          }}
        >
          {step.off ? "—" : String(index + 1).padStart(2, "0")}
        </span>
      </div>

      <div
        className={cx(
          "mt-2 rounded-mk-lg border p-4 transition-colors duration-[140ms]",
          step.off ? "border-dashed border-mk-border bg-transparent" : "border-mk-border bg-mk-surface",
        )}
        style={{ opacity: step.off ? 0.5 : 1 }}
      >
        {/* controls */}
        <div className="mb-2 flex items-center justify-end gap-0.5">
          <IconBtn label="上移" onClick={() => onMove(-1)}>
            <ArrowUp size={13} strokeWidth={2} />
          </IconBtn>
          <IconBtn label="下移" onClick={() => onMove(1)}>
            <ArrowDown size={13} strokeWidth={2} />
          </IconBtn>
          <IconBtn label={step.off ? "加回来" : "这一步不要"} onClick={onToggle}>
            {step.off ? <RotateCcw size={13} strokeWidth={2} /> : <X size={13} strokeWidth={2} />}
          </IconBtn>
        </div>

        {editing ? (
          <>
            <input
              value={step.title}
              onChange={(e) => onPatch({ title: e.target.value })}
              className="w-full rounded-mk-md border border-mk-input-border bg-mk-paper px-2.5 py-1.5
                         text-mk-body font-semibold text-mk-ink focus:border-mk-accent focus:outline-none"
            />
            <textarea
              value={step.blurb}
              onChange={(e) => onPatch({ blurb: e.target.value })}
              rows={3}
              className="mt-2 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-paper
                         px-2.5 py-1.5 text-mk-small leading-[1.7] text-mk-ink
                         focus:border-mk-accent focus:outline-none"
            />
          </>
        ) : (
          <>
            <button
              type="button"
              onClick={onEdit}
              className="block w-full text-left text-mk-body font-semibold leading-[1.5] text-mk-ink
                         hover:text-mk-accent-700 focus-visible:outline-none"
              title="点一下可以改"
            >
              {step.title}
            </button>
            <p className="mt-1 text-mk-small leading-[1.7] text-mk-secondary">{step.blurb}</p>
          </>
        )}

        <dl className="mt-3 space-y-1.5 border-t border-mk-border pt-2.5">
          <Row k="你带来" v={step.youBring} />
          <Row k="印记做" v={step.iBring} tone="ai" />
          <Row k="你判断" v={step.decide} tone="hers" />
        </dl>

        {what ? (
          <p className="mt-2.5 text-[11px] text-mk-faint">这一步打开：{what}</p>
        ) : null}

        {/* her timeline */}
        {!step.off ? (
          <div className="mt-3 border-t border-mk-border pt-2.5">
            <label className="eco-mono block text-mk-faint" htmlFor={`when-${step.id}`}>
              你打算什么时候
            </label>
            <input
              id={`when-${step.id}`}
              value={step.when}
              onChange={(e) => onPatch({ when: e.target.value })}
              placeholder="自己写"
              className="mt-1 w-full rounded-mk-md border border-mk-input-border bg-mk-paper px-2.5 py-1.5
                         text-mk-small text-mk-ink placeholder:text-mk-faint
                         focus:border-mk-accent focus:outline-none"
            />
            <div className="mt-1.5 flex flex-wrap gap-1">
              {WHENS.map((w) => (
                <button
                  key={w}
                  type="button"
                  onClick={() => onPatch({ when: w })}
                  className="rounded-mk-full border border-mk-border px-2 py-0.5 text-[11px] text-mk-muted
                             transition-colors duration-[120ms] hover:border-mk-accent hover:text-mk-accent-700
                             focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                >
                  {w}
                </button>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function Row({ k, v, tone }: { k: string; v: string; tone?: "ai" | "hers" }) {
  if (!v.trim()) return null;
  return (
    <div className="flex gap-2">
      <dt
        className="eco-mono w-[38px] shrink-0"
        style={{
          color:
            tone === "ai"
              ? "#4E7EA6"
              : tone === "hers"
                ? "var(--mk-accent-700)"
                : "var(--mk-faint)",
        }}
      >
        {k}
      </dt>
      <dd className="min-w-0 flex-1 text-[12px] leading-[1.65] text-mk-secondary">{v}</dd>
    </div>
  );
}

function IconBtn({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="flex h-6 w-6 items-center justify-center rounded-mk-sm text-mk-faint
                 transition-colors duration-[120ms] hover:bg-mk-paper hover:text-mk-ink
                 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      {children}
    </button>
  );
}
