import { useEffect, useMemo, useRef, useState } from "react";
import type { ProposalGuideStep, SubQuestion } from "@mind-imprint/contracts";
import { useProposalTrack } from "./useProposalTrack";
import { reviewProposalPart } from "../../api/proposalTrack";
import { getDraft } from "../api/workspace";
import { useStudioChat } from "@/studio/ai/StudioChatContext";
import { ReviewingHint } from "@/ui";
import { GuidedWritingCard } from "./GuidedWritingCard";
import { PartsOverview, type OverviewPart } from "./PartsOverview";
import { useCardTags } from "./useCardTags";
import { useSnippets } from "./WritingBlock";
import { assembleGuidedDoc, partSectionKey } from "./docSections";
import { putBuffer } from "../../api/writing";

// ProposalGuide — slice 3a · the guided proposal scaffold that sits ABOVE the
// prose writing surface. It walks the student through the proposal's parts
// (free/guided modes; the research plan expands into one card per sub-question,
// one at a time). It never writes the body text (铁律①) and never blocks
// advancing (铁律②).

// The 9-part overview shown in the guided outline intro (static UI copy).
const OUTLINE_PARTS = [
  "对题目的理解",
  "研究问题与范围",
  "暂定论点",
  "研究计划（先定 2–4 个子问题，再逐个展开）",
  "资源",
  "可能的挑战",
  "研究方法",
  "可行性 / 限制 / 伦理",
  "预期结果",
];

// ── Presentational scaffold (pure; easy to test) ────────────────────────────

