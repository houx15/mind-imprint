import { Says, errorMarkdown } from "./Says";
import { executionStatus, executionLabel } from "./stepProgress";
import { ProjectProgressVisual } from "./ProjectProgressVisual";
import { apiErrorText } from "../api/errorText";
import { useEffect, useState } from "react";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import { beforeNavigate } from "../routing";
import {
  PLAN_DECISIONS,
  PLAN_RESOLUTION_LABELS,
  WAITING_STATUSES,
  type PendingChange,
  type Plan,
  type PlanResolution,
  type PlanStep,
} from "../api/projectRoom";

/**
 * PlanPanel — the living tasklist, and Plan Check when something is waiting.
 *
 * 印记 revises this list as the project teaches them both something. What it
 * cannot do is change what the project IS without her: a structural change
 * arrives here as a `PendingChange` and the panel becomes a decision surface
 * until she has answered it.
 */
export function PlanPanel({
  plan,
  pending,
  onResolve,
  onApprove,
  onSubmit,
  onDiscuss,
  busy = false,
}: {
  plan: Plan | null;
  pending: PendingChange[];
  onResolve: (id: string, resolution: PlanResolution, reason: string) => Promise<void>;
  onApprove: (versionId: string) => Promise<void>;
  onSubmit: (stepId: string, note: string, url: string) => Promise<void>;
  onDiscuss: (step: PlanStep) => void;
  busy?: boolean;
}) {
  // Plan Check takes the panel over rather than popping up. A popup says
  // "dismiss me"; this is a decision, and it should read like one.
  const open = pending[0];
  if (open) return <PlanCheck change={open} onResolve={onResolve} />;

  if (!plan) {
    // An empty panel still needs its header, or the right-hand column reads as
    // an unfinished region rather than an empty one.
    return (
      <div className="flex h-full flex-col">
        <header className="border-b border-mk-border px-5 py-4">
          <h2 className="text-mk-body font-semibold text-mk-ink">计划</h2>
        </header>
        <p className="px-5 py-4 text-mk-small text-mk-muted">
          计划待生成
        </p>
      </div>
    );
  }

  const unapproved = plan.approvedAt === null;

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-mk-border px-5 py-4">
        <div className="flex items-baseline justify-between">
          <h2 className="text-mk-body font-semibold text-mk-ink">计划</h2>
          <span className="text-mk-label text-mk-faint">v0.{plan.version}</span>
        </div>
        {plan.summary && <p className="mt-1 text-mk-small text-mk-secondary">{plan.summary}</p>}
        {plan.reason && <p className="mt-3 text-mk-small text-mk-secondary"><span className="font-semibold">{plan.version > 1 ? "调整原因" : "安排依据"}：</span>{plan.reason}</p>}
      </header>

      <div className="flex-1 overflow-y-auto px-5 py-4">
        <ProjectProgressVisual steps={plan.steps} showSteps={false} />
        <ol className="project-plan-steps flex flex-col gap-3">
          {plan.steps.map((s) => (
            <StepRow key={s.id} step={s} approved={!unapproved} busy={busy} onSubmit={onSubmit} onDiscuss={onDiscuss} />
          ))}
        </ol>
      </div>

      {unapproved && (
        // Nothing runs before she has read it and said so.
        <footer className="border-t border-mk-border px-5 py-4">
          <p className="text-mk-small text-mk-secondary">请审核计划并确认，或提出修改意见</p>
          <button
            type="button"
            disabled={busy}
            onClick={() => void onApprove(plan.versionId)}
            className="mt-3 w-full rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            {busy ? "处理中…" : "确认计划"}
          </button>
        </footer>
      )}
    </div>
  );
}

