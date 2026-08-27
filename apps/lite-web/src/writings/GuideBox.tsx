import { Button } from "@/ui";
import type { WritingBlockGuide } from "../api/writingRoom";

/**
 * GuideBox — the guiding box, shared by 结构 and 段落.
 *
 * It lives in both because the product asked for guidance at both moments:
 * *"AI should guide me to think about the outlines"* AND *"the key is the
 * AI-generated guiding box"* for the paragraphs. Those are the same
 * mechanism at two zoom levels — what is this block for, and then what goes
 * in it — so they are one component, not two that drift.
 *
 * It renders questions and nothing else. That is the 铁律① guarantee, and it
 * is a guarantee about the DATA TYPE rather than about the model's manners:
 * the server has already dropped every entry that doesn't end in a question
 * mark (writing_guide.go's parseWritingGuide), and a question cannot be
 * pasted into an essay the way a demonstration sentence can.
 *
 * Nothing in here writes to any field. There is no "use this" affordance,
 * because there is nothing here that could be used.
 */
export function GuideBox({
  guide,
  onSummonCard,
  onDismiss,
}: {
  guide: WritingBlockGuide;
  /** Omitted where the room has no card surface to summon into. */
  onSummonCard?: (cardId: string) => void;
  onDismiss: () => void;
}) {
  return (
    <div
      className="flex flex-col gap-2.5 rounded-mk-md border p-3.5"
      style={{ borderColor: "var(--mk-accent-300)", background: "color-mix(in srgb, var(--mk-accent-500) 5%, var(--mk-paper))" }}
    >
      <div className="flex items-center justify-between gap-2">
        <span
          className="rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
          style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
        >
          想一想
        </span>
        <button type="button" onClick={onDismiss} className="text-mk-label text-mk-faint hover:text-mk-muted">
          收起
        </button>
      </div>

      <ol className="flex list-none flex-col gap-2">
        {guide.questions.map((q, i) => (
          <li key={i} className="flex gap-2">
            <span
              className="mt-[3px] flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-mk-full text-[10px] font-semibold"
              style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
            >
              {i + 1}
            </span>
            <span className="text-mk-body text-mk-ink">{q}</span>
          </li>
        ))}
      </ol>

      {guide.cardId && onSummonCard && (
        <div className="flex flex-wrap items-center gap-2 border-t border-mk-border pt-2.5">
          <span className="text-mk-small text-mk-muted">{guide.cardReason || "这一块也许适合用一张工具卡拆开想。"}</span>
          {/* 「叫出来」, not 「打开」: summoning mints the card as *proposed*,
              and she still confirms it in the rail before it opens (铁律②).
              Labelling this 打开 would promise a step it doesn't take. */}
          <Button size="sm" variant="secondary" onClick={() => onSummonCard(guide.cardId)}>
            把这张卡叫出来
          </Button>
        </div>
      )}
    </div>
  );
}