export function ProposalGuide({
  step,
  bufferNonEmpty,
  reviewing,
  onChooseMode,
  onStart,
  onSaveSubQuestions,
  onNext,
  onPrev,
  onStillStuck,
  onDone,
  onOpenReading,
  partValue,
  onPartChange,
  overview,
  tags,
  onSetTag,
  onJumpPart,
}: {
  step: ProposalGuideStep | null;
  bufferNonEmpty: boolean;
  reviewing: boolean;
  onChooseMode: (m: "free" | "guided") => void;
  onStart: () => void;
  onSaveSubQuestions: (list: SubQuestion[]) => void;
  onNext: () => void;
  onPrev: () => void;
  onStillStuck: () => void;
  onDone: () => void;
  onOpenReading: (note?: string) => void;
  // slice 4b retrofit · the current part's text (its own auto-growing textarea).
  partValue?: string;
  onPartChange?: (v: string) => void;
  // §4 gaps G8/G10 · the parts overview + per-part status tags.
  overview?: OverviewPart[];
  tags?: Record<string, string>;
  onSetTag?: (key: string, status: "green" | "yellow" | "") => void;
  onJumpPart?: (key: string) => void;
}) {
  if (!step) return null;

  // §gaps G5/G6 · the mode-choice and the outline intro now render as scripted
  // actions IN THE CHAT (StudioTurnChips), driven by ProposalGuidePane's effect —
  // not as pane gates. So the pane shows nothing for those two sub-states.
  if (step.mode === "") return null;
  if (step.mode === "free") return null; // free mode → the ProsePane is the whole surface.
  if (!step.started) return null; // guided, not started → the chat's outline intro drives 开始写作.

  // 4a · guided, started, the sub-question define step.
  if (step.kind === "subq-define") {
    return (
      <SubQuestionEditor
        step={step}
        onSave={onSaveSubQuestions}
        onOpenReading={onOpenReading}
        onNext={onNext}
        onPrev={onPrev}
        onDiscuss={onStillStuck}
      />
    );
  }

  // §4 gap G9 · the final 通读与润色 step: read the whole assembled proposal (in
  // the ProsePane below), polish it, get a whole-proposal 批注, then 完成提案.
  if (step.key === "polish") {
    return (
      <div className="border-b border-mk-border bg-mk-paper px-8 py-4">
        <div className="mx-auto max-w-[70ch]">
          {overview && overview.length > 0 && (
            <div className="mb-3">
              <PartsOverview parts={overview} tags={tags ?? {}} currentKey={step.key} onJump={(k) => onJumpPart?.(k)} />
            </div>
          )}
          <div className="mb-2 flex items-center gap-2">
            <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">第 {step.index + 1} / {step.total} 步</span>
            <h3 className="font-sans text-[15px] font-bold text-mk-ink">写成稿 · 通读与润色</h3>
            <button type="button" onClick={onPrev} className="ml-auto rounded-mk-md px-2 py-1 text-[13px] font-semibold text-mk-muted hover:text-mk-accent">← 上一步</button>
          </div>
          <p className="text-[14px] leading-relaxed text-mk-ink">下面是把你各部分连起来的<strong>提案成稿</strong>——这才是要交出去的东西，不是一张张卡片。通读一遍，把各部分理顺、衔接补上、语言润色，让它读起来是一篇完整的提案。让印记像老师一样通篇看一遍，你据此修改；满意后点右上角「完成提案」。</p>
          <div className="mt-3">
            {reviewing ? <ReviewingHint /> : (
              <button type="button" onClick={onDone} className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">让印记通篇看一遍</button>
            )}
          </div>
        </div>
      </div>
    );
  }

  // 4b · guided, started, a normal part (or a sub-question card) — a
  // GuidedWritingCard: guidance + the part's OWN auto-growing textarea (§4's
  // per-part "snippet writing frame") + 我依然有问题/我写好了 + nav.
  return (
    <div className="border-b border-mk-border bg-mk-paper px-8 py-4">
      <div className="mx-auto max-w-[70ch]">
        {/* §4 gap G10 · the consolidated parts overview (all parts + status), click to jump. */}
        {overview && overview.length > 0 && (
          <div className="mb-3">
            <PartsOverview parts={overview} tags={tags ?? {}} currentKey={step.key} onJump={(k) => onJumpPart?.(k)} />
          </div>
        )}
        <div className="mb-2 flex items-center gap-2">
          <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">第 {step.index + 1} / {step.total} 步</span>
          <div className="ml-auto flex items-center gap-2">
            <button type="button" onClick={onPrev} disabled={step.index === 0} className="rounded-mk-md px-2 py-1 text-[13px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-40">← 上一步</button>
            <button type="button" onClick={onNext} disabled={step.index >= step.total - 1} className="rounded-mk-md px-2 py-1 text-[13px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-40">下一步 →</button>
          </div>
        </div>
        <GuidedWritingCard
          title={step.title}
          guidance={step.card?.prompt ?? "印记正在为这一步准备引导……"}
          example={step.card?.example}
          value={partValue ?? ""}
          onChange={onPartChange ?? (() => {})}
          onStillStuck={onStillStuck}
          onDone={onDone}
          reviewing={reviewing}
          placeholder="在这里写这一部分……"
          tag={tags?.[step.key]}
          onTag={onSetTag ? (s) => onSetTag(step.key, s) : undefined}
        />
        {step.card?.refHint && <p className="mt-2 text-[13px] text-mk-muted">💡 {step.card.refHint}</p>}
      </div>
    </div>
  );
}

