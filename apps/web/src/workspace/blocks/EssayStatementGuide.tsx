import { useEffect, useMemo, useState } from "react";
import type { ProposalGuideStep, SubQuestion, ClaimRevisionVerdict } from "@mind-imprint/contracts";
import { GuidedWritingCard } from "./GuidedWritingCard";
import { FilledCardsFold, type FilledCard } from "./FilledCardsFold";
import { guidedSectionLabel } from "./sectionLabels";
import { useEssayStatement } from "./useEssayStatement";
import { useSnippets } from "./WritingBlock";
import { reviewEssayPart, reviseClaim } from "../../api/essayStatement";
import { advanceEssayStage } from "../../api/evidenceMap";
import { useStudioChat } from "@/studio/ai/StudioChatContext";

export type ReviseClaim = (
  subQuestionId: string,
  newText: string,
  confirm: boolean,
) => Promise<{ verdict: ClaimRevisionVerdict; applied: boolean }>;

// ClaimRow — one editable sub-question at the 大纲 step (§133–134). Editing the
// text and 保存 classifies the change: a rephrase auto-applies (materials still
// hold); a total_change warns before replacing (materials/writing no longer
// apply) and needs a second 确定替换. Never deletes the student's writing (铁律②).
function ClaimRow({ sq, index, onReviseClaim, onApplied }: {
  sq: SubQuestion;
  index: number;
  onReviseClaim: ReviseClaim;
  onApplied: () => void;
}) {
  const [draft, setDraft] = useState(sq.text);
  const [verdict, setVerdict] = useState<ClaimRevisionVerdict | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => { setDraft(sq.text); setVerdict(null); }, [sq.text]);

  const changed = draft.trim() !== "" && draft.trim() !== sq.text.trim();

  async function apply(confirm: boolean) {
    setBusy(true);
    try {
      const { verdict: v, applied } = await onReviseClaim(sq.id, draft.trim(), confirm);
      if (applied) { setVerdict(null); onApplied(); return; }
      // rephrase → apply straight away; total_change → surface the warning.
      if (v.kind === "rephrase") { await apply(true); return; }
      setVerdict(v);
    } catch {
      /* best-effort */
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className="rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
      <div className="flex items-center gap-2">
        <span className="flex-none text-[12px] font-bold text-mk-faint">论点 {index + 1}</span>
        <input
          value={draft}
          onChange={(e) => { setDraft(e.target.value); setVerdict(null); }}
          className="min-w-0 flex-1 rounded-mk border border-mk-border bg-mk-paper px-2 py-1 text-[13.5px] text-mk-ink focus:border-mk-accent focus:outline-none"
        />
        <button
          type="button"
          disabled={!changed || busy}
          onClick={() => void apply(false)}
          className="flex-none rounded-mk border border-mk-border px-2 py-0.5 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-40"
        >
          {busy ? "印记在看…" : "保存"}
        </button>
      </div>
      {verdict?.kind === "total_change" && (
        <div className="mt-1.5 rounded-mk bg-mk-warning-bg px-2.5 py-1.5 text-[12px] leading-relaxed text-mk-ink">
          <p><span className="font-bold text-mk-warning">这看起来是一个全新的子问题。</span>{verdict.why}它原来的材料和已经写的论述可能都不再适用。确定要换吗？</p>
          <div className="mt-1.5 flex gap-2">
            <button type="button" disabled={busy} onClick={() => void apply(true)} className="rounded-mk bg-mk-accent px-2.5 py-0.5 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50">确定替换</button>
            <button type="button" onClick={() => { setDraft(sq.text); setVerdict(null); }} className="rounded-mk px-2 py-0.5 text-[12px] font-semibold text-mk-muted hover:text-mk-ink">取消</button>
          </div>
        </div>
      )}
    </li>
  );
}

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
  onReviseClaim,
  onClaimApplied,
  onFinishStatement,
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
  onReviseClaim: ReviseClaim;
  onClaimApplied: () => void;
  onFinishStatement: () => void;
  reviewing?: boolean;
}) {
  if (!step) return null;
  const isLast = step.index >= step.total - 1;

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
          <p className="mt-2 text-[14px] leading-relaxed text-mk-ink">{step.card?.prompt ?? "根据你搜集的证据，看看这些论点是否还准确——可以直接改。每一条论点，就是后面要逐条写的一段。"}</p>
          {step.subQuestions.length > 0 && (
            <ul className="mt-3 flex flex-col gap-2">
              {step.subQuestions.map((sq, i) => (
                <ClaimRow key={sq.id} sq={sq} index={i} onReviseClaim={onReviseClaim} onApplied={onClaimApplied} />
              ))}
            </ul>
          )}
          <div className="mt-3 flex items-center gap-3">
            <button type="button" onClick={onNext} className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">大纲调好了，开始写论点</button>
            {nav}
          </div>
        </div>
      </div>
    );
  }

  // A claim / synthesis / challenges / conclusion / structure step → a card. The
  // container is paper so the accent-50 GuidedWritingCard keeps its figure/ground.
  const isClaim = step.kind === "subq";
  return (
    <div className="border-b border-mk-border bg-mk-paper px-8 py-4">
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
        {/* slice 4b-2 · the last step (论证结构) is the end of the statement walk —
            finish it to enter 成文 (submission), instead of a dead-end 下一步. */}
        {isLast && (
          // Secondary (bordered) so it doesn't compete with the card's filled
          // 我写好了 primary — 完成 is the step-after-review action.
          <div className="mt-3 flex justify-end">
            <button type="button" onClick={onFinishStatement} className="rounded-mk-md border border-mk-accent bg-mk-surface px-4 py-1.5 text-[14px] font-bold text-mk-accent hover:bg-mk-accent-50">
              完成正文陈述，进入成文
            </button>
          </div>
        )}
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
  onStageAdvanced,
}: {
  projectId: string;
  onAnnotationsChanged?: () => void;
  onStageAdvanced?: () => void;
}) {
  const track = useEssayStatement(projectId);
  const snip = useSnippets(projectId);
  const { sendStudioTurn, openCard } = useStudioChat();
  const [reviewing, setReviewing] = useState(false);

  const step = track.step;
  const stepKey = step?.key ?? "";
  // The current step's snippet (section = the step key). Bound to the card.
  const [text, setText] = useState("");

  // Re-seed the local text when the step changes.
  useEffect(() => {
    const existing = snip.snippets.find((s) => s.section === stepKey);
    setText(existing?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepKey]);

  function onChange(v: string) {
    setText(v);
    // Write by SECTION against the live snapshot — never a mutable id ref that
    // can still point at the previous claim after a fast advance (same wrong-slot
    // race the proposal guide had).
    snip.upsertSection(stepKey, v);
  }

  // The finished parts (every written step except the one being written now),
  // folded + re-readable with their original guidance. This is the essay's
  // equivalent of the proposal's #82 fold — without it, a finished claim
  // vanished the moment the walk advanced.
  const filledParts: FilledCard[] = useMemo(() => {
    const s = track.step;
    if (!s) return [];
    const textByKey = new Map(snip.snippets.map((x) => [x.section ?? "", x.text.trim()] as const));
    return (s.steps ?? [])
      .filter((st) => st.key !== s.key)
      // A claim's fold header carries its sub-question text (「论点 2：…」), so a
      // finished part names itself instead of a bare 「论点 2」.
      .map((st) => ({ key: st.key, title: guidedSectionLabel(st.key, s.subQuestions) ?? st.title, text: textByKey.get(st.key) ?? "", guidance: st.card?.prompt, example: st.card?.example }))
      .filter((p) => p.text !== "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [track.step, snip.snippets]);

  // Edit a finished part in place from the fold (finished ≠ locked). Keep the
  // active textarea in sync if the edited part is also the current step.
  function editFilledPart(key: string, value: string) {
    snip.upsertSection(key, value);
    if (key === stepKey) setText(value);
  }

  function onStillStuck() {
    if (!step) return;
    void sendStudioTurn(`我在写「${step.title}」这部分，还是有点卡，能带我想想吗？`);
  }

  async function onDone() {
    if (!step) return;
    const wasLast = step.index >= step.total - 1;
    setReviewing(true);
    // Flush the live value to this step's section before review + advance.
    snip.upsertSection(stepKey, text);
    try {
      await reviewEssayPart(projectId, step.key);
      onAnnotationsChanged?.();
    } catch {
      /* best-effort */
    } finally {
      setReviewing(false);
      // §guided-writing · after 我写好了 (review), GUIDE the student to the next
      // step (user: "after student writing, we guide him to the next step"). The
      // last step stays put so its 完成正文陈述 gate shows.
      if (!wasLast) void track.next();
    }
  }

  async function onFinishStatement() {
    try {
      await advanceEssayStage(projectId, "submission");
      onStageAdvanced?.(); // parent reloads studio state → statement pane unmounts
    } catch {
      /* best-effort */
    }
  }

  return (
    <>
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
        onReviseClaim={(sqId, newText, confirm) => reviseClaim(projectId, sqId, newText, confirm)}
        onClaimApplied={() => void track.reload()}
        onFinishStatement={() => void onFinishStatement()}
        reviewing={reviewing}
      />
      {step?.started && filledParts.length > 0 && (
        <div className="border-b border-mk-border bg-mk-paper px-8 pb-4">
          <div className="mx-auto max-w-[70ch]">
            <FilledCardsFold cards={filledParts} onEdit={editFilledPart} />
          </div>
        </div>
      )}
    </>
  );
}
