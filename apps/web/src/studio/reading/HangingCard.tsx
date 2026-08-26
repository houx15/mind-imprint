import type { ReactNode } from "react";
import type { SelectionEval } from "@mind-imprint/contracts";

export type HangingCardStatus = "proposed" | "active" | "evaluating" | "feedback";

export type HangingCardProps = {
  cardName: string;
  status: HangingCardStatus;
  exampleWhy: string; // AI's explanation of the example (shown at proposed)
  eval?: SelectionEval | null; // shown at feedback
  onStartPick: () => void; // proposed → active
  onConfirm: () => void; // feedback → completed
  onRepick: () => void; // feedback → active
  // onSkip — deadlock prevention: available at BOTH "proposed" and "active"
  // (not just the feedback repick), so an in-flight lens is ALWAYS
  // dismissable. Optional so a caller that truly can't offer a skip path
  // (none today) still compiles.
  onSkip?: () => void;
  // hasExample — false when the lens was summoned but the AI couldn't ground a
  // single illustrative sentence (graceful-degrade summon). The "proposed"
  // state then drops the "看懂示范" framing and invites her to pick her own
  // sentence directly. Defaults true (the router/normal summon always has one).
  hasExample?: boolean;
  // pickHint (D1, Task 18) — a transient line shown in place of the default
  // "active" instruction when her last click was rejected (she clicked the
  // AI's own underlined example instead of picking her own sentence). The
  // hook clears it automatically a few seconds later. Absent/null → the
  // default instruction renders, unchanged.
  pickHint?: string | null;
};

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// The signature move (demo app.js:422-424): the card hangs under the AI's
// example block while `proposed` (she hasn't picked yet), and MOVES to hang
// under her own chosen sentence the moment she has one AND is past
// `proposed` — `active`/`evaluating`/`feedback` all read her pick, never the
// example, once she has actually picked something. If she is `active` but
// has not picked yet (studentBlockId null), it stays put on the example —
// there's nothing else to hang it on.
export function anchorBlockId(exampleBlockId: string, studentBlockId: string | null, status: HangingCardStatus): string {
  if (studentBlockId && status !== "proposed") return studentBlockId;
  return exampleBlockId;
}

// Verdict/check tones read straight off the shared semantic tokens (never a
// local hex table) — strong/pass reads as success, partial as a heads-up
// (warning), rethink/miss as the thing needing another look (danger).
export const VERDICT_TONE: Record<SelectionEval["verdict"], { fg: string; bg: string }> = {
  strong: { fg: "text-mk-success", bg: "bg-mk-success-bg" },
  partial: { fg: "text-mk-warning", bg: "bg-mk-warning-bg" },
  rethink: { fg: "text-mk-danger", bg: "bg-mk-danger-bg" },
};

export const CHECK_TONE: Record<SelectionEval["checks"][number]["status"], { fg: string; bg: string; label: string }> = {
  pass: { fg: "text-mk-success", bg: "bg-mk-success-bg", label: "✓" },
  partial: { fg: "text-mk-warning", bg: "bg-mk-warning-bg", label: "~" },
  miss: { fg: "text-mk-danger", bg: "bg-mk-danger-bg", label: "✕" },
};

function PrimaryButton({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-[7px] rounded-mk-sm border border-mk-taro-fg bg-mk-taro-fg px-[15px] py-[9px] font-sans text-[14px] font-bold text-white"
    >
      {children}
    </button>
  );
}

// The inline hanging card — the signature "read together" mechanic. It
// hangs under exactly one paragraph at a time (ReadingRoom, via
// anchorBlockId above, guarantees only one render). Exactly ONE primary
// action per status (focus mandate, 铁律 3: one thing at a time).
function SkipLink({ onSkip }: { onSkip: () => void }) {
  return (
    <button
      type="button"
      onClick={onSkip}
      className="mt-[10px] block border-0 bg-transparent p-0 font-sans text-[12px] font-semibold text-mk-muted underline"
    >
      跳过这副透镜
    </button>
  );
}

// HangingCard is macaron-tinted (taro) rather than solid-accent — a
// deliberate second hue so the summoned-lens card reads as its own distinct
// "practice space" beside the room's primary (accent-colored) chrome
// composer/CTAs/tabs, mirroring the note block's taro treatment below it in
// ReadingRoom.tsx. `radius-sm` (8px) matches the design system's default
// card radius.
export function HangingCard({ cardName, status, exampleWhy, eval: selectionEval, onStartPick, onConfirm, onRepick, onSkip, hasExample = true, pickHint }: HangingCardProps) {
  return (
    <div className="relative mt-1 mb-[22px] ml-[18px] overflow-hidden rounded-mk-sm border border-mk-taro-bg bg-mk-surface shadow-mk-sm">
      <div className="lens-connector" aria-hidden="true" />
      <div className="h-1 bg-mk-taro-fg" />
      <div className="px-[15px] pb-[14px] pt-[12px]">
        <div className="mb-2 font-sans text-[12px] font-bold text-mk-taro-fg">{cardName}</div>

        {status === "proposed" && hasExample && (
          <>
            <details className="mb-[10px]">
              <summary className="cursor-pointer font-sans text-[12px] font-semibold text-mk-muted">为什么是这句（方法说明）</summary>
              <div className="mt-[6px] font-sans text-[14px] leading-[1.6] text-mk-secondary">{exampleWhy}</div>
            </details>
            <PrimaryButton onClick={onStartPick}>看懂示范，开始选句</PrimaryButton>
            {onSkip && <SkipLink onSkip={onSkip} />}
          </>
        )}

        {status === "proposed" && !hasExample && (
          <>
            <div className="mb-[10px] font-sans text-[14px] leading-[1.6] text-mk-secondary">{exampleWhy}</div>
            <PrimaryButton onClick={onStartPick}>开始选句</PrimaryButton>
            {onSkip && <SkipLink onSkip={onSkip} />}
          </>
        )}

        {status === "active" && (
          <>
            {/* D1: a click on the AI's own underlined example is rejected —
                correctly, she must find her OWN sentence — but it used to
                reject SILENTLY. pickHint swaps in here, in the same voice
                and styling as the ordinary instruction (not an error), then
                clears itself and reverts. */}
            <div className="font-sans text-[14px] leading-[1.6] text-mk-ink">{pickHint || "在文章里点出你自己的证据句"}</div>
            {onSkip && <SkipLink onSkip={onSkip} />}
          </>
        )}

        {status === "evaluating" && (
          <div className="font-sans text-[14px] leading-[1.6] text-mk-muted">印记正在看你的选择…</div>
        )}

        {status === "feedback" && selectionEval && (
          <>
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

            <div className="mb-3 mt-[10px] font-sans text-[14px] leading-[1.6] text-mk-ink">{selectionEval.nextStep}</div>

            <div className="flex items-center justify-between">
              <button
                type="button"
                onClick={onRepick}
                className="border-0 bg-transparent p-0 font-sans text-[12px] font-semibold text-mk-taro-fg underline"
              >
                重新选一句
              </button>
              <PrimaryButton onClick={onConfirm}>记下这条发现</PrimaryButton>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
