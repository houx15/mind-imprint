import { useEffect, useMemo, useRef, useState } from "react";
import type { ProposalGuideStep, SubQuestion } from "@mind-imprint/contracts";
import { useProposalTrack } from "./useProposalTrack";
import { reviewProposalPart } from "../../api/proposalTrack";
import { getDraft } from "../api/workspace";
import { useStudioChat } from "@/studio/ai/StudioChatContext";
import { ProsePane } from "./ProsePane";
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

  // 1 · mode not chosen → the choice gate.
  if (step.mode === "") {
    return (
      <div className="border-b border-mk-border bg-mk-paper px-8 py-5">
        <div className="mx-auto max-w-[70ch]">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">要怎么写这份提案？</h3>
          <p className="mt-1 text-[14px] text-mk-muted">
            {bufferNonEmpty
              ? "你已经写了一些——可以自己接着写，也可以让印记带你逐部分打磨。"
              : "你可以自己写，也可以让印记一步步带你写。"}
          </p>
          <div className="mt-4 flex gap-3">
            <button
              type="button"
              onClick={() => onChooseMode("free")}
              className={`rounded-mk-md border px-4 py-2 text-[14px] font-bold ${bufferNonEmpty ? "border-mk-accent bg-mk-accent-50 text-mk-accent" : "border-mk-border text-mk-ink hover:border-mk-accent"}`}
            >
              我自己写
            </button>
            <button
              type="button"
              onClick={() => onChooseMode("guided")}
              className={`rounded-mk-md border px-4 py-2 text-[14px] font-bold ${bufferNonEmpty ? "border-mk-border text-mk-ink hover:border-mk-accent" : "border-mk-accent bg-mk-accent-50 text-mk-accent"}`}
            >
              一步步带我写
            </button>
          </div>
        </div>
      </div>
    );
  }

  // 2 · free mode → no scaffold (the ProsePane is the whole surface).
  if (step.mode === "free") return null;

  // 3 · guided, not started → the outline intro.
  if (!step.started) {
    return (
      <div className="border-b border-mk-border bg-mk-paper px-8 py-5">
        <div className="mx-auto max-w-[70ch]">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">一份扎实的提案，大概长这样</h3>
          <ol className="mt-3 flex list-decimal flex-col gap-1 pl-5 text-[14px] text-mk-ink">
            {OUTLINE_PARTS.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ol>
          <p className="mt-3 text-[13px] text-mk-muted">研究计划会拆成你自己的 2–4 个子问题，一个一个来。</p>
          <button
            type="button"
            onClick={onStart}
            className="mt-4 rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600"
          >
            开始写作
          </button>
        </div>
      </div>
    );
  }

  // 4a · guided, started, the sub-question define step.
  if (step.kind === "subq-define") {
    return (
      <SubQuestionEditor
        step={step}
        onSave={onSaveSubQuestions}
        onOpenReading={onOpenReading}
        onNext={onNext}
        onPrev={onPrev}
      />
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
}: {
  step: ProposalGuideStep;
  onSave: (list: SubQuestion[]) => void;
  onOpenReading: (note?: string) => void;
  onNext: () => void;
  onPrev: () => void;
}) {
  const [rows, setRows] = useState<SubQuestion[]>(() =>
    step.subQuestions.length ? step.subQuestions : [{ id: "", text: "" }, { id: "", text: "" }],
  );
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
  const { sendStudioTurn } = useStudioChat();
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

  const step = track.step;
  const isGuided = step?.mode === "guided" && step.started;

  // slice 4b retrofit · the current part's text is a snippet (section
  // "prop:<key>"); the ordered parts are assembled into the proposal buffer so
  // free mode + export + finish (which all read the buffer) stay consistent.
  const stepKey = step?.key ?? "";
  const [partText, setPartText] = useState("");
  const partIdRef = useRef<string | null>(null);
  const assembleTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    const existing = snip.snippets.find((s) => s.section === partSectionKey(stepKey));
    partIdRef.current = existing?.id ?? null;
    setPartText(existing?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepKey]);

  function assembleToBuffer() {
    if (!step) return;
    const textByKey: Record<string, string> = {};
    for (const s of snip.snippets) {
      if (s.section && s.section.startsWith("prop:")) textByKey[s.section.slice(5)] = s.text;
    }
    void putBuffer(projectId, assembleGuidedDoc(step.steps, textByKey), "proposal").catch(() => {});
  }

  function onPartChange(v: string) {
    setPartText(v);
    if (partIdRef.current) snip.update(partIdRef.current, v);
    else partIdRef.current = snip.add(v, partSectionKey(stepKey));
    if (assembleTimer.current) clearTimeout(assembleTimer.current);
    assembleTimer.current = setTimeout(assembleToBuffer, 1000);
  }

  function onStillStuck() {
    if (!step) return;
    void sendStudioTurn(`我在写「${step.title}」这部分，还是有点卡，能带我想想吗？`);
  }

  async function onDone() {
    if (!step) return;
    setReviewing(true);
    assembleToBuffer(); // make sure the buffer reflects this part before the review reads it
    try {
      // The flagship reviewer produces 批注 (view-only, left panel — 铁律①).
      await reviewProposalPart(projectId, step.key);
      onAnnotationsChanged?.();
    } catch {
      /* best-effort; still advance */
    } finally {
      setReviewing(false);
      void track.next();
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
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
      {/* Free mode writes the whole proposal in the ProsePane; guided mode writes
          per-part in the cards above (§4). */}
      {!isGuided && (
        <div className="min-h-0 flex-1">
          <ProsePane projectId={projectId} doc="proposal" locked={locked} onSendToCoach={(t) => void sendStudioTurn(t)} />
        </div>
      )}
    </div>
  );
}