function StepRow({ step, approved, busy, onSubmit, onDiscuss }: { step: PlanStep; approved: boolean; busy: boolean; onSubmit: (stepId: string, note: string, url: string) => Promise<void>; onDiscuss: (step: PlanStep) => void }) {
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState(step.submission?.note ?? "");
  const [url, setURL] = useState(step.submission?.url ?? "");
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const dirty = note !== (step.submission?.note ?? "") || url !== (step.submission?.url ?? "");
  useEffect(() => {
    if (!dirty) return;
    return beforeNavigate(async () => {
      const message = "请先提交或清除当前产出草稿，再离开项目。";
      setSubmitError(message);
      throw new Error(message);
    });
  }, [dirty]);
  // The two waiting states are the reason the vocabulary has seven entries:
  // they let the plan say WHY nothing is moving. They must not look like the
  // others.
  const status = executionStatus(step);
  const waiting = status === "doing" || (!step.progress && WAITING_STATUSES.includes(step.status));
  const done = status === "done";

  return (
    <li data-status={status} className="rounded-mk-md border border-mk-border bg-mk-surface">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-start gap-3 px-3 py-2.5 text-left"
      >
        <span className="project-plan-marker">{done ? "✓" : step.ordinal}</span>
        <span className="flex-1">
          <span className={done ? "text-mk-body text-mk-muted line-through" : "text-mk-body text-mk-ink"}>
            {step.title}
          </span>
          <span
            className="ml-2 inline-block whitespace-nowrap rounded-mk-full px-1.5 py-0.5 text-mk-label"
            style={
              waiting
                ? { background: "var(--mk-warning-bg)", color: "var(--mk-warning)" }
                : { background: "var(--mk-paper)", color: "var(--mk-muted)" }
            }
          >
            {executionLabel(step)}
          </span>
        </span>
      </button>

      {open && (
        <div className="border-t border-mk-border px-3 py-3 text-mk-small">
          {step.blurb && <Line label="安排" value={step.blurb} />}
          {step.goal && <Line label="目标" value={step.goal} />}
          {/* 谁做这件事用人名标签表示，不用「印记做 / 你做」当行首标签——
              标签是给东西命名的，「印记做」是在替它讲话（AGENTS.md §界面文案 0）。 */}
          {step.iBring && <Owned who="印记" value={step.iBring} />}
          {step.youBring && <Owned who="你" value={step.youBring} />}
          {step.decide && <Line label="判断" value={step.decide} />}
          {step.thenBring && <Line label="产出" value={step.thenBring} />}
          {step.submission && (
            <div className="mt-3 rounded-mk-md bg-mk-paper p-3">
              <p className="text-mk-label uppercase text-mk-faint">已提交产出</p>
              {step.submission.note && <p className="mt-1 whitespace-pre-wrap text-mk-small text-mk-ink">{step.submission.note}</p>}
              {step.submission.url && <a className="mt-1 block break-all text-mk-small underline" href={step.submission.url} target="_blank" rel="noreferrer">打开链接</a>}
              <p className="mt-1 text-mk-label text-mk-muted">由你确认完成，尚未经审核</p>
              <button type="button" disabled={busy || submitting} onClick={() => onDiscuss(step)} className="mt-2 text-mk-small font-semibold text-mk-accent-500 disabled:opacity-40">与印记讨论</button>
            </div>
          )}
          {approved && step.status !== "cancelled" && (
            <div className="mt-3 border-t border-mk-border pt-3">
              <p className="text-mk-label uppercase text-mk-faint">{step.submission ? "更新产出" : "提交产出"}</p>
              <GrowingTextarea aria-label="产出说明" value={note} disabled={busy || submitting} onChange={(e) => setNote(e.target.value)} rows={2} placeholder="产出说明" className="mt-2 w-full rounded-mk-md border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-small" />
              <input aria-label="产出链接" value={url} disabled={busy || submitting} onChange={(e) => setURL(e.target.value)} placeholder="产出链接（可选）" className="mt-2 w-full rounded-mk-md border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-small" />
              <p className="mt-1 text-mk-label text-mk-muted">可以提交在外部编程 Agent 或其他工具中完成的内容，不需要导入或同步。</p>
              <label className="mt-2 flex items-start gap-2 text-mk-small text-mk-secondary"><input type="checkbox" disabled={busy || submitting} checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />我确认这是本步实际完成的产出</label>
              {submitError && <div role="alert" className="mt-2 text-mk-small" style={{ color: "var(--mk-danger)" }}><Says content={errorMarkdown(submitError)} /></div>}
              <button type="button" disabled={busy || submitting || !confirmed || (!note.trim() && !url.trim()) || !dirty} onClick={async () => {
                setSubmitting(true); setSubmitError(null);
                try { await onSubmit(step.id, note.trim(), url.trim()); }
                catch (err) { setSubmitError(apiErrorText(err)); }
                finally { setSubmitting(false); }
              }} className="mt-3 rounded-mk-full bg-mk-accent-500 px-4 py-2 text-mk-small font-semibold text-white disabled:opacity-40">{submitting ? "提交中" : step.submission ? "更新产出" : "提交产出"}</button>
            </div>
          )}
        </div>
      )}
    </li>
  );
}