// SubQuestionEditor — the research-plan define step: guide the student to 2–4
// researchable, correlated, constitutive sub-questions. This is the heart of the
// proposal (all-statuses.md §4), so it gets its own editable list surface.
function SubQuestionEditor({
  step,
  onSave,
  onOpenReading,
  onNext,
  onPrev,
  onDiscuss,
}: {
  step: ProposalGuideStep;
  onSave: (list: SubQuestion[]) => void;
  onOpenReading: (note?: string) => void;
  onNext: () => void;
  onPrev: () => void;
  // §4 gap G7 · brainstorm the sub-questions WITH 印记 in the chat first.
  onDiscuss?: () => void;
}) {
  const [rows, setRows] = useState<SubQuestion[]>(() =>
    step.subQuestions.length ? step.subQuestions : [{ id: "", text: "" }, { id: "", text: "" }],
  );
  // §4 gap G7 · think/discuss FIRST, then write the questions one-by-one — don't
  // dump 4 empty boxes up front. Skip straight to filling if they already have some.
  const [filling, setFilling] = useState(step.subQuestions.length > 0);
  const nonEmpty = useMemo(() => rows.filter((r) => r.text.trim() !== ""), [rows]);
  const canConfirm = nonEmpty.length >= 2;

  return (
    <div className="border-b border-mk-border bg-mk-accent-50 px-8 py-5">
      <div className="mx-auto max-w-[70ch]">
        <div className="flex items-center gap-2">
          <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">
            第 {step.index + 1} / {step.total} 步
          </span>
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">{step.title}</h3>
        </div>
        {step.card ? (
          <p className="mt-3 whitespace-pre-wrap text-[14.5px] leading-relaxed text-mk-ink">{step.card.prompt}</p>
        ) : (
          <p className="mt-3 text-[14px] text-mk-muted">把关键研究问题拆成 2–4 个相互关联、能一起回答它的子问题。</p>
        )}
        {step.card?.example && (
          <div className="mt-3 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2">
            <p className="text-[12px] font-bold text-mk-muted">范例（英文，供参考）</p>
            <p className="mt-1 whitespace-pre-wrap text-[13.5px] leading-relaxed text-mk-muted">{step.card.example}</p>
          </div>
        )}

        {/* §4 gap G7 · THINK phase — brainstorm with 印记 first, then start filling. */}
        {!filling ? (
          <div className="mt-4 flex flex-col gap-3">
            <p className="text-[13.5px] leading-relaxed text-mk-muted">先别急着填空——这是提案最关键的一步。可以先和印记聊聊：你的核心问题可以从哪几个角度拆开？它们怎么串成一条能一起回答核心问题的链？想清楚了再一条条写下来。</p>
            <div className="flex flex-wrap items-center gap-3">
              {onDiscuss && (
                <button type="button" onClick={onDiscuss} className="rounded-mk-md border border-mk-accent bg-mk-accent-50 px-4 py-1.5 text-[14px] font-bold text-mk-accent hover:bg-mk-accent-100">先和印记聊聊子问题</button>
              )}
              <button type="button" onClick={() => setFilling(true)} className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">我想好了，一条条写下来</button>
              <button type="button" onClick={() => onOpenReading("我还没想清楚子问题，想先读点文献。")} className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[14px] font-bold text-mk-ink hover:border-mk-accent">先去做文献探索</button>
            </div>
          </div>
        ) : (
        <>
        <div className="mt-4 flex flex-col gap-2">
          {rows.map((r, i) => (
            <div key={i} className="flex items-center gap-2">
              <span className="flex-none text-[13px] font-bold text-mk-accent">{i + 1}</span>
              <input
                value={r.text}
                onChange={(e) => setRows((rs) => rs.map((x, j) => (j === i ? { ...x, text: e.target.value } : x)))}
                placeholder="写一个子问题……"
                className="min-w-0 flex-1 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-1.5 text-[14px] text-mk-ink outline-none focus:border-mk-accent"
              />
              {rows.length > 2 && (
                <button
                  type="button"
                  onClick={() => setRows((rs) => rs.filter((_, j) => j !== i))}
                  className="flex-none text-[13px] font-semibold text-mk-faint hover:text-mk-danger"
                >
                  删除
                </button>
              )}
            </div>
          ))}
          {rows.length < 4 && (
            <button
              type="button"
              onClick={() => setRows((rs) => [...rs, { id: "", text: "" }])}
              className="self-start rounded-mk-md border border-dashed border-mk-border px-3 py-1 text-[13px] font-semibold text-mk-faint hover:border-mk-accent hover:text-mk-accent"
            >
              + 再加一个子问题
            </button>
          )}
        </div>

        <div className="mt-4 flex flex-wrap items-center gap-3">
          <button
            type="button"
            disabled={!canConfirm}
            onClick={() => { onSave(nonEmpty); onNext(); }}
            className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50"
            title={canConfirm ? "" : "先写至少 2 个子问题"}
          >
            确认子问题
          </button>
          <button
            type="button"
            onClick={() => onOpenReading("我还没想清楚子问题，想先读点文献。")}
            className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[14px] font-bold text-mk-ink hover:border-mk-accent"
          >
            先去做文献探索
          </button>
          <button
            type="button"
            onClick={onPrev}
            disabled={step.index === 0}
            className="ml-auto rounded-mk-md px-2 py-1 text-[13px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-40"
          >
            ← 上一步
          </button>
        </div>
        </>
        )}
      </div>
    </div>
  );
}

