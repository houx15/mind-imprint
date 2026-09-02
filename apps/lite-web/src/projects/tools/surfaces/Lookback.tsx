import { apiErrorText } from "../../../api/errorText";
import { useCallback, useEffect, useState } from "react";
import {
  answerLookback,
  bySection,
  getLookback,
  lookbackTodo,
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

  const reload = useCallback(async () => {
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
        <p className="text-mk-small text-mk-muted">印记正在读这个项目发生过的事，为你写复盘问题…</p>
      )}

      {/* 六段。段是骨架：她答完一串零碎的问题，仍然没被带着从"做了什么"
          走到"我学到了什么"。 */}
      <div className="space-y-4">
        {bySection(prompts).map((g) => (
          <section key={g.key}>
            <p className="text-mk-small text-mk-secondary">{g.title}</p>
            <div className="mt-1.5 space-y-2">
              {g.prompts.map((p) => (
                <PromptRow key={p.id} prompt={p} onSave={(a) => void save(p.id, a)} />
              ))}
            </div>
          </section>
        ))}
      </div>

      {prompts.length > 0 && (
        <p className="mt-3 text-mk-small text-mk-faint">
          请根据你的真实项目体验和感受来回答
        </p>
      )}
    </ToolFrame>
  );
}

function PromptRow({
  prompt,
  onSave,
}: {
  prompt: LookbackPrompt;
  onSave: (answer: string) => void;
}) {
  const [text, setText] = useState(prompt.answer);
  useEffect(() => setText(prompt.answer), [prompt.answer]);

  return (
    <div className="rounded-mk-md border border-mk-border px-3 py-2.5">
      <p className="text-mk-small text-mk-ink">{prompt.prompt}</p>
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
