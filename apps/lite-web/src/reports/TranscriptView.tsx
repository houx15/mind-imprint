// TranscriptView — 她读完或写完之后回头看的那段对话。
//
// ## 只读是结构性的，不是禁用出来的
//
// 这一屏里没有输入框、没有重试发送、没有任何会往这次原子里写字的控件 —— 它连
// 一条能写的路径都不存在。`FinishedReadingPanel` 顶上那句「READ-ONLY BY
// CONSTRUCTION」说的就是这件事，这一屏挂在它下面，守的是同一条。
//
// 取数走房间本来就有的那条接口（`GET /readings/{id}/messages` 与写作的孪生），
// 不新增接口：这段对话一直都在库里，只是在这之前没有任何界面去读它。
//
// 失败时说一句实话，并给一颗真能按的按钮。2026-09-11 的走查里她为一句
// 「稍后刷新可见」连着卡了四步去找一颗不存在的刷新按钮 —— 一句她照做不了的
// 指令比不说更糟。
import { useCallback, useEffect, useState } from "react";
import { apiErrorText } from "../api/errorText";
import { listReadingMessages, type LiteMessage } from "@lite/api/readingRoom";
import { listWritingMessages } from "@lite/api/writingRoom";
import type { AtomKind } from "@lite/api/reports";
import { useAlive } from "@lite/shared/useAlive";
import { LiteChatMarkdown } from "../readings/LiteChatMarkdown";
import { transcriptLines } from "./transcriptLines";

export function TranscriptView({ kind, atomId }: { kind: AtomKind; atomId: string }) {
  const [msgs, setMsgs] = useState<LiteMessage[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();

  const load = useCallback(() => {
    setError(null);
    const p = kind === "reading" ? listReadingMessages(atomId) : listWritingMessages(atomId);
    p.then((m) => {
      if (alive.current) setMsgs(m);
    }).catch((e: unknown) => {
      if (alive.current) setError(apiErrorText(e));
    });
  }, [kind, atomId, alive]);

  useEffect(load, [load]);

  if (error) {
    return (
      <div className="mk-rp-measure flex flex-wrap items-center gap-2 py-8">
        {/* 报错：动词+失败，再接后台原话。学生和我们看到的是同一句。 */}
        <p className="text-mk-body text-mk-danger">读取失败：{error}</p>
        <button
          type="button"
          onClick={load}
          className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700"
        >
          再试一次
        </button>
      </div>
    );
  }
  if (!msgs) return <p className="mk-rp-measure py-8 text-mk-small text-mk-muted">处理中。</p>;

  const lines = transcriptLines(msgs);
  if (lines.length === 0) {
    return <p className="mk-rp-measure py-8 text-mk-body text-mk-muted">这一次没有留下对话。</p>;
  }

  return (
    <div className="mk-rp-measure flex flex-col gap-4 py-8">
      {lines.map((line) =>
        line.kind === "card" ? (
          <div key={line.seq} className="rounded-mk-lg border border-mk-border bg-mk-surface px-5 py-4">
            <p className="text-mk-label text-mk-faint">印记给了一张卡片 · {line.label}</p>
            {line.answer && <p className="mt-2 whitespace-pre-wrap text-mk-body text-mk-ink">{line.answer}</p>}
          </div>
        ) : (
          <div
            key={line.seq}
            className={line.who === "student" ? "flex flex-col items-end gap-1" : "flex flex-col items-start gap-1"}
          >
            <span className="text-mk-label text-mk-faint">{line.who === "student" ? "我" : "印记"}</span>
            <div
              className="max-w-[46rem] rounded-mk-lg px-5 py-4 text-mk-body text-mk-ink"
              style={{
                background: line.who === "student" ? "var(--mk-accent-50)" : "var(--mk-surface)",
                border: line.who === "student" ? "none" : "1px solid var(--mk-border)",
              }}
            >
              {/* 她的话按原样摆，连换行都照旧：那是她自己写的字。
                  印记的话当初就是 Markdown 渲染出来的，回看里换一种渲染方式，
                  同一段话会长成另一个样子。 */}
              {line.who === "student" ? (
                <p className="whitespace-pre-wrap">{line.text}</p>
              ) : (
                <LiteChatMarkdown text={line.text} />
              )}
            </div>
          </div>
        ),
      )}
    </div>
  );
}
