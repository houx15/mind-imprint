import { useEffect, useRef, useState } from "react";
import type { ProposalGuideStep } from "@mind-imprint/contracts";
import { GuidedWritingCard } from "./GuidedWritingCard";
import { useEssaySubmission } from "./useEssaySubmission";
import { useSnippets } from "./WritingBlock";
import { getEssayStatement } from "../../api/essayStatement";
import { putBuffer } from "../../api/writing";
import { assembleEssayDoc } from "./docSections";
import { useStudioChat } from "@/studio/ai/StudioChatContext";

// EssaySubmissionGuide — slice 4c · the guided submission walk (§6 stage-3). Four
// fixed steps: 引言 → 结论 → 成文 (assemble the whole draft) → 润色定稿 (whole-draft
// review loop → 完成整篇论文). Presentational view + a container that wires the
// track, per-step snippet storage, the compose assembly, and the finish hand-off.

export const SUBMISSION_PLAN_INTRO =
  "正文的各个部分你都写好了。接下来我们把它组装成一篇完整的论文——先写引言，再写结论，然后把各部分拼接成全文，最后通篇润色定稿。准备好了吗？";

export function EssaySubmissionView({
  step,
  value,
  onChange,
  onStart,
  onStillStuck,
  onDone,
  onNext,
  onPrev,
  onCompose,
  onGoToDraft,
  onRequestFinish,
  composing = false,
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
  onCompose: () => void;
  onGoToDraft: () => void;
  onRequestFinish: () => void;
  composing?: boolean;
  reviewing?: boolean;
}) {
  if (!step) return null;

  if (!step.started) {
    return (
      <div className="border-b border-mk-border bg-mk-paper px-8 py-5">
        <div className="mx-auto max-w-[70ch]">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">成文 · 把论文写完整</h3>
          <p className="mt-2 whitespace-pre-wrap text-[14px] leading-relaxed text-mk-ink">{SUBMISSION_PLAN_INTRO}</p>
          <button type="button" onClick={onStart} className="mt-4 rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600">
            准备好了，开始
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

  const header = (
    <div className="mb-2 flex items-center gap-2">
      <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">第 {step.index + 1} / {step.total} 步</span>
      <h3 className="font-sans text-[15px] font-bold text-mk-ink">{step.title}</h3>
      {nav}
    </div>
  );

  // 成文 — assemble the whole draft, then edit it in 正文.
  if (step.key === "sub:compose") {
    return (
      <div className="border-b border-mk-border bg-mk-accent-50 px-8 py-4">
        <div className="mx-auto max-w-[70ch]">
          {header}
          <p className="text-[14px] leading-relaxed text-mk-ink">{step.card?.prompt ?? "把已写好的引言、各条论点、比较/综合、反方回应、结论拼成一整篇，再读一遍衔接。"}</p>
          <div className="mt-3 flex items-center gap-3">
            <button type="button" onClick={onCompose} disabled={composing} className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50">
              {composing ? "拼接中…" : "把各部分拼接成全文"}
            </button>
            <button type="button" onClick={onGoToDraft} className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[13px] font-semibold text-mk-muted hover:text-mk-accent">去正文里编辑</button>
          </div>
          <p className="mt-2 text-[12px] text-mk-faint">拼接是组装，不是重写——全文会出现在「正文」里，你可以自由修改。已有正文不会被覆盖。</p>
        </div>
      </div>
    );
  }

  // 润色定稿 — whole-draft review loop + finish.
  if (step.key === "sub:polish") {
    return (
      <div className="border-b border-mk-border bg-mk-accent-50 px-8 py-4">
        <div className="mx-auto max-w-[70ch]">
          {header}
          <p className="text-[14px] leading-relaxed text-mk-ink">{step.card?.prompt ?? "通读全文、润色语言与衔接。让印记像老师一样通篇体检，你据此修改，满意后完成整篇论文。"}</p>
          <div className="mt-3 flex items-center gap-3">
            <button type="button" onClick={onGoToDraft} className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[13px] font-semibold text-mk-muted hover:text-mk-accent">去正文里润色 / 通篇体检</button>
            <button type="button" onClick={onRequestFinish} className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">完成整篇论文</button>
          </div>
        </div>
      </div>
    );
  }

  // 引言 / 结论 → a GuidedWritingCard (no 写作卡 offer — those live in the claims).
  return (
    <div className="border-b border-mk-border bg-mk-accent-50 px-8 py-4">
      <div className="mx-auto max-w-[70ch]">
        {header}
        <GuidedWritingCard
          title={step.title}
          guidance={step.card?.prompt ?? "写这一部分。"}
          example={step.card?.example}
          value={value}
          onChange={onChange}
          onStillStuck={onStillStuck}
          onDone={onDone}
          reviewing={reviewing}
          placeholder="在这里写这一部分……"
        />
      </div>
    </div>
  );
}

// EssaySubmissionPane — the container: wires the submission track + per-step snippet
// storage (section = step.key) + the 成文 assembly + the finish hand-off.
export function EssaySubmissionPane({
  projectId,
  onGoToDraft,
  onRequestFinish,
}: {
  projectId: string;
  onGoToDraft: () => void;
  onRequestFinish: () => void;
}) {
  const track = useEssaySubmission(projectId);
  const snip = useSnippets(projectId);
  const { sendStudioTurn } = useStudioChat();
  const [composing, setComposing] = useState(false);

  const step = track.step;
  const stepKey = step?.key ?? "";
  const [text, setText] = useState("");
  const snippetIdRef = useRef<string | null>(null);

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
      snippetIdRef.current = snip.add(v, stepKey);
    }
  }

  function onStillStuck() {
    if (!step) return;
    void sendStudioTurn(`我在写论文的「${step.title}」，还是有点卡，能带我想想吗？`);
  }

  // 我写好了 (引言/结论) → the whole-draft review isn't for a single part here; a
  // part review reuses the essay 批注 path via the statement review endpoint's
  // essay doc. For submission we keep it light: mark done → advance.
  function onDone() {
    if (!step) return;
    void track.next();
  }

  // 成文 · assemble the whole draft from the written parts (引言 + claims in outline
  // order + 综合 + 反方 + 结论) into the essay buffer, ONLY if the buffer is empty
  // (never clobber a hand-edited draft). Then land the student in 正文.
  async function onCompose() {
    setComposing(true);
    try {
      const textBySection: Record<string, string> = {};
      for (const s of snip.snippets) {
        if (s.section) textBySection[s.section] = s.text;
      }
      // claim order = the outline order of the sub-questions.
      let claimParts: { section: string; title: string }[] = [];
      try {
        const statement = await getEssayStatement(projectId);
        claimParts = statement.subQuestions.map((sq, i) => ({ section: `claim:${sq.id}`, title: `论点 ${i + 1}` }));
      } catch {
        /* fall back to whatever claim:* snippets exist, unordered */
        claimParts = snip.snippets
          .filter((s) => s.section?.startsWith("claim:"))
          .map((s, i) => ({ section: s.section as string, title: `论点 ${i + 1}` }));
      }
      const parts = [
        { section: "sub:intro", title: "引言" },
        ...claimParts,
        { section: "synthesis", title: "比较 / 综合" },
        { section: "challenges", title: "面对反方观点" },
        { section: "sub:conclusion", title: "结论" },
      ];
      const doc = assembleEssayDoc(parts, textBySection);
      if (doc.trim() !== "") {
        await putBuffer(projectId, doc, "essay");
      }
    } catch {
      /* best-effort */
    } finally {
      setComposing(false);
      onGoToDraft();
    }
  }

  return (
    <EssaySubmissionView
      step={step}
      value={text}
      onChange={onChange}
      onStart={() => void track.start()}
      onStillStuck={onStillStuck}
      onDone={onDone}
      onNext={() => void track.next()}
      onPrev={() => void track.prev()}
      onCompose={() => void onCompose()}
      onGoToDraft={onGoToDraft}
      onRequestFinish={onRequestFinish}
      composing={composing}
    />
  );
}
