import { useCallback, useEffect, useState } from "react";
import { getMe } from "../../../api/auth";
import { apiErrorText } from "../../../api/errorText";
import { getPlan, type PlanStep } from "../../../api/projectRoom";
import {
  addSubstep,
  confirmSubstep,
  effectiveOwner,
  listSubsteps,
  reassign,
  shareOfWork,
  type Owner,
  type Substep,
} from "../../../api/split";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Split —— 分工建议。
 *
 * 产品负责人 2026-09-02：
 *   「it is something like a task list, left side is task name, second column
 *    is name tags (AI/student's name). third column is why. student can
 *    confirm, or chat with AI to modify, need to provide reason.」
 *
 * 所以这是一张三列的表：做什么 / 谁做 / 为什么。方案由印记提，她通读一遍，
 * 认可就确认；要改就到对话里跟印记说——改动带着上下文发生在对话里，比在表格
 * 里点几个下拉框更接近她真正要做的那件事（说清楚为什么这一格该她自己来）。
 *
 * 🚨 名字标签用真名。「印记做 / 你做」是在替一个人讲话；一个带颜色的名字标签
 * 只是在指认谁，这正是标签该干的事（AGENTS.md §界面文案 0）。两个人一起做，
 * 就并排挂两个标签。
 */

const YINJI_HUE = "var(--mk-taro)";
const STUDENT_HUE = "var(--mk-matcha)";

function NameTag({ name, hue }: { name: string; hue: string }) {
  return (
    <span
      className="inline-block shrink-0 rounded-mk-full px-2 py-0.5 text-mk-small"
      style={{ background: `color-mix(in srgb, ${hue} 18%, transparent)`, color: "var(--mk-ink)" }}
    >
      {name}
    </span>
  );
}

/** 一格挂几个标签：一个人一个，两个人两个。 */
function Owners({ owner, me }: { owner: Owner; me: string }) {
  if (owner === "both") {
    return (
      <span className="flex flex-wrap gap-1">
        <NameTag name="印记" hue={YINJI_HUE} />
        <NameTag name={me} hue={STUDENT_HUE} />
      </span>
    );
  }
  return owner === "yinji" ? (
    <NameTag name="印记" hue={YINJI_HUE} />
  ) : (
    <NameTag name={me} hue={STUDENT_HUE} />
  );
}

/**
 * OwnerTrack —— 名字牌坐在一条三段轨道上：印记 ─ 一起 ─ 她。
 *
 * 🚨 名字牌本身就是控件，不是一个装饰。点另一段就换人——这一下把「AI 顺手做掉」
 * 从默认值变成一个她主动做出的选择，而选择要给理由。
 *
 * 用点、不用拖：三段就在眼前，点一下最直接，触屏也不会误触；拖在这里换不来
 * 任何额外的意思。
 */
function OwnerTrack({
  owner,
  me,
  onPick,
}: {
  owner: Owner;
  me: string;
  onPick: (o: Owner) => void;
}) {
  const seats: { owner: Owner; label: string; hue: string }[] = [
    { owner: "yinji", label: "印记", hue: YINJI_HUE },
    { owner: "both", label: "一起", hue: "var(--mk-lake)" },
    { owner: "student", label: me, hue: STUDENT_HUE },
  ];
  return (
    <span className="inline-flex overflow-hidden rounded-mk-full border border-mk-border">
      {seats.map((s) => {
        const on = s.owner === owner;
        return (
          <button
            key={s.owner}
            type="button"
            onClick={() => onPick(s.owner)}
            className="px-2 py-0.5 text-mk-small"
            style={
              on
                ? { background: `color-mix(in srgb, ${s.hue} 24%, transparent)`, color: "var(--mk-ink)" }
                : { color: "var(--mk-faint)" }
            }
          >
            {s.label}
          </button>
        );
      })}
    </span>
  );
}

export function Split({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [steps, setSteps] = useState<PlanStep[]>([]);
  const [stepId, setStepId] = useState<string | null>(null);
  const [subs, setSubs] = useState<Substep[]>([]);
  const [me, setMe] = useState("你");
  const [error, setError] = useState<string | null>(null);
  // 正在改归属的那一行：她点了另一段轨道，还欠一句理由。
  const [moving, setMoving] = useState<{ id: string; owner: Owner } | null>(null);
  const [why, setWhy] = useState("");
  // 她要补上的那一件。
  const [adding, setAdding] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [newWhy, setNewWhy] = useState("");

  /** 补一件印记漏掉的。归属默认给她自己——她想起来的，通常也是她要做的。 */
  async function addOne() {
    if (!stepId || !newTitle.trim() || !newWhy.trim()) return;
    try {
      const got = await addSubstep(projectId, stepId, {
        title: newTitle.trim(), owner: "student", reason: newWhy.trim(),
      });
      setSubs((prev) => [...prev, got]);
      setAdding(false);
      setNewTitle("");
      setNewWhy("");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  /** 把这一格改判给另一个人。理由是硬的：服务端拒绝没有理由的改动。 */
  async function move() {
    if (!moving || !why.trim()) return;
    try {
      const got = await reassign(projectId, moving.id, moving.owner, why.trim());
      setSubs((prev) => prev.map((x) => (x.id === got.id ? got : x)));
      setMoving(null);
      setWhy("");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  useEffect(() => {
    void (async () => {
      try {
        const { plan } = await getPlan(projectId);
        const all = plan?.steps ?? [];
        setSteps(all);
        const live = all.find((s) => s.status !== "done" && s.status !== "cancelled");
        setStepId(live?.id ?? all[0]?.id ?? null);
      } catch (err) {
        setError(apiErrorText(err));
      }
      // 名字拿不到就退回「你」——一个标签不该把整块界面拖垮。
      try {
        const u = await getMe();
        if (u.display_name.trim()) setMe(u.display_name.trim());
      } catch {
        /* 保持「你」 */
      }
    })();
  }, [projectId]);

  const reload = useCallback(async () => {
    if (!stepId) return;
    setSubs(await listSubsteps(projectId, stepId));
  }, [projectId, stepId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const step = steps.find((s) => s.id === stepId) ?? null;
  const share = shareOfWork(subs);

  async function confirmAll() {
    try {
      for (const s of subs.filter((x) => !x.confirmedAt)) {
        await confirmSubstep(projectId, s.id);
      }
      onFinish({ stepId, mine: share.total - share.yinji, total: share.total }, "");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <ToolFrame
      title={tool.label}
      task="你和 AI 的分工"
      why={tool.reason}
      todo={subs.length === 0 ? "无分工" : ""}
      finishLabel="方案无误，开始执行"
      onFinish={() => void confirmAll()}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {steps.length > 1 && (
        <div className="mb-3 flex flex-wrap gap-1">
          {steps.map((s) => (
            <button
              key={s.id}
              type="button"
              onClick={() => setStepId(s.id)}
              className="rounded-mk-full px-2.5 py-1 text-mk-small"
              style={
                s.id === stepId
                  ? { background: "var(--mk-accent-500)", color: "#fff" }
                  : { border: "1px solid var(--mk-border)", color: "var(--mk-secondary)" }
              }
            >
              {s.title}
            </button>
          ))}
        </div>
      )}

      {step && <p className="text-mk-body text-mk-ink">{step.title}</p>}

      {subs.length === 0 ? (
        <p className="mt-2 text-mk-small text-mk-muted">
          这一步还没有分工方案。印记提出分工后，会在这里让你审核。
        </p>
      ) : (
        <>
          {/* 🚨 「我把多少思考外包出去了」要在她按确认之前就是个看得见的量。
              一条堆叠的比例条，拖名字牌时实时重排。
              不拦截确认——跳过也是信号（铁律④），这里只是把事实摆出来。 */}
          <div className="mt-2">
            <div className="flex h-2 overflow-hidden rounded-mk-full" style={{ background: "var(--mk-paper)" }}>
              {([
                { n: share.yinji, hue: YINJI_HUE },
                { n: share.both, hue: "var(--mk-lake)" },
                { n: share.student, hue: STUDENT_HUE },
              ] as const).map((seg, i) =>
                seg.n > 0 ? (
                  <span
                    key={i}
                    style={{
                      width: `${(seg.n / Math.max(share.total, 1)) * 100}%`,
                      background: `color-mix(in srgb, ${seg.hue} 55%, transparent)`,
                    }}
                  />
                ) : null,
              )}
            </div>
            <p className="mt-1.5 text-mk-small text-mk-muted">
              共 {share.total} 项，其中 {share.yinji} 项由印记完成。
            </p>
            {share.total > 0 && share.yinji * 3 >= share.total * 2 && (
              <p className="mt-1 text-mk-small" style={{ color: "var(--mk-warning)" }}>
                这一步大部分由印记完成。请确认这是你想要的分工。
              </p>
            )}
          </div>

          <div className="mt-2 overflow-hidden rounded-mk-md border border-mk-border">
            <table className="w-full border-collapse text-left">
              <thead>
                <tr className="bg-mk-paper text-mk-small text-mk-secondary">
                  <th className="px-2.5 py-1.5 font-normal">任务</th>
                  <th className="px-2.5 py-1.5 font-normal">负责</th>
                </tr>
              </thead>
              <tbody>
                {subs.map((s) => (
                  <tr key={s.id} className="border-t border-mk-border align-top">
                    <td className="px-2.5 py-2">
                      <p className="text-mk-small text-mk-ink">{s.title}</p>
                      {/* 为什么这么分。她要能反对的，正是这一句。 */}
                      <p className="mt-0.5 text-mk-small text-mk-muted">{s.reason}</p>
                      {s.addedByStudent && (
                        <p className="mt-0.5 text-mk-small" style={{ color: STUDENT_HUE }}>
                          你补的
                        </p>
                      )}
                      {s.studentOwner && s.studentReason && (
                        <p className="mt-0.5 text-mk-small text-mk-secondary">
                          你改的：{s.studentReason}
                        </p>
                      )}
                    </td>
                    <td className="px-2.5 py-2">
                      <OwnerTrack
                        me={me}
                        owner={effectiveOwner(s)}
                        onPick={(o) => {
                          if (o === effectiveOwner(s)) return;
                          setMoving({ id: s.id, owner: o });
                          setWhy("");
                        }}
                      />
                      {/* 换了人就当场说一句为什么。说不出来就不算改——服务端
                          也是这么要求的，这里只是把那道门槛摆到她眼前。 */}
                      {moving?.id === s.id && (
                        <div className="mt-1.5">
                          <input
                            autoFocus
                            value={why}
                            onChange={(e) => setWhy(e.target.value)}
                            onKeyDown={(e) => e.key === "Enter" && void move()}
                            placeholder="修改原因"
                            className="w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2 py-1 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                          />
                          <div className="mt-1 flex gap-2">
                            <button
                              type="button"
                              onClick={() => void move()}
                              disabled={!why.trim()}
                              className="rounded-mk-full px-2.5 py-0.5 text-mk-small disabled:opacity-40"
                              style={{ background: STUDENT_HUE, color: "var(--mk-surface)" }}
                            >
                              确认修改
                            </button>
                            <button
                              type="button"
                              onClick={() => setMoving(null)}
                              className="text-mk-small text-mk-secondary"
                            >
                              取消
                            </button>
                          </div>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* 🚨 审一份方案不等于逐格同意，先要问它漏了什么。 */}
          {adding ? (
            <div className="mt-2 rounded-mk-md border border-dashed border-mk-border px-3 py-2.5">
              <input
                autoFocus
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                placeholder="子任务内容"
                className="w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
              />
              <input
                value={newWhy}
                onChange={(e) => setNewWhy(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void addOne()}
                placeholder="设置原因"
                className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
              />
              <div className="mt-2 flex gap-2">
                <button
                  type="button"
                  onClick={() => void addOne()}
                  disabled={!newTitle.trim() || !newWhy.trim()}
                  className="rounded-mk-full px-3 py-1 text-mk-small disabled:opacity-40"
                  style={{ background: STUDENT_HUE, color: "var(--mk-surface)" }}
                >
                  添加
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
              还少一件事
            </button>
          )}

          <p className="mt-2 text-mk-small text-mk-faint">
            请在负责人一栏调整分工，并说明修改原因。
          </p>
        </>
      )}
    </ToolFrame>
  );
}