// ── Container: wires the track hook + coach thread + review + prose surface ──

export function ProposalGuidePane({
  projectId,
  locked,
  onOpenReading,
  onAnnotationsChanged,
}: {
  projectId: string;
  locked: boolean;
  onOpenReading: (note?: string) => void;
  // slice 3b · fired after a review produces 批注, so the left panel re-fetches.
  onAnnotationsChanged?: () => void;
}) {
  const track = useProposalTrack(projectId);
  const { sendStudioTurn, setChatAction } = useStudioChat();
  const snip = useSnippets(projectId);
  const { tags, setTag } = useCardTags(projectId);
  const [reviewing, setReviewing] = useState(false);
  const [bufferNonEmpty, setBufferNonEmpty] = useState(false);

  // §4 gap G10 · the parts overview: every writable step (kind subq/fixed, i.e.
  // not the mode gate / define step) with whether it has text yet.
  const overview: OverviewPart[] = useMemo(() => {
    const s = track.step;
    if (!s) return [];
    const has = new Set(snip.snippets.filter((x) => x.section?.startsWith("prop:") && x.text.trim() !== "").map((x) => x.section!.slice(5)));
    return (s.steps ?? [])
      .filter((st) => st.kind !== "subq-define")
      .map((st) => ({ key: st.key, title: st.title, hasText: has.has(st.key) }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [track.step, snip.snippets]);

  useEffect(() => {
    let cancelled = false;
    void getDraft(projectId, "proposal")
      .then((c) => { if (!cancelled) setBufferNonEmpty(c.trim() !== ""); })
      .catch(() => { /* leave false */ });
    return () => { cancelled = true; };
  }, [projectId]);

  // §gaps G5/G6 · drive the mode-choice + outline-intro as scripted actions IN
  // THE CHAT (not pane gates): set them while in those sub-states, clear once the
  // student has chosen a mode + started (or in free mode).
  const stepMode = track.step?.mode;
  const stepStarted = track.step?.started;
  useEffect(() => {
    if (!setChatAction) return;
    if (stepMode === "") {
      setChatAction({
        id: "proposal-mode",
        text: bufferNonEmpty
          ? "接下来写研究提案。你已经写了一些——可以自己接着写，也可以让我一步步带你逐部分打磨。"
          : "接下来我们写研究提案。你可以自己写，也可以让我一步步带你写。",
        actions: [
          { label: "我自己写", run: () => void track.chooseMode("free") },
          { label: "一步步带我写", primary: true, run: () => void track.chooseMode("guided") },
        ],
      });
    } else if (stepMode === "guided" && !stepStarted) {
      setChatAction({
        id: "proposal-outline",
        text: `一份扎实的提案大概长这样：\n${OUTLINE_PARTS.map((p, i) => `${i + 1}. ${p}`).join("\n")}\n\n研究计划会拆成你自己的 2–4 个子问题，一个一个来。准备好了就开始吧。`,
        actions: [{ label: "开始写作", primary: true, run: () => void track.start() }],
      });
    } else {
      setChatAction(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepMode, stepStarted, bufferNonEmpty]);
  // Clear the chat action when leaving the proposal room.
  useEffect(() => () => setChatAction?.(null), [setChatAction]);

  const step = track.step;

  // slice 4b retrofit · the current part's text is a snippet (section
  // "prop:<key>"); the ordered parts are assembled into the proposal buffer so
  // free mode + export + finish (which all read the buffer) stay consistent.
  const stepKey = step?.key ?? "";
  const [partText, setPartText] = useState("");
  const assembleTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    const existing = snip.snippets.find((s) => s.section === partSectionKey(stepKey));
    setPartText(existing?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepKey]);

  function assembleToBuffer() {
    if (!step) return;
    const textByKey: Record<string, string> = {};
    // Read the LIVE snapshot (snip.all()) — right after an upsertSection the
    // `snippets` state still lags a render, so assembling off it would drop the
    // part just written (and mis-key the buffer).
    for (const s of snip.all()) {
      if (s.section && s.section.startsWith("prop:")) textByKey[s.section.slice(5)] = s.text;
    }
    void putBuffer(projectId, assembleGuidedDoc(step.steps, textByKey), "proposal").catch(() => {});
  }

  function onPartChange(v: string) {
    setPartText(v);
    // Write by SECTION (prop:<current stepKey>) against the live snapshot — never
    // via a mutable id ref, which could still point at the PREVIOUS part after a
    // fast advance and clobber its slot (root cause of the wrong-slot bug).
    snip.upsertSection(partSectionKey(stepKey), v);
    if (assembleTimer.current) clearTimeout(assembleTimer.current);
    assembleTimer.current = setTimeout(assembleToBuffer, 1000);
  }

  function onStillStuck() {
    if (!step) return;
    void sendStudioTurn(`我在写「${step.title}」这部分，还是有点卡，能带我想想吗？`);
  }

  async function onDone() {
    if (!step) return;
    const isPolish = step.key === "polish";
    setReviewing(true);
    // §4 gap G9 · on the 通读与润色 step the student edits the WHOLE buffer directly
    // (ProsePane), so DON'T re-assemble from the per-part snippets (that would
    // clobber their polish) and DON'T advance (it's the last step; finish is the
    // separate 完成提案 button).
    if (!isPolish) {
      // Flush THIS part's live textarea value to its own section slot before we
      // assemble + advance — the 1s debounce may not have fired yet, and doing it
      // by section (not a stale id ref) guarantees the current draft lands in the
      // current part, never a neighbour's slot.
      snip.upsertSection(partSectionKey(stepKey), partText);
      assembleToBuffer();
    }
    try {
      // The flagship reviewer produces 批注 (view-only, left panel — 铁律①).
      await reviewProposalPart(projectId, step.key);
      onAnnotationsChanged?.();
    } catch {
      /* best-effort; still advance */
    } finally {
      setReviewing(false);
      if (!isPolish) void track.next();
    }
  }

  return (
    <div className="flex flex-col">
      {!locked && (
        <ProposalGuide
          step={step}
          bufferNonEmpty={bufferNonEmpty}
          reviewing={reviewing}
          onChooseMode={(m) => void track.chooseMode(m)}
          onStart={() => void track.start()}
          onSaveSubQuestions={(list) => void track.saveSubQuestions(list)}
          onNext={() => void track.next()}
          onPrev={() => void track.prev()}
          onStillStuck={onStillStuck}
          onDone={() => void onDone()}
          onOpenReading={onOpenReading}
          partValue={partText}
          onPartChange={onPartChange}
          overview={overview}
          tags={tags}
          onSetTag={setTag}
          onJumpPart={(k) => {
            const idx = (step?.steps ?? []).findIndex((s) => s.key === k);
            if (idx >= 0) void track.jump(idx);
          }}
        />
      )}
    </div>
  );
}
