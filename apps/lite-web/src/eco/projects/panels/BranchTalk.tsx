import { useEffect, useRef, useState } from "react";
import { ArrowUp, CornerUpLeft } from "lucide-react";
import { useEco } from "../../store";
import { BRANCHES } from "../../data/plan";
import type { Project } from "../../data/types";
import { Bold, Btn, Field, Sys, cx } from "../../ui";

/**
 * 支线 — a side conversation about ONE road.
 *
 * ## Why a branch and not just more chat
 * Because the question 「要贴多少个点」 belongs to one option and nothing
 * else. Asked in the main thread it drags the whole project into an option
 * she has not chosen; asked here it stays where it belongs, and the main
 * conversation still reads as a project rather than as five arguments
 * interleaved.
 *
 * The branch also gives the digging a SHAPE: it ends with 「我带回去的判断」.
 * Without that field, opening an option is reading. With it, opening an option
 * is an errand she came back from — which is what makes the decision on the
 * next screen hers instead of the last thing she happened to read.
 *
 * 🚨 The scripted answers are deliberately honest about their limits. Several
 * of them say *go ask a real person, my guess is worth less than their
 * answer*, because on questions about a specific garden and a specific
 * property manager that is simply true. See the
 * `ai-errors-must-surface-never-fake` rule: a confident invented answer here
 * would be the worst thing this screen could do.
 */
export function BranchTalk({
  project,
  approachId,
  onBack,
}: {
  project: Project;
  approachId: string;
  onBack: () => void;
}) {
  const { askInBranch, setTakeaway } = useEco();
  const [draft, setDraft] = useState("");
  const endRef = useRef<HTMLDivElement>(null);

  const approach = project.approaches.find((a) => a.id === approachId);
  const branch = project.branches.find((b) => b.approachId === approachId);
  const script = BRANCHES[approachId];
  const asked = branch?.log.filter((t) => t.role === "student").length ?? 0;

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [branch?.log.length]);

  if (!approach || !script) {
    return <p className="p-6 text-mk-body text-mk-muted">这条路还没写好。</p>;
  }

  const unasked = script.asks.filter(
    (a) => !branch?.log.some((t) => t.role === "student" && t.text === a.q),
  );

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="border-b border-mk-border px-6 py-4">
        <Btn variant="quiet" size="sm" iconStart={<CornerUpLeft size={14} />} onClick={onBack}>
          回到两条路
        </Btn>
        <div className="mt-2 flex items-center gap-2">
          <span className="h-2.5 w-2.5 rounded-mk-full" style={{ background: approach.hue }} />
          <Sys>只谈这一条 · 谈完不用非选它</Sys>
        </div>
        <h2 className="mt-1 text-mk-h1 text-mk-ink">{approach.name}</h2>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
        <ol className="mx-auto max-w-[640px] space-y-4">
          <li>
            <Bubble role="coach" text={script.opener} />
          </li>
          {branch?.log.map((t) => (
            <li key={t.id}>
              <Bubble role={t.role} text={t.text} />
            </li>
          ))}
          {asked >= 2 && !branch?.takeaway ? (
            <li>
              <Bubble role="coach" text={script.nudge} />
            </li>
          ) : null}
        </ol>

        {/* the point of the branch */}
        <div className="mx-auto mt-6 max-w-[640px] rounded-mk-lg border border-mk-border bg-mk-surface p-5">
          <Field
            label="我带回去的判断"
            hint="一句话。不是「这条路不错」，是你到底看明白了它的什么。写了它才会出现在选路那一页上。"
            value={branch?.takeaway ?? ""}
            onChange={(v) => setTakeaway(project.id, approachId, v)}
            rows={2}
            placeholder="例如：这条路的死穴不是技术，是三个月后还有没有人补二维码。"
          />
          {branch?.takeaway.trim() ? (
            <Btn className="mt-3" variant="outline" onClick={onBack}>
              带着这句回去
            </Btn>
          ) : null}
        </div>
      </div>

      {/* asks + composer */}
      <div className="border-t border-mk-border bg-mk-surface px-6 py-4">
        {unasked.length > 0 ? (
          <div className="mx-auto mb-2.5 flex max-w-[640px] flex-wrap gap-1.5">
            {unasked.slice(0, 3).map((a) => (
              <button
                key={a.q}
                type="button"
                onClick={() => askInBranch(project.id, approachId, a.q)}
                className="rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary
                           transition-colors duration-[120ms] hover:border-mk-accent hover:text-mk-accent-700
                           focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                {a.q}
              </button>
            ))}
          </div>
        ) : null}
        <form
          className="mx-auto flex max-w-[640px] items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const text = draft.trim();
            if (!text) return;
            askInBranch(project.id, approachId, text);
            setDraft("");
          }}
        >
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="也可以自己问"
            className="h-[44px] flex-1 rounded-mk-lg border border-mk-input-border bg-mk-paper px-3.5
                       text-mk-body text-mk-ink placeholder:text-mk-faint
                       focus:border-mk-accent focus:outline-none focus:ring-2 focus:ring-mk-accent-200"
          />
          <Btn type="submit" disabled={!draft.trim()} iconStart={<ArrowUp size={16} />}>
            问
          </Btn>
        </form>
      </div>
    </div>
  );
}

function Bubble({ role, text }: { role: "coach" | "student"; text: string }) {
  const coach = role === "coach";
  return (
    <div className={cx("flex gap-3", coach ? "" : "flex-row-reverse")}>
      <span
        className={cx(
          "mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold",
          coach ? "text-white" : "text-mk-secondary",
        )}
        style={{
          background: coach
            ? "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))"
            : "var(--mk-border)",
        }}
      >
        {coach ? "印" : "我"}
      </span>
      <div
        className="max-w-[86%] whitespace-pre-wrap rounded-mk-lg px-4 py-3 text-mk-body-lg leading-[1.9] text-mk-ink"
        style={{
          background: coach ? "var(--mk-surface)" : "var(--mk-accent-50)",
          border: `1px solid ${coach ? "var(--mk-border)" : "var(--mk-accent-200)"}`,
        }}
      >
        <Bold text={text} />
      </div>
    </div>
  );
}
