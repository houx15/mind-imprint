import { useEffect, useRef, useState } from "react";
import type { ProposalGuideStep } from "@mind-imprint/contracts";
import { GuidedWritingCard } from "./GuidedWritingCard";
import { useEssayStatement } from "./useEssayStatement";
import { useSnippets } from "./WritingBlock";
import { reviewEssayPart } from "../../api/essayStatement";
import { useStudioChat } from "@/studio/ai/StudioChatContext";

// EssayStatementGuide — slice 4b · the guided statement walk in the essay writing
// room. Two presentational states (§6 + user):
//   - the READY GATE (on the 大纲 page): 印记's whole-plan intro + 准备好了吗？.
//   - the STEP CARD: the current step's GuidedWritingCard (claim / synthesis /
//     challenges / conclusion / structure), bound to that step's snippet.
// The outline step is guidance + a 下一步 (the outline itself is edited in the 大纲
// tab). Presentational + pure so it's easy to test; the container wires the hook,
// snippets, and 批注 reload.

// The 写作卡 offered on a claim step (§6: only here).
export const CLAIM_CARDS = [
  { id: "pee", label: "PEE 写作卡" },
  { id: "toulmin", label: "论证构建卡" },
  { id: "argument-map", label: "论证地图卡" },
];

// The plan-intro copy shown on the ready gate (user's own wording).
export const STATEMENT_PLAN_INTRO =
  "接下来我们开始写正文。右边的大纲，是根据前面讨论的核心问题和子问题形成的初步大纲。我们先依次写各个子问题的论述段落，然后进行对比和讨论，最后构建整体问题与部分之间的关系。准备好了吗？";

export function EssayStatementView({
  step,
  value,
  onChange,
  onStart,
  onStillStuck,
  onDone,
  onNext,
  onPrev,
  onOfferCard,
  reviewing = false,
}: {
  step: ProposalGuideStep | null;
  value: string;
  onChange: (v: string) => void;
  onStart: () => void;
  onStillStuck: () => void;
  onDone: () => void;
  onNext: () => void;
  onPrev: () => void;
  onOfferCard: (cardId: string) => void;
  reviewing?: boolean;
}) {
  if (!step) return null;

  // Ready gate (before the walk starts).
  if (!step.started) {
    return (
      <div className="border-b border-mk-border bg-mk-paper px-8 py-5">
        <div className="mx-auto max-w-[70ch]">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">开始写正文</h3>
          <p className="mt-2 whitespace-pre-wrap text-[14px] leading-relaxed text-mk-ink">{STATEMENT_PLAN_INTRO}</p>
          <button
            type="button"
            onClick={onStart}
            className="mt-4 rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600"
          >
            准备好了，开始写作
          </button>
        </div>
      </div>
    );
  }

  const nav = (
    <div className="ml-auto flex items-center gap-2">
      <button type="button" onClick={onPrev} disabled={step.index === 0} className="rounded-mk-md px-2 py-1 text-[13px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-40">← 上一步</button>
      <button type="button" onClick={onNext} disabled={step.index >= step.total - 1} className="rounded-mk-md px-2 py-1 text-[13px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-40">下一步 →</button>
    </div>
  );

  // The outline step: guidance + go-adjust-the-outline (the outline lives in the
  // 大纲 tab); no textarea here.
  if (step.key === "outline") {
    return (
      <div className="border-b border-mk-border bg-mk-accent-50 px-8 py-4">
        <div className="mx-auto max-w-[70ch]">
          <div className="flex items-center gap-2">
            <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">第 {step.index + 1} / {step.total} 步</span>
            <h3 className="font-sans text-[15px] font-bold text-mk-ink">{step.title}</h3>
          </div>
          <p className="mt-2 text-[14px] leading-relaxed text-mk-ink">{step.card?.prompt ?? "在「大纲」里，根据你搜集的证据调整各条论点的顺序与层次。"}</p>
          <div className="mt-3 flex items-center gap-3">
            <button type="button" onClick={onNext} className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">大纲调好了，开始写论点</button>
            {nav}
          </div>
        </div>
      </div>
    );
  }

  // A claim / synthesis / challenges / conclusion / structure step → a card.
  const isClaim = step.kind === "subq";
  return (
    <div className="border-b border-mk-border bg-mk-accent-50 px-8 py-4">
      <div className="mx-auto max-w-[70ch]">
        <div className="mb-2 flex items-center gap-2">
          <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">第 {step.index + 1} / {step.total} 步</span>
          {nav}
        </div>
        <GuidedWritingCard
          title={step.title}
          guidance={step.card?.prompt ?? "写这一部分。"}
          example={step.card?.example}
          value={value}
          onChange={onChange}
          onStillStuck={onStillStuck}
          onDone={onDone}
          reviewing={reviewing}
          placeholder="在这里写这一段……"
          cardOffer={isClaim ? CLAIM_CARDS : undefined}
          onOfferCard={onOfferCard}
        />
      </div>
    </div>
  );
}

// EssayStatementPane — the container: wires the statement track + per-step
// snippet storage + coach + essay 批注. Each step's text is a snippet
// (section = the step key); the outline/ready-gate steps carry no text.
export function EssayStatementPane({
  projectId,
  onAnnotationsChanged,
}: {
  projectId: string;
  onAnnotationsChanged?: () => void;
}) {
  const track = useEssayStatement(projectId);
  const snip = useSnippets(projectId);
  const { sendStudioTurn, openCard } = useStudioChat();
  const [reviewing, setReviewing] = useState(false);

  const step = track.step;
  const stepKey = step?.key ?? "";
  // The current step's snippet (section = the step key). Bound to the card.
  const [text, setText] = useState("");
  const snippetIdRef = useRef<string | null>(null);

  // Re-seed the local text + snippet id when the step changes.
  useEffect(() => {
    const existing = snip.snippets.find((s) => s.section === stepKey);
    snippetIdRef.current = existing?.id ?? null;
    setText(existing?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepKey]);

  function onChange(v: string) {
    setText(v);
    if (snippetIdRef.current) {
      snip.update(snippetIdRef.current, v);
    } else {
      snippetIdRef.current = snip.add(v, stepKey); // add once, then update
    }
  }

  function onStillStuck() {
    if (!step) return;
    void sendStudioTurn(`我在写「${step.title}」这部分，还是有点卡，能带我想想吗？`);
  }

  async function onDone() {
    if (!step) return;
    setReviewing(true);
    try {
      await reviewEssayPart(projectId, step.key);
      onAnnotationsChanged?.();
    } catch {
      /* best-effort */
    } finally {
      setReviewing(false);
    }
  }

  return (
    <EssayStatementView
      step={step}
      value={text}
      onChange={onChange}
      onStart={() => void track.start()}
      onStillStuck={onStillStuck}
      onDone={() => void onDone()}
      onNext={() => void track.next()}
      onPrev={() => void track.prev()}
      onOfferCard={(cardId) => openCard(cardId)}
      reviewing={reviewing}
    />
  );
}
