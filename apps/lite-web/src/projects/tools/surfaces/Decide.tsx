import { useCallback, useEffect, useState } from "react";
import { Check } from "lucide-react";
import { Icon } from "@/ui";
import { apiErrorText } from "../../../api/errorText";
import {
  addOption,
  decisionTodo,
  joinWhyNot,
  listDecisions,
  openDecisionOf,
  optionTone,
  optionTag,
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
  // 🚨 放掉的每一条分开答。一个大框只会得到「其他的都不太合适」，而
  // "为什么放掉另外那两条" 才是这件工具真正教的东西。
  const [dropped, setDropped] = useState<Record<string, string>>({});
  // 🚨 什么情况会让她改主意。`pbl_decision.flip` 这一列 0111 就加了，0113 把
  // 界面撤掉之后一直空着——而它是复盘阶段唯一能回头对照的东西：当初写下的那个
  // 条件，后来到底发生了没有。一个决定因此从一次表态变成一个可以被推翻的假设。
  //
  // 不设成门槛：一个决定不写翻盘条件也仍然是个决定，硬拦只会逼出一句应付的话。
  const [flip, setFlip] = useState("");
  // 她自己要加的那条路。
  const [adding, setAdding] = useState(false);
  const [mineLabel, setMineLabel] = useState("");
  const [mineWhy, setMineWhy] = useState("");
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

  const whyNot = decision
    ? joinWhyNot(
        decision.options
          .filter((o) => o.label !== choice)
          .map((o) => ({ label: o.label, why: dropped[o.id] ?? "" })),
      )
    : "";

  async function finish() {
    if (!decision) return;
    try {
      const got = await settleDecision(projectId, decision.id, {
        choice: choice.trim(),
        why: why.trim(),
        whyNot: whyNot.trim(),
        flip: flip.trim(),
      });
      onFinish({ decisionId: got.id, choice: got.choice }, got.why);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function addMine() {
    if (!decision || !mineLabel.trim()) return;
    try {
      setDecision(await addOption(projectId, decision.id, {
        label: mineLabel.trim(),
        description: mineWhy.trim(),
      }));
      setAdding(false);
      setMineLabel("");
      setMineWhy("");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const todo = decisionTodo(decision, { choice, why, whyNot });
  const chosenIndex = decision?.options.findIndex((o) => o.label === choice) ?? -1;
  const chosenTone = optionTone(chosenIndex < 0 ? 0 : chosenIndex);
  const answeredDrops = decision
    ? decision.options.filter((o) => o.label !== choice && (dropped[o.id] ?? "").trim() !== "")
        .length
    : 0;

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

          {/* 🚨 印记给的这几条不是全集。「在别人摆好的选项里挑一个」和「决定」
              是两回事——后者包含「这些都不对，我要的是另一样」。 */}
          {!choice &&
            (adding ? (
              <div className="mt-2 rounded-mk-md border border-dashed border-mk-border px-3 py-2.5">
                <input
                  autoFocus
                  value={mineLabel}
                  onChange={(e) => setMineLabel(e.target.value)}
                  placeholder="这条路叫什么"
                  className="w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
                <input
                  value={mineWhy}
                  onChange={(e) => setMineWhy(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && void addMine()}
                  placeholder="它意味着什么"
                  className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
                <div className="mt-2 flex gap-2">
                  <button
                    type="button"
                    onClick={() => void addMine()}
                    disabled={!mineLabel.trim()}
                    className="rounded-mk-full px-3 py-1 text-mk-small disabled:opacity-40"
                    style={{ background: "var(--mk-accent-500)", color: "var(--mk-surface)" }}
                  >
                    添加方案
                  </button>
                  <button
                    type="button"
                    onClick={() => setAdding(false)}
                    className="text-mk-small text-mk-secondary"
                  >
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setAdding(true)}
                className="mt-2 w-full rounded-mk-md border border-dashed border-mk-border py-2 text-mk-small text-mk-secondary"
              >
                都不合适，我有别的方案
              </button>
            ))}

          <p className="mt-1 text-mk-small text-mk-muted">
            {choice ? "请说明未选择其他方案的原因。" : "请选择一个方案。"}
          </p>

          <div className="mt-3 space-y-2">
            {decision.options.map((o, i) => {
              const on = choice === o.label;
              const t = optionTone(i);
              // 选中一张之后，别的暗下去——她要看见的是"我挑了这条，放掉了那些"。
              const faded = choice !== "" && !on;
              return (
                <button
                  key={o.id}
                  type="button"
                  onClick={() => setChoice(on ? "" : o.label)}
                  className="block w-full rounded-mk-md border px-3 py-2.5 text-left transition-opacity"
                  // 🚨 整张卡染上这条路自己的淡底，不挂左侧色条。
                  style={{
                    borderColor: on ? t.solid : "transparent",
                    background: t.bg,
                    opacity: faded ? 0.5 : 1,
                    boxShadow: on ? `0 0 0 2px color-mix(in srgb, ${t.solid} 32%, transparent)` : undefined,
                  }}
                >
                  <span className="flex items-start gap-2">
                    <span
                      className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-semibold"
                      style={{ background: t.solid, color: "var(--mk-surface)" }}
                    >
                      {optionTag(i)}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-mk-small font-semibold" style={{ color: t.fg }}>
                        {o.label}
                      </span>
                      {o.description && (
                        <span className="mt-1 block text-mk-small text-mk-secondary">
                          {o.description}
                        </span>
                      )}
                    </span>
                    {on && (
                      <span className="mt-0.5 shrink-0" style={{ color: t.solid }}>
                        <Icon icon={Check} size={15} />
                      </span>
                    )}
                  </span>
                </button>
              );
            })}
          </div>

          {/* 两个小问题。选完才出现——先比较，再解释。 */}
          {choice && (
            <div className="mt-5 space-y-4 border-t border-mk-border pt-4">
              <div>
                <div className="flex items-center gap-2">
                  <span
                    className="h-3.5 w-1 rounded-mk-full"
                    style={{ background: chosenTone.solid }}
                  />
                  <label className="text-mk-body font-semibold text-mk-ink">选择原因</label>
                </div>
                <textarea
                  value={why}
                  onChange={(e) => setWhy(e.target.value)}
                  rows={2}
                  placeholder="请说明它解决了什么，或它比其他方案好在哪里"
                  className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <span
                    className="h-3.5 w-1 rounded-mk-full"
                    style={{ background: "var(--mk-warning)" }}
                  />
                  <label className="text-mk-body font-semibold text-mk-ink">改主意的条件</label>
                  <span className="text-mk-small text-mk-faint">选填</span>
                </div>
                <p className="mt-0.5 text-mk-small text-mk-muted">
                  出现什么情况，你会回来改这个决定。复盘时会拿它对照。
                </p>
                <input
                  value={flip}
                  onChange={(e) => setFlip(e.target.value)}
                  placeholder="例如：接龙发出去两天还不到 5 个人报名"
                  className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
              </div>

              {/* 🚨 放掉的每一条各答一次，而不是一个大框。
                  一个大框得到的是「其他的都不太合适」；一张一张摆在面前，她才会
                  真的想起每一条当初为什么看起来可行。 */}
              <div>
                <div className="flex items-center gap-2">
                  <span className="h-3.5 w-1 rounded-mk-full" style={{ background: "var(--mk-border)" }} />
                  <label className="text-mk-body font-semibold text-mk-ink">
                    未选方案（{answeredDrops}/{decision.options.length - 1}）
                  </label>
                </div>
                <p className="mt-0.5 text-mk-small text-mk-muted">
                  请逐项说明放弃该方案的原因。
                </p>
                <div className="mt-2 space-y-2">
                  {decision.options.map((o, i) =>
                    o.label === choice ? null : (
                      <div
                        key={o.id}
                        className="rounded-mk-md border px-3 py-2"
                        style={{ borderColor: "transparent", background: optionTone(i).bg }}
                      >
                        <div className="flex items-center gap-2">
                          <span
                            className="flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-semibold"
                            style={{ background: optionTone(i).solid, color: "var(--mk-surface)" }}
                          >
                            {optionTag(i)}
                          </span>
                          <span className="text-mk-small" style={{ color: optionTone(i).fg }}>
                            {o.label}
                          </span>
                        </div>
                        <input
                          value={dropped[o.id] ?? ""}
                          onChange={(e) =>
                            setDropped((prev) => ({ ...prev, [o.id]: e.target.value }))
                          }
                          placeholder="放弃原因"
                          className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                        />
                      </div>
                    ),
                  )}
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </ToolFrame>
  );
}
