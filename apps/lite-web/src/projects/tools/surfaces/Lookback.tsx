import { apiErrorText } from "../../../api/errorText";
import { useCallback, useEffect, useRef, useState } from "react";
import { Check } from "lucide-react";
import { Icon } from "@/ui";
import {
  FEELING_WORDS,
  answerLookback,
  bySection,
  getLookback,
  lookbackTodo,
  sectionProgress,
  type LookbackPrompt,
} from "../../../api/lookback";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Lookback —— 复盘。
 *
 * 产品负责人 2026-09-01 只说了一句：「it can be a form.」是表单，但**不是一张
 * 空格**——问「你学到了什么」的空表，和被否掉的那种卡片是同一件东西。
 *
 * 这里的每一问都是服务端从这个项目真的发生过的事里长出来的：她改写过的问题、
 * 她定过的判断、她退回去的成果。她面对的是自己当时写下的那句话。
 *
 * 尤其是那些带着「什么会让我改主意」的判断：它现在回来问她，那件事发生了没有。
 * 做决定时写下那一句的意义，到这里才兑现。
 */
export function Lookback({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [prompts, setPrompts] = useState<LookbackPrompt[]>([]);
  const [error, setError] = useState<string | null>(null);
  // 🚨 只拉一次。StrictMode 会把挂载跑两遍，两次 GET 都会触发一次生成——问题
  // 翻倍不说，钱也白花一份。服务端另有一道锁兜底，但常见的这一种在这里就该拦住。
  const asked = useRef(false);

  const reload = useCallback(async () => {
    if (asked.current) return;
    asked.current = true;
    try {
      setPrompts(await getLookback(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function save(id: string, answer: string) {
    try {
      const got = await answerLookback(projectId, id, answer);
      setPrompts((prev) => prev.map((p) => (p.id === got.id ? got : p)));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const answered = prompts.filter((p) => p.answer.trim()).length;

  return (
    <ToolFrame
      title={tool.label}
      task="回顾项目落地全流程"
      why={tool.reason}
      todo={lookbackTodo(prompts)}
      finishLabel="完成"
      onFinish={() =>
        onFinish({ answered }, "")
      }
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {prompts.length === 0 && !error && (
        <p className="text-mk-small text-mk-muted">
          印记正在读这个项目发生过的事，为你写复盘问题。这一步要读完整个项目，
          通常要等一分钟左右。
        </p>
      )}

      {/* 整体进度。复盘是六段几十题，没有一条进度她不知道自己走到哪儿了。 */}
      {prompts.length > 0 && (
        <div className="mb-4">
          <div className="flex items-baseline justify-between">
            <p className="text-mk-small text-mk-secondary">
              已回答 {answered} / {prompts.length}
            </p>
            <p className="text-mk-small text-mk-faint">请根据你的真实项目体验和感受来回答</p>
          </div>
          <div
            className="mt-1.5 h-1.5 w-full overflow-hidden rounded-mk-full"
            style={{ background: "var(--mk-paper)" }}
          >
            <div
              className="h-full rounded-mk-full transition-[width]"
              style={{
                width: `${prompts.length ? (answered / prompts.length) * 100 : 0}%`,
                background: "var(--mk-accent-500)",
              }}
            />
          </div>
        </div>
      )}

      {/* 六段。段是骨架：她答完一串零碎的问题，仍然没被带着从"做了什么"
          走到"我学到了什么"。
          🚨 每段一个颜色和一条色轨（产品负责人 2026-09-03：「colorful」）。
          原来六段共用同一号灰字，读下来是一张长表，看不出自己在哪一段。 */}
      <div className="space-y-5">
        {bySection(prompts).map((g) => {
          const at = sectionProgress(g.prompts);
          return (
            <section key={g.key}>
              <div className="flex items-center gap-2">
                <span
                  className="h-4 w-1 rounded-mk-full"
                  style={{ background: g.hue }}
                />
                <p className="text-mk-body font-semibold text-mk-ink">{g.title}</p>
                <span
                  className="rounded-mk-full px-1.5 text-[11px] font-semibold"
                  style={{
                    background:
                      at.done === at.total
                        ? `color-mix(in srgb, ${g.hue} 18%, transparent)`
                        : "var(--mk-paper)",
                    color: at.done === at.total ? g.hue : "var(--mk-faint)",
                  }}
                >
                  {at.done}/{at.total}
                </span>
              </div>
              <div className="mt-2 space-y-2">
                {g.prompts.map((p) => (
                  <PromptRow
                    key={p.id}
                    prompt={p}
                    hue={g.hue}
                    // 「感受如何」最难下笔：给几个词点一下起头。
                    words={g.key === "how" ? [...FEELING_WORDS] : []}
                    onSave={(a) => void save(p.id, a)}
                  />
                ))}
              </div>
            </section>
          );
        })}
      </div>
    </ToolFrame>
  );
}

function PromptRow({
  prompt,
  hue,
  words,
  onSave,
}: {
  prompt: LookbackPrompt;
  hue: string;
  /** 点一下就填进去的几个词。空数组 = 这一段不给词。 */
  words: string[];
  onSave: (answer: string) => void;
}) {
  const [text, setText] = useState(prompt.answer);
  useEffect(() => setText(prompt.answer), [prompt.answer]);
  const done = prompt.answer.trim() !== "";

  /** 点一个词：填进去当开头，她接着往下写。已经在里面就拿掉。 */
  function toggleWord(w: string) {
    const next = text.startsWith(w) ? text.slice(w.length).replace(/^[，,。\s]+/, "") : `${w}，${text}`;
    setText(next);
    if (next.trim() !== prompt.answer.trim()) onSave(next.trim());
  }

  return (
    <div
      className="rounded-mk-md border px-3 py-2.5"
      style={{
        borderColor: done ? `color-mix(in srgb, ${hue} 45%, transparent)` : "var(--mk-border)",
        background: done ? `color-mix(in srgb, ${hue} 6%, transparent)` : "transparent",
      }}
    >
      <div className="flex items-start gap-2">
        {/* 答过的打勾，没答的留一个空圈。扫一眼就知道还剩哪几题。 */}
        <span
          className="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-mk-full"
          style={{
            background: done ? hue : "transparent",
            border: done ? "none" : "1.5px solid var(--mk-border)",
            color: "#fff",
          }}
        >
          {done && <Icon icon={Check} size={11} />}
        </span>
        <p className="text-mk-small text-mk-ink">{prompt.prompt}</p>
      </div>

      {/* 🚨 点一个词起头。问一个中学生"你感觉如何"，她面对的是一个空框和一个
          不知道该多正式的期待——这几个词是台阶，不是选项。 */}
      {words.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {words.map((w) => {
            const on = text.startsWith(w);
            return (
              <button
                key={w}
                type="button"
                onClick={() => toggleWord(w)}
                className="rounded-mk-full px-2 py-0.5 text-mk-small"
                style={
                  on
                    ? { background: hue, color: "#fff" }
                    : { border: "1px solid var(--mk-border)", color: "var(--mk-secondary)" }
                }
              >
                {w}
              </button>
            );
          })}
        </div>
      )}

      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text.trim() !== prompt.answer.trim() && onSave(text.trim())}
        rows={3}
        placeholder="写下你的真实想法"
        className="mt-2 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}
