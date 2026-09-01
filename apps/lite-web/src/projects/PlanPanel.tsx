import { useState } from "react";
import {
  PLAN_RESOLUTIONS,
  PLAN_RESOLUTION_LABELS,
  STEP_STATUS_LABELS,
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
}: {
  plan: Plan | null;
  pending: PendingChange[];
  onResolve: (id: string, resolution: PlanResolution, reason: string) => Promise<void>;
  onApprove: (versionId: string) => Promise<void>;
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
          还没有计划。先把要做的事聊清楚，印记会拟一版给你看。
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
      </header>

      <div className="flex-1 overflow-y-auto px-5 py-4">
        <ol className="flex flex-col gap-3">
          {plan.steps.map((s) => (
            <StepRow key={s.id} step={s} />
          ))}
        </ol>
      </div>

      {unapproved && (
        // Nothing runs before she has read it and said so.
        <footer className="border-t border-mk-border px-5 py-4">
          <p className="text-mk-small text-mk-secondary">看一遍。改哪儿都行，你说了算。</p>
          <button
            type="button"
            onClick={() => void onApprove(plan.versionId)}
            className="mt-3 w-full rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white"
            style={{ background: "var(--mk-accent-500)" }}
          >
            准备好了 · 开始
          </button>
        </footer>
      )}
    </div>
  );
}

function StepRow({ step }: { step: PlanStep }) {
  const [open, setOpen] = useState(false);
  // The two waiting states are the reason the vocabulary has seven entries:
  // they let the plan say WHY nothing is moving. They must not look like the
  // others.
  const waiting = WAITING_STATUSES.includes(step.status);
  const done = step.status === "done";

  return (
    <li className="rounded-mk-md border border-mk-border bg-mk-surface">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-start gap-3 px-3 py-2.5 text-left"
      >
        <span className="mt-0.5 text-mk-label text-mk-faint">{step.ordinal}</span>
        <span className="flex-1">
          <span className={done ? "text-mk-body text-mk-muted line-through" : "text-mk-body text-mk-ink"}>
            {step.title}
          </span>
          <span
            className="ml-2 rounded-mk-full px-1.5 py-0.5 text-mk-label"
            style={
              waiting
                ? { background: "var(--mk-warning-bg)", color: "var(--mk-warning)" }
                : { background: "var(--mk-paper)", color: "var(--mk-muted)" }
            }
          >
            {STEP_STATUS_LABELS[step.status] ?? step.status}
          </span>
        </span>
      </button>

      {open && (
        <div className="border-t border-mk-border px-3 py-3 text-mk-small">
          {step.goal && <Line label="这一步要干嘛" value={step.goal} />}
          {step.iBring && <Line label="印记做" value={step.iBring} />}
          {step.youBring && <Line label="你做" value={step.youBring} />}
          {/* Always shown, never optional: a step where she decides nothing is
              a step she should not sit through. */}
          <Line label="你来判断" value={step.decide} strong />
          {step.thenBring && <Line label="做完交回来" value={step.thenBring} />}
        </div>
      )}
    </li>
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
    } catch {
      setError("没保存上，再试一次。");
      setSaving(false);
    }
  }

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-mk-border px-5 py-4">
        <h2 className="text-mk-body font-semibold text-mk-ink">要不要改计划</h2>
        <p className="mt-1 text-mk-small text-mk-secondary">{change.evidence}</p>
      </header>

      <div className="flex-1 overflow-y-auto px-5 py-4">
        <p className="text-mk-label uppercase text-mk-faint">印记想改的</p>
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

        <p className="mt-6 text-mk-label uppercase text-mk-faint">你怎么定</p>
        <div className="mt-2 flex flex-col gap-1.5">
          {PLAN_RESOLUTIONS.map((r) => (
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
          placeholder="为什么这么定？"
          className="mt-4 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
        {error && (
          <p className="mt-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {error}
          </p>
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
          {saving ? "记下来…" : "就这么定"}
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
      return "删掉";
    case "modify":
      return "改成";
    case "defer":
      return "先放着";
    default:
      return k;
  }
}
