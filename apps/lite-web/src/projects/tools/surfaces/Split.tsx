import { useCallback, useEffect, useState } from "react";
import { getMe } from "../../../api/auth";
import { apiErrorText } from "../../../api/errorText";
import { getPlan, type PlanStep } from "../../../api/projectRoom";
import {
  confirmSubstep,
  effectiveOwner,
  listSubsteps,
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

const YINJI_HUE = "#8B5CF6";
const STUDENT_HUE = "#10B981";

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

export function Split({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [steps, setSteps] = useState<PlanStep[]>([]);
  const [stepId, setStepId] = useState<string | null>(null);
  const [subs, setSubs] = useState<Substep[]>([]);
  const [me, setMe] = useState("你");
  const [error, setError] = useState<string | null>(null);

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
          这一步还没有分工方案。到对话里请印记给一个。
        </p>
      ) : (
        <>
          {/* 在她还能改的时候，把分工的样子摆出来。 */}
          <p className="mt-2 text-mk-small text-mk-muted">
            共 {share.total} 项，其中 {share.yinji} 项由印记完成。
          </p>

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
                      {s.studentOwner && s.studentReason && (
                        <p className="mt-0.5 text-mk-small text-mk-secondary">
                          你改的：{s.studentReason}
                        </p>
                      )}
                    </td>
                    <td className="px-2.5 py-2">
                      <Owners owner={effectiveOwner(s)} me={me} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <p className="mt-2 text-mk-small text-mk-faint">
            想调整分工，到左边的对话里跟印记说，并说明理由。
          </p>
        </>
      )}
    </ToolFrame>
  );
}
