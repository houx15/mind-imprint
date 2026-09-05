import { useState } from "react";
import { BookOpen, PenLine, X } from "lucide-react";
import { createReading } from "../api/readings";
import { createWriting } from "../api/writings";
import { liteRoutePath, navigate } from "../routing";
import { Sys } from "../tree/ui";
import { reasonSentence } from "./recommend";
import type { PlacedStar } from "./skyLayout";

/**
 * RecSheet —— 点开一颗推荐星之后的那一面。
 *
 * 三件事，按这个顺序：
 *
 * 1. **为什么给你这个。** 用她自己树上的词和真实的学科边说，不是一句「猜你
 *    喜欢」。这句话由 `reasonSentence` 从数据里拼出来，所以它永远是真的 ——
 *    一次模型调用给出的推荐说不清自己的理由，只能补一句编出来的解释。
 * 2. **一条去处。** 一个不能做任何事的推荐是一句评论。这里和树上的「继续深挖」
 *    走同一条路：`createReading({ title })` / `createWriting({ idea })`，然后
 *    走进那个房间。
 * 3. **不感兴趣。** 这一屏唯一的负反馈。它落库（迁移 0137），不是只存在这次
 *    会话里 —— 她拒绝了什么和她做了什么一样是过程数据。
 */
export function RecSheet({
  star,
  onClose,
  onDismiss,
}: {
  star: PlacedStar | null;
  onClose: () => void;
  onDismiss: (interestId: string) => Promise<void>;
}) {
  const [busy, setBusy] = useState<"read" | "write" | "dismiss" | null>(null);
  const [error, setError] = useState("");

  if (!star) return null;

  async function go(kind: "read" | "write") {
    if (!star) return;
    setBusy(kind);
    setError("");
    try {
      if (kind === "read") {
        const r = await createReading({ title: `关于${star.zh}` });
        navigate(liteRoutePath({ tab: "readings", readingId: r.id }));
      } else {
        const w = await createWriting({ idea: `我想写写${star.zh}` });
        navigate(liteRoutePath({ tab: "writings", writingId: w.id }));
      }
    } catch (e: unknown) {
      // 动词 + 失败，再接后台原话（AGENTS.md 界面文案 §8）。
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  async function drop() {
    if (!star) return;
    setBusy("dismiss");
    setError("");
    try {
      await onDismiss(star.id);
      onClose();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div
      className="fixed inset-y-0 right-0 z-40 flex w-[min(420px,92vw)] flex-col overflow-y-auto px-7 py-6"
      style={{ background: "#0C0A0E", borderLeft: "1px solid rgba(240,233,224,.14)" }}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <Sys tone="dark">还没走过 · UNEXPLORED</Sys>
          <h2 className="mt-1 text-mk-h2 text-[#F5EFE7]">{star.zh}</h2>
          <p className="text-mk-small text-[#8E8175]">{star.en}</p>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label="关闭"
          className="rounded-mk-full p-1.5 text-[#C0B4A6] transition-colors hover:bg-[rgba(240,233,224,.1)]"
        >
          <X size={18} strokeWidth={1.8} />
        </button>
      </div>

      <div
        className="mt-6 rounded-mk-md p-4"
        style={{ background: "rgba(240,233,224,.05)", border: "1px solid rgba(240,233,224,.1)" }}
      >
        <Sys tone="dark">为什么给你这个</Sys>
        <p className="mt-2 text-mk-body leading-[1.85] text-[#EFE7DC]">{reasonSentence(star)}</p>
      </div>

      <div className="mt-6">
        <Sys tone="dark">它扎在哪几门学科上</Sys>
        <ul className="mt-2 flex flex-wrap gap-2">
          {star.viaZh.map((zh) => (
            <li
              key={zh}
              className="rounded-mk-full px-3 py-1 text-mk-small text-[#C0B4A6]"
              style={{ border: "1px solid rgba(240,233,224,.18)" }}
            >
              {zh}
            </li>
          ))}
        </ul>
      </div>

      <div className="mt-8 space-y-2.5">
        <button
          type="button"
          disabled={busy !== null}
          onClick={() => void go("read")}
          className="flex w-full items-center gap-2.5 rounded-mk-md px-4 py-3 text-mk-body text-[#F5EFE7]
                     transition-colors hover:bg-[rgba(240,233,224,.1)] disabled:opacity-50"
          style={{ border: "1px solid rgba(240,233,224,.2)" }}
        >
          <BookOpen size={16} strokeWidth={1.8} />
          {busy === "read" ? "处理中" : "去阅读室找一篇"}
        </button>
        <button
          type="button"
          disabled={busy !== null}
          onClick={() => void go("write")}
          className="flex w-full items-center gap-2.5 rounded-mk-md px-4 py-3 text-mk-body text-[#F5EFE7]
                     transition-colors hover:bg-[rgba(240,233,224,.1)] disabled:opacity-50"
          style={{ border: "1px solid rgba(240,233,224,.2)" }}
        >
          <PenLine size={16} strokeWidth={1.8} />
          {busy === "write" ? "处理中" : "去写作室写一篇"}
        </button>
      </div>

      {error ? (
        <p className="mt-4 break-words text-mk-small leading-[1.8] text-[#E08FA8]">{error}</p>
      ) : null}

      <button
        type="button"
        disabled={busy !== null}
        onClick={() => void drop()}
        className="mt-6 self-start text-mk-small text-[#8E8175] underline underline-offset-4
                   transition-colors hover:text-[#C0B4A6] disabled:opacity-50"
      >
        {busy === "dismiss" ? "处理中" : "不感兴趣"}
      </button>
    </div>
  );
}
