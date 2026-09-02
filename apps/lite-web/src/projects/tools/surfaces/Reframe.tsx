import { useCallback, useEffect, useState } from "react";
import {
  confirmReframe,
  createReframe,
  currentReframe,
  draftReframe,
  listReframes,
  reframeSentence,
  updateReframe,
  type Reframe as ReframeRow,
} from "../../../api/reframe";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";
import { apiErrorText } from "../../../api/errorText";

/**
 * Reframe —— 把问题说清楚。
 *
 * 一次只问一个（铁律③）。四个格子如果一起摊开，她会把它当成一张表格填完；
 * 一次一句，她才会真的想"这个人到底是谁"。
 *
 * 上一版一直摆在旁边。问题被重新框定是这门课的核心事件，而她只有看得见
 * "我原来以为是这个"，才知道自己刚才改动了什么。
 */

const STEPS = [
  {
    field: "who" as const,
    ask: "这件事里，具体是谁？",
    hint: "落到一个具体的人。「大家」「学生」太大了，想一个你真的见过的。",
    placeholder: "比如：中午最后一批来打饭的人",
  },
  {
    field: "needs" as const,
    ask: "他需要什么？",
    hint: "说他要的那个东西，先不说你打算怎么给。",
    placeholder: "比如：知道菜还剩不剩",
  },
  {
    field: "why" as const,
    ask: "为什么这对他重要？",
    hint: "如果没有会怎样？答得出这个，问题才站得住。",
    placeholder: "比如：白跑一趟就只能买面包",
  },
  {
    field: "hmw" as const,
    ask: "那么，我们可以怎样……？",
    hint: "写成一个还没有答案的问句。太具体就变成方案了。",
    placeholder: "比如：我们可以怎样让他在出门前就知道还剩什么",
  },
];

export function Reframe({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [all, setAll] = useState<ReframeRow[]>([]);
  const [row, setRow] = useState<ReframeRow | null>(null);
  const [step, setStep] = useState(0);
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [ready, setReady] = useState(false);

  const previous = currentReframe(all);

  const boot = useCallback(async () => {
    const list = await listReframes(projectId);
    setAll(list);
    const draft = draftReframe(list);
    if (draft) {
      setRow(draft);
      // 回到第一个还空着的格子，而不是从头再来一遍。
      const at = STEPS.findIndex((s) => !draft[s.field].trim());
      setStep(at === -1 ? STEPS.length - 1 : at);
      setValue(at === -1 ? draft.hmw : "");
    } else {
      const live = currentReframe(list);
      const made = await createReframe(projectId, live ? { supersedes: live.id } : {});
      setRow(made);
      setStep(0);
      setValue("");
    }
    setReady(true);
  }, [projectId]);

  useEffect(() => {
    void boot();
  }, [boot]);

  const current = STEPS[step];

  async function next() {
    if (!row || !current) return;
    const text = value.trim();
    if (!text) return;
    try {
      const got = await updateReframe(projectId, row.id, { [current.field]: text });
      setRow(got);
      if (step < STEPS.length - 1) {
        setStep(step + 1);
        setValue(STEPS[step + 1] ? got[STEPS[step + 1]!.field] : "");
      }
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function finish() {
    if (!row) return;
    try {
      const got = await confirmReframe(projectId, row.id);
      // 她自己写的那句「我们可以怎样……」，原样带走，不拼接、不做正则修补。
      onFinish({ reframeId: got.id }, got.hmw);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const filled = row ? STEPS.filter((s) => row[s.field].trim()).length : 0;
  const done = filled === STEPS.length;

  return (
    <ToolFrame
      title={tool.label}
      task="一次说一句，把这件事到底是谁的问题说清楚"
      why={tool.reason}
      todo={done ? "" : `还有 ${STEPS.length - filled} 句没说`}
      finishLabel="确认选择"
      onFinish={() => void finish()}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {previous && (
        // 她原来以为问题是什么。看得见这一句，才知道自己刚改动了什么。
        <div className="mb-3 rounded-mk-md px-3 py-2" style={{ background: "var(--mk-paper)" }}>
          <p className="text-mk-small text-mk-muted">你之前说的是</p>
          <p className="mt-0.5 text-mk-small text-mk-secondary">{reframeSentence(previous)}</p>
        </div>
      )}

      {!ready && <p className="text-mk-small text-mk-muted">打开中…</p>}

      {ready && row && (
        <>
          {/* 已经说过的几句，摊在上面 */}
          <div className="space-y-1.5">
            {STEPS.slice(0, step).map((s, i) => (
              <button
                key={s.field}
                type="button"
                onClick={() => {
                  setStep(i);
                  setValue(row[s.field]);
                }}
                className="block w-full rounded-mk-md border border-mk-border px-3 py-2 text-left"
              >
                <span className="text-mk-small text-mk-muted">{s.ask}</span>
                <span className="mt-0.5 block text-mk-small text-mk-ink">{row[s.field] || "——"}</span>
              </button>
            ))}
          </div>

          {/* 正在问的这一句 */}
          {current && (
            <div className="mt-3">
              <p className="text-mk-body text-mk-ink">{current.ask}</p>
              <p className="mt-1 text-mk-small text-mk-muted">{current.hint}</p>
              <textarea
                value={value}
                onChange={(e) => setValue(e.target.value)}
                rows={3}
                placeholder={current.placeholder}
                className="mt-2 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
              />
              {step < STEPS.length - 1 && (
                <button
                  type="button"
                  onClick={() => void next()}
                  disabled={!value.trim()}
                  className="mt-2 rounded-mk-full px-4 py-1.5 text-mk-small text-white disabled:opacity-40"
                  style={{ background: "var(--mk-accent-500)" }}
                >
                  下一句
                </button>
              )}
              {step === STEPS.length - 1 && (
                <button
                  type="button"
                  onClick={() => void next()}
                  disabled={!value.trim() || value.trim() === row.hmw}
                  className="mt-2 rounded-mk-full border border-mk-border px-4 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
                >
                  记下来
                </button>
              )}
            </div>
          )}

          {/* 拼起来读一遍 */}
          {row.who && row.needs && row.why && (
            <div className="mt-4 rounded-mk-md px-3 py-2" style={{ background: "var(--mk-paper)" }}>
              <p className="text-mk-small text-mk-ink">{reframeSentence(row)}</p>
              {row.hmw && <p className="mt-1 text-mk-small text-mk-secondary">{row.hmw}</p>}
            </div>
          )}
        </>
      )}
    </ToolFrame>
  );
}
