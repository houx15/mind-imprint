import { useCallback, useEffect, useState } from "react";
import { getPlan, type PlanStep } from "../../../api/projectRoom";
import {
  OWNER_LABELS,
  confirmSubstep,
  effectiveOwner,
  listSubsteps,
  reassign,
  shareOfWork,
  splitTodo,
  type Owner,
  type Substep,
} from "../../../api/split";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";
import { apiErrorText } from "../../../api/errorText";

/**
 * Split —— 这一步里，哪几格你做，哪几格印记做。
 *
 * 产品负责人 2026-09-01：印记提一张卡，每一格写清归谁、为什么；她可以改，
 * **改也要写为什么**。
 *
 * 那一句为什么是整张卡的意义。不要求理由，她只会一路点"同意"，这张卡就只是
 * 在帮 AI 领活；而 AI 多领一件，她就少做一件。
 *
 * 顶上那行「印记领了 5 格里的 4 格」也是同一件事：让她在**还能改的时候**看见
 * 分工的样子，而不是事后统计。
 */
export function Split({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [steps, setSteps] = useState<PlanStep[]>([]);
  const [stepId, setStepId] = useState<string | null>(null);
  const [subs, setSubs] = useState<Substep[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void (async () => {
      const { plan } = await getPlan(projectId);
      const all = plan?.steps ?? [];
      setSteps(all);
      const live = all.find((s) => s.status !== "done" && s.status !== "cancelled");
      setStepId(live?.id ?? all[0]?.id ?? null);
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

  async function change(s: Substep, owner: Owner, why: string) {
    try {
      const got = await reassign(projectId, s.id, owner, why);
      setSubs((prev) => prev.map((x) => (x.id === got.id ? got : x)));
    } catch (e) {
      setError(apiErrorText(e));
    }
  }

  async function keep(s: Substep) {
    try {
      const got = await confirmSubstep(projectId, s.id);
      setSubs((prev) => prev.map((x) => (x.id === got.id ? got : x)));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <ToolFrame
      title={tool.label}
      task="你和 AI 的分工"
      why={tool.reason}
      todo={splitTodo(subs)}
      finishLabel="方案无误，开始执行"
      onFinish={() =>
        onFinish({ stepId, mine: share.total - share.yinji, total: share.total }, "")
      }
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
          印记还没给这一步分格。到对话里说一句「这一步怎么分」。
        </p>
      ) : (
        <>
          {/* 在她还能改的时候，把分工的样子摆出来。 */}
          <p className="mt-2 text-mk-small text-mk-muted">
            {share.yinji === 0
              ? `${share.total} 格都是你自己做。`
              : `${share.total} 格里，印记领了 ${share.yinji} 格。`}
          </p>

          <div className="mt-2 space-y-2">
            {subs.map((s) => (
              <SubstepRow
                key={s.id}
                substep={s}
                onChange={(o, w) => void change(s, o, w)}
                onKeep={() => void keep(s)}
              />
            ))}
          </div>
        </>
      )}
    </ToolFrame>
  );
}

function SubstepRow({
  substep,
  onChange,
  onKeep,
}: {
  substep: Substep;
  onChange: (owner: Owner, why: string) => void;
  onKeep: () => void;
}) {
  const [pending, setPending] = useState<Owner | null>(null);
  const [why, setWhy] = useState("");
  const owner = effectiveOwner(substep);

  return (
    <div className="rounded-mk-md border border-mk-border px-3 py-2">
      <div className="flex items-start justify-between gap-2">
        <p className="text-mk-small text-mk-ink">{substep.title}</p>
        <span
          className="shrink-0 rounded-mk-full px-2 py-0.5 text-mk-small"
          style={{
            background:
              owner === "yinji"
                ? "color-mix(in srgb, #8B5CF6 16%, transparent)"
                : owner === "both"
                  ? "color-mix(in srgb, #F59E0B 16%, transparent)"
                  : "color-mix(in srgb, #10B981 16%, transparent)",
            color: "var(--mk-ink)",
          }}
        >
          {OWNER_LABELS[owner]}
        </span>
      </div>

      {/* 印记为什么这么分。她要能反对的，正是这句话。 */}
      <p className="mt-1 text-mk-small text-mk-muted">{substep.reason}</p>
      {substep.studentOwner && substep.studentReason && (
        <p className="mt-1 text-mk-small text-mk-secondary">你改的：{substep.studentReason}</p>
      )}

      {!substep.confirmedAt && !pending && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          <button
            type="button"
            onClick={onKeep}
            className="rounded-mk-full px-3 py-1 text-mk-small text-white"
            style={{ background: "var(--mk-accent-500)" }}
          >
            就这样
          </button>
          {(["student", "yinji", "both"] as Owner[])
            .filter((o) => o !== owner)
            .map((o) => (
              <button
                key={o}
                type="button"
                onClick={() => setPending(o)}
                className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary"
              >
                改成{OWNER_LABELS[o]}
              </button>
            ))}
        </div>
      )}

      {pending && (
        // 🚨 改一格就要写一句为什么。这个输入框就是这张卡存在的理由。
        <div className="mt-2">
          <label className="text-mk-small text-mk-secondary">
            改成「{OWNER_LABELS[pending]}」，为什么？
          </label>
          <div className="mt-1 flex items-end gap-2">
            <input
              value={why}
              onChange={(e) => setWhy(e.target.value)}
              placeholder="比如：这一段我想自己写"
              className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
            <button
              type="button"
              disabled={!why.trim()}
              onClick={() => {
                onChange(pending, why.trim());
                setPending(null);
                setWhy("");
              }}
              className="shrink-0 rounded-mk-full px-3 py-1.5 text-mk-small text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              改
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