/** 一行「谁 · 做什么」。谁用一个带色的名字标签，和分工建议里那套一致。 */
function Owned({ who, value }: { who: string; value: string }) {
  const mine = who === "你";
  return (
    <div className="mt-1.5 flex items-start gap-2">
      <span
        className="mt-0.5 shrink-0 rounded-mk-full px-2 py-0.5 text-mk-small"
        style={{
          background: mine ? "var(--mk-matcha-bg)" : "var(--mk-taro-bg)",
          color: "var(--mk-ink)",
        }}
      >
        {who}
      </span>
      <span className="min-w-0 text-mk-small text-mk-ink">{value}</span>
    </div>
  );
}

function Line({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <p className="mb-1.5 last:mb-0">
      <span className="text-mk-label uppercase text-mk-faint">{label}</span>{" "}
      <span className={strong ? "font-semibold text-mk-ink" : "text-mk-secondary"}>{value}</span>
    </p>
  );
}

/**
 * PlanCheck — 印记 wants to change what the project IS. She decides.
 *
 * Four things and nothing else: what changed, why, the diff, and what she has
 * to decide. No re-display of the whole plan — a long summary traded for a
 * click is how consent gets manufactured.
 */
function PlanCheck({
  change,
  onResolve,
}: {
  change: PendingChange;
  onResolve: (id: string, resolution: PlanResolution, reason: string) => Promise<void>;
}) {
  const [choice, setChoice] = useState<PlanResolution | null>(null);
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const diffLines = Object.entries(change.diff ?? {}).flatMap(([k, v]) =>
    (Array.isArray(v) ? v : [String(v)]).map((item) => ({ k, item: String(item) })),
  );

  async function submit() {
    if (!choice || !reason.trim() || saving) return;
    setSaving(true);
    setError(null);
    try {
      await onResolve(change.id, choice, reason.trim());
    } catch (err) {
      setError(apiErrorText(err));
      setSaving(false);
    }
  }

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-mk-border px-5 py-4">
        <h2 className="text-mk-body font-semibold text-mk-ink">计划调整</h2>
        <p className="mt-1 text-mk-small text-mk-secondary">{change.evidence}</p>
      </header>

      <div className="flex-1 overflow-y-auto px-5 py-4">
        <p className="text-mk-label uppercase text-mk-faint">修改建议</p>
        <ul className="mt-2 flex flex-col gap-1.5">
          {diffLines.map((d, i) => (
            <li key={i} className="text-mk-small">
              <span className="text-mk-label uppercase text-mk-muted">{diffLabel(d.k)}</span>{" "}
              <span className="text-mk-ink">{d.item}</span>
            </li>
          ))}
          {diffLines.length === 0 && (
            <li className="text-mk-small text-mk-muted">（没有列出具体差异）</li>
          )}
        </ul>

        <p className="mt-6 text-mk-label uppercase text-mk-faint">决策时刻</p>
        <div className="mt-2 flex flex-col gap-1.5">
          {PLAN_DECISIONS.map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setChoice(r)}
              className="rounded-mk-md border px-3 py-2 text-left text-mk-body"
              style={{
                borderColor: choice === r ? "var(--mk-ink)" : "var(--mk-border)",
                background: choice === r ? "var(--mk-paper)" : "transparent",
                color: "var(--mk-ink)",
              }}
            >
              {PLAN_RESOLUTION_LABELS[r]}
            </button>
          ))}
        </div>

        {/* 🚨 The reason gates the button. Every settle in this product costs
            her a sentence — the moment 「就用这个」 works on its own, this is a
            machine that generates and a student who approves. */}
        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={3}
          placeholder="原因"
          className="mt-4 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
        {error && (
          <div className="mt-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            <Says content={errorMarkdown(error)} />
          </div>
        )}
      </div>

      <footer className="border-t border-mk-border px-5 py-4">
        <button
          type="button"
          onClick={() => void submit()}
          disabled={!choice || !reason.trim() || saving}
          className="w-full rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          {/* 状态用「已/待/中」的成对词，不用大白话（文案第 4 条）。 */}
          {saving ? "处理中" : "确认选择"}
        </button>
      </footer>
    </div>
  );
}

function diffLabel(k: string): string {
  switch (k) {
    case "add":
      return "新增";
    case "remove":
      return "删除";
    case "modify":
      return "修改";
    case "defer":
      return "暂定";
    default:
      return k;
  }
}
