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
import { tone, type ToneName } from "../../../shared/tone";
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

/**
 * 四句，每句一个颜色。
 *
 * 🚨 产品负责人 2026-09-03：「colorful, interactive」。原来四步共用同一号灰字，
 * 她填到第三格时已经不记得这三句是在拼一个东西——而"这四句拼起来是一句问题
 * 陈述"恰恰是这件工具唯一要教的事。给了颜色，上面那句边填边亮的句子才成立：
 * 谁是蓝的、需要什么是绿的、为什么是琥珀色的，一眼看得出还差哪一块。
 */
const STEPS = [
  {
    field: "who" as const,
    tone: "mist" as const,
    slot: "谁",
    ask: "这件事里，具体是谁？",
    hint: "落到一个具体的人。「大家」「学生」太大了，想一个你真的见过的。",
    placeholder: "比如：中午最后一批来打饭的人",
  },
  {
    field: "needs" as const,
    tone: "matcha" as const,
    slot: "需要什么",
    ask: "这个人需要什么？",
    hint: "说这个人要的那个东西，先不说你打算怎么给。",
    placeholder: "比如：知道菜还剩不剩",
  },
  {
    field: "why" as const,
    tone: "peach" as const,
    slot: "为什么",
    ask: "为什么这对这个人重要？",
    hint: "如果没有会怎样？答得出这个，问题才站得住。",
    placeholder: "比如：白跑一趟就只能买面包",
  },
  {
    field: "hmw" as const,
    tone: "taro" as const,
    slot: "我们可以怎样",
    ask: "那么，我们可以怎样……？",
    hint: "写成一个还没有答案的问句。太具体就变成方案了。",
    placeholder: "比如：我们可以怎样让这个人在出门前就知道还剩什么",
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

  /** 跳回某一句去改。上面那句话里的空格、下面的进度条、已答的几行都用它。 */
  function go(i: number) {
    if (!row) return;
    setStep(i);
    setValue(row[STEPS[i]!.field]);
  }

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
          {/* 🚨 这句话边填边亮。四格拼起来是一句问题陈述——这是这件工具唯一
              要教的事，所以它得一直在她眼前，而不是等四句都填完才出现。
              空的那格是一块虚线，点一下就跳回去填。 */}
          <div
            className="mb-3 rounded-mk-md px-3 py-2.5"
            style={{ background: "var(--mk-paper)" }}
          >
            <p className="text-mk-small leading-[2] text-mk-ink">
              <Slot step={STEPS[0]!} text={row.who} onGo={() => go(0)} />
              <span className="text-mk-secondary">需要</span>
              <Slot step={STEPS[1]!} text={row.needs} onGo={() => go(1)} />
              <span className="text-mk-secondary">，因为</span>
              <Slot step={STEPS[2]!} text={row.why} onGo={() => go(2)} />
              <span className="text-mk-secondary">。</span>
            </p>
            {(row.hmw || step === 3) && (
              <p className="mt-1.5 text-mk-small leading-[2] text-mk-ink">
                <Slot step={STEPS[3]!} text={row.hmw} onGo={() => go(3)} />
              </p>
            )}
          </div>

          {/* 四步走到哪儿了 */}
          <div className="mb-3 flex gap-1">
            {STEPS.map((s, i) => (
              <button
                key={s.field}
                type="button"
                onClick={() => go(i)}
                className="h-1.5 flex-1 rounded-mk-full"
                style={{
                  background: row[s.field].trim()
                    ? tone(s.tone).solid
                    : i === step
                      ? tone(s.tone).bg
                      : "var(--mk-border)",
                }}
                aria-label={s.slot}
                title={s.slot}
              />
            ))}
          </div>

          {/* 已经说过的几句，摊在上面 */}
          <div className="space-y-1.5">
            {STEPS.slice(0, step).map((s, i) => (
              <button
                key={s.field}
                type="button"
                onClick={() => go(i)}
                className="block w-full rounded-mk-md border px-3 py-2 text-left"
                style={{ borderColor: "transparent", background: tone(s.tone).bg }}
              >
                <span className="text-mk-small text-mk-muted">{s.ask}</span>
                <span className="mt-0.5 block text-mk-small text-mk-ink">{row[s.field] || "——"}</span>
              </button>
            ))}
          </div>

          {/* 正在问的这一句 */}
          {current && (
            <div
              className="mt-3 rounded-mk-md border px-3 py-2.5"
              style={{ borderColor: tone(current.tone).solid }}
            >
              <p className="text-mk-body font-semibold text-mk-ink">{current.ask}</p>
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
                  style={{ background: tone(current.tone).solid }}
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

        </>
      )}
    </ToolFrame>
  );
}

/**
 * Slot —— 那句话里的一格。
 *
 * 填了就用这一格的颜色印出来，没填就是一块同色的虚线，点一下跳回去填。
 * 她因此永远看得见"这句话还缺哪一块"，而不是填完四个框才发现拼出来不通顺。
 */
function Slot({
  step,
  text,
  onGo,
}: {
  step: { tone: ToneName; slot: string };
  text: string;
  onGo: () => void;
}) {
  const filled = text.trim() !== "";
  return (
    <button
      type="button"
      onClick={onGo}
      className="mx-0.5 rounded-mk-sm px-1 align-baseline"
      style={
        filled
          ? {
              background: tone(step.tone).bg,
              boxShadow: `inset 0 -2px 0 ${tone(step.tone).solid}`,
              color: tone(step.tone).fg,
            }
          : {
              border: `1px dashed ${tone(step.tone).solid}`,
              color: tone(step.tone).fg,
            }
      }
    >
      {filled ? text : step.slot}
    </button>
  );
}
