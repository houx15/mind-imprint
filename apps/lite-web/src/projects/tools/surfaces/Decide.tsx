import { useCallback, useEffect, useState } from "react";
import { Check } from "lucide-react";
import { Icon } from "@/ui";
import { apiErrorText } from "../../../api/errorText";
import {
  decisionTodo,
  listDecisions,
  openDecisionOf,
  settleDecision,
  type Decision,
} from "../../../api/decide";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Decide —— 理性决策。
 *
 * 产品负责人 2026-09-02：
 *   「it happens when AI proposes several thing to decide... present several
 *    options as cards, with title, description, and students can select one to
 *    confirm. but when confirm, they need to answer two small questions:
 *    why this, and why not others.」
 *
 * 所以这一屏只做三件事：把印记提的几条路摆成卡片、让她选一张、问两个小问题。
 *
 * 🚨 第二个问题（为什么不选别的）才是这件工具真正教的东西。选中一个不难——
 * 顺眼就点了；说得出为什么放掉另外两条，才说明她真的把它们放在一起比过。
 *
 * 上一版是我自己设计的：她加选项、定标准、给每个选项填「赢在哪疼在哪」。那既是
 * 让她替印记把活干了，也把一个当场的判断拖成了一张表格。已废弃。
 */
export function Decide({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [decision, setDecision] = useState<Decision | null>(null);
  const [ready, setReady] = useState(false);
  const [choice, setChoice] = useState("");
  const [why, setWhy] = useState("");
  const [whyNot, setWhyNot] = useState("");
  const [error, setError] = useState<string | null>(null);

  const boot = useCallback(async () => {
    try {
      setDecision(openDecisionOf(await listDecisions(projectId)));
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setReady(true);
    }
  }, [projectId]);

  useEffect(() => {
    void boot();
  }, [boot]);

  async function finish() {
    if (!decision) return;
    try {
      const got = await settleDecision(projectId, decision.id, {
        choice: choice.trim(),
        why: why.trim(),
        whyNot: whyNot.trim(),
      });
      onFinish({ decisionId: got.id, choice: got.choice }, got.why);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const todo = decisionTodo(decision, { choice, why, whyNot });

  return (
    <ToolFrame
      title={tool.label}
      task="针对每个选项的原因及可能后果进行深入思考，再做出决定"
      why={tool.reason}
      todo={todo}
      finishLabel="确认选择"
      onFinish={() => void finish()}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {ready && !decision && (
        <p className="text-mk-small text-mk-muted">
          暂时没有需要决策的内容。印记提出几个方案时，会在这里让你选。
        </p>
      )}

      {decision && (
        <>
          <p className="text-mk-body text-mk-ink">{decision.subject}</p>

          <div className="mt-3 space-y-2">
            {decision.options.map((o) => {
              const on = choice === o.label;
              return (
                <button
                  key={o.id}
                  type="button"
                  onClick={() => setChoice(on ? "" : o.label)}
                  className="block w-full rounded-mk-md border px-3 py-2.5 text-left"
                  style={{
                    borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
                    background: on
                      ? "color-mix(in srgb, var(--mk-accent-500) 8%, transparent)"
                      : "transparent",
                  }}
                >
                  <span className="flex items-start justify-between gap-2">
                    <span className="text-mk-small font-semibold text-mk-ink">{o.label}</span>
                    {on && (
                      <span className="mt-0.5 shrink-0" style={{ color: "var(--mk-accent-500)" }}>
                        <Icon icon={Check} size={15} />
                      </span>
                    )}
                  </span>
                  {o.description && (
                    <span className="mt-1 block text-mk-small text-mk-secondary">
                      {o.description}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {/* 两个小问题。选完才出现——先比较，再解释。 */}
          {choice && (
            <div className="mt-4 space-y-3 border-t border-mk-border pt-3">
              <div>
                <label className="text-mk-small text-mk-secondary">为什么选它</label>
                <textarea
                  value={why}
                  onChange={(e) => setWhy(e.target.value)}
                  rows={2}
                  placeholder="它解决了什么，或者它比别的好在哪里"
                  className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
              </div>
              <div>
                <label className="text-mk-small text-mk-secondary">为什么不选别的</label>
                <textarea
                  value={whyNot}
                  onChange={(e) => setWhyNot(e.target.value)}
                  rows={2}
                  placeholder="另外几个方案，你分别是因为什么放掉的"
                  className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
              </div>
            </div>
          )}
        </>
      )}
    </ToolFrame>
  );
}
