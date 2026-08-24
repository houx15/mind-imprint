import type { SelectionEval } from "@mind-imprint/contracts";
import { VERDICT_TONE, CHECK_TONE, cx } from "./HangingCard";

/**
 * A READ-ONLY recap of a confirmed reading finding, shown inline in the article
 * when the student clicks a highlighted sentence that a completed lens produced.
 *
 * It deliberately wears the SAME visual language as {@link HangingCard}'s
 * "feedback" state — taro tint, a top taro bar, the lens name as a header, the
 * verdict pill, and the per-check ✓/~/✕ rows — so the highlight the student
 * clicks reads as "the 透镜卡 I already used on this sentence", not a foreign
 * little note box. The difference from HangingCard is intent: this one is a
 * frozen record (no 重新选一句 / 记下这条发现 actions), so it carries no buttons.
 *
 * 铁律①: this recaps the student's own analytical finding about a SOURCE — the
 * lens only judged how well her selected sentence supports her judgment; it
 * never drafts a sentence of her essay.
 */
export function ConfirmedFindingCard({
  cardName,
  eval: selectionEval,
  finding,
}: {
  cardName: string;
  eval: SelectionEval;
  finding: string;
}) {
  return (
    <div className="relative overflow-hidden rounded-mk-sm border border-mk-taro-bg bg-mk-surface shadow-mk-sm">
      <div className="h-1 bg-mk-taro-fg" />
      <div className="px-[15px] pb-[14px] pt-[12px]">
        <div className="mb-2 font-sans text-[12px] font-bold text-mk-taro-fg">{cardName}</div>

        <div
          className={cx(
            "mb-2 inline-flex items-center gap-[6px] rounded-mk-full px-[10px] py-[3px] font-sans text-[12px] font-bold",
            VERDICT_TONE[selectionEval.verdict].fg,
            VERDICT_TONE[selectionEval.verdict].bg,
          )}
        >
          {selectionEval.verdictLabel}
        </div>
        <div className="mb-[10px] font-sans text-[14px] leading-[1.6] text-mk-secondary">{selectionEval.verdictReason}</div>

        {selectionEval.checks.map((check) => (
          <div key={check.key} className="mb-[6px] flex items-start gap-2">
            <span
              className={cx(
                "mt-px flex h-4 w-4 shrink-0 items-center justify-center rounded-mk-full font-sans text-[12px] font-bold",
                CHECK_TONE[check.status].fg,
                CHECK_TONE[check.status].bg,
              )}
            >
              {CHECK_TONE[check.status].label}
            </span>
            <div>
              <div className="font-sans text-[12px] font-bold text-mk-ink">{check.label}</div>
              <div className="font-sans text-[12px] leading-[1.55] text-mk-secondary">{check.explanation}</div>
            </div>
          </div>
        ))}

        <div className="mt-[10px] border-t border-mk-taro-bg pt-[10px]">
          <div className="mb-1 font-sans text-[12px] font-bold text-mk-muted">你记下的发现</div>
          <div className="font-sans text-[14px] leading-[1.6] text-mk-ink">{finding}</div>
        </div>
      </div>
    </div>
  );
}
