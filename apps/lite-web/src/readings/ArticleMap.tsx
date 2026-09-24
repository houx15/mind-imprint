import { useState } from "react";
import { Map as MapIcon, X } from "lucide-react";
import { Icon } from "@/ui";
import { explainReadingBlock, type ReadingBlockNote, type ReadingBlockTool } from "@lite/api/readingRoom";
import { apiErrorText } from "@lite/api/errorText";

/**
 * ArticleMap —— 整篇那一层。摆在正文上面，不在任何一段旁边。
 *
 * # 为什么它不是一件段落工具
 *
 * 产品负责人 2026-09-24：「for poems or 记叙文, we also need a whole-level view
 * of analysis (actually, all need that, the article structure, etc.」
 *
 * 有四样东西**结构上**不在段落这个尺度上：详略安排要比各段的长短；首尾呼应要
 * 同时拿着第一段和最后一段；叙事的「转」是一条跨段的链子；而《江雪》被切成了
 * 两段，任何一件段落工具都看不见整首诗。
 *
 * # 每种体裁交出的是不同的一张图
 *
 * 这一栏摆什么由服务端目录决定（`scope === "article"`），一种体裁一件：
 * 议论文是论证图，说明文是知识结构，新闻报道是事件与来源，记叙文与散文是
 * 事件与人物，诗是起承转合，文言文是全篇章法。名字和里面的分类词都来自
 * `docs/reference/writing-teaching/reading-suggestion.md` 第 4 节。
 *
 * # 每一块都点得回原文
 *
 * 一块一句逐字来自原文的引文（服务端核对过，核不上的那一块已经被丢掉）。
 * 点一块就跳到正文里那个位置 —— 一张说「这篇分成四块」却指不到原文哪里的图，
 * 她一个字都验不了。
 */
export function ArticleMap({
  readingId,
  tools,
  notes,
  onNote,
  onJumpToQuote,
}: {
  readingId: string;
  /** 服务端目录里这一篇的全部工具；这一栏只用 scope === "article" 的那几件。 */
  tools: ReadingBlockTool[];
  notes: ReadingBlockNote[];
  onNote: (note: ReadingBlockNote) => void;
  /** 点一块 → 跳到正文里那句引文所在的位置。 */
  onJumpToQuote: (quote: string) => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const picks = tools.filter((t) => t.scope === "article");
  if (picks.length === 0) return null;

  const noteFor = (tool: string) => notes.find((n) => n.tool === tool && n.article);
  const shown = open ? noteFor(open) : undefined;
  const shownLabel = picks.find((t) => t.id === open)?.label ?? "";

  async function run(tool: ReadingBlockTool) {
    if (open === tool.id) {
      setOpen(null);
      return;
    }
    if (noteFor(tool.id)) {
      setOpen(tool.id);
      return;
    }
    setBusy(tool.id);
    setError(null);
    try {
      // blockId 由服务端换成整篇那个哨兵值，所以这里传什么都不影响存到哪。
      // 传 "" 是为了让请求本身读起来就是「这一次不是对某一段」。
      const note = await explainReadingBlock(readingId, "", tool.id, "");
      onNote(note);
      setOpen(tool.id);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(null);
    }
  }

  return (
    <section className="mk-artmap" aria-label="整篇结构">
      <div className="mk-artmap__bar">
        <Icon icon={MapIcon} size={14} className="shrink-0 text-mk-accent-700" />
        {picks.map((t) => (
          <button
            key={t.id}
            type="button"
            className="mk-artmap__btn"
            aria-pressed={open === t.id}
            disabled={busy !== null}
            onClick={() => void run(t)}
          >
            {busy === t.id ? "…" : t.label}
          </button>
        ))}
      </div>

      {error && (
        <p role="alert" className="mt-2 text-mk-small text-mk-danger">
          {error}
        </p>
      )}

      {shown?.article && (
        <div className="mk-artmap__panel">
          <div className="mk-artmap__head">
            <span className="text-mk-caption text-mk-accent-700">{shownLabel}</span>
            <button
              type="button"
              aria-label="收起整篇结构"
              onClick={() => setOpen(null)}
              className="shrink-0 rounded-mk-xs p-0.5 text-mk-faint hover:text-mk-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              <Icon icon={X} size={14} />
            </button>
          </div>

          {shown.article.spine && (
            <p className="mk-artmap__spine">
              <span className="mk-artmap__spine-key">组织方式</span>
              {shown.article.spine}
            </p>
          )}

          <ol className="mk-artmap__parts">
            {shown.article.parts.map((p, i) => (
              <li key={`${p.quote}-${i}`} className="mk-artmap__part">
                <button
                  type="button"
                  className="mk-artmap__jump"
                  onClick={() => onJumpToQuote(p.quote)}
                  title="跳到正文里这一处"
                >
                  <span className="mk-artmap__label">{p.label}</span>
                  <span className="mk-artmap__quote">{p.quote}</span>
                </button>
                {p.note && <p className="mk-artmap__note">{p.note}</p>}
              </li>
            ))}
          </ol>

          {shown.article.takeaway && (
            <p className="mk-artmap__takeaway">
              <span className="mk-artmap__spine-key">主旨</span>
              {shown.article.takeaway}
            </p>
          )}
        </div>
      )}
    </section>
  );
}
