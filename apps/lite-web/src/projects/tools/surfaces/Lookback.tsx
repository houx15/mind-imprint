import { useCallback, useEffect, useState } from "react";
import {
  answerLookback,
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
    } catch {
      setError("没打开，刷新试试。");
    }
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function save(id: string, answer: string) {
    try {
      const got = await answerLookback(projectId, id, answer);
      setPrompts((prev) => prev.map((p) => (p.id === got.id ? got : p)));
    } catch {
      setError("没记下来，再试一次。");
    }
  }

  const answered = prompts.filter((p) => p.answer.trim()).length;

  return (
    <ToolFrame
      title={tool.label}
      task="回头看这个项目，你是怎么走到这儿的"
      why={tool.reason}
      todo={lookbackTodo(prompts)}
      finishLabel="复盘完了"
      onFinish={() =>
        onFinish(
          { answered },
          `我把这个项目回头看了一遍，答了 ${answered} 问。`,
        )
      }
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {prompts.length === 0 && !error && (
        <p className="text-mk-small text-mk-muted">在整理这个项目发生过的事…</p>
      )}

      <div className="space-y-2">
        {prompts.map((p) => (
          <PromptRow key={p.id} prompt={p} onSave={(a) => void save(p.id, a)} />
        ))}
      </div>

      {prompts.length > 0 && (
        <p className="mt-3 text-mk-small text-mk-faint">
          这些问题是从你这个项目里发生过的事写出来的。
        </p>
      )}
    </ToolFrame>
  );
}

/** 这一问从哪来的。让她知道这不是随便问的。 */
const FROM: Record<LookbackPrompt["anchorKind"], string> = {
  reframe: "你改写过的问题",
  decision: "你做过的一个决定",
  artifact: "你退回去的一份东西",
  free: "",
};

function PromptRow({
  prompt,
  onSave,
}: {
  prompt: LookbackPrompt;
  onSave: (answer: string) => void;
}) {
  const [text, setText] = useState(prompt.answer);
  useEffect(() => setText(prompt.answer), [prompt.answer]);
  const from = FROM[prompt.anchorKind];

  return (
    <div className="rounded-mk-md border border-mk-border px-3 py-2.5">
      {from && <p className="text-mk-small text-mk-faint">{from}</p>}
      <p className="mt-0.5 text-mk-small text-mk-ink">{prompt.prompt}</p>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text.trim() !== prompt.answer.trim() && onSave(text.trim())}
        rows={3}
        placeholder="想到什么写什么，不用写得好看"
        className="mt-2 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}
