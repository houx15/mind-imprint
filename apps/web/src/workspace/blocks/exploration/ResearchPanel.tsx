import { useCallback, useEffect, useState } from "react";
import type { EvidenceMap, EvidenceMapSubQuestion, SubQuestionVerdict } from "@mind-imprint/contracts";
import { getEvidenceMap, reviewSubQuestion, advanceEssayStage } from "@/api/evidenceMap";

// ResearchPanel — slice 4a · the research-stage guidance in the reading room. It
// walks the student ONE sub-question at a time (§5/§6): the "current task" is the
// first sub-question without a saturated verdict (or the one just reviewed). Per
// sub-question, the student can ask 印记 "证据够了吗？" — a flagship saturation
// review (support + challenge + diminishing returns), advisory. When a
// sub-question is ready they can write that claim, or finish all research and
// write — both flip the essay stage to statement (opens the writing surface).

// currentTask picks the sub-question to focus: the first one without a
// "saturated" verdict; falls back to the first sub-question ("start from
// sub-question 1"). Pure + exported for testing.
export function currentTask(
  subQuestions: EvidenceMapSubQuestion[],
  verdicts: Record<string, SubQuestionVerdict>,
): EvidenceMapSubQuestion | null {
  if (subQuestions.length === 0) return null;
  const firstUnsaturated = subQuestions.find((sq) => !verdicts[sq.id]?.saturated);
  return firstUnsaturated ?? subQuestions[0]!;
}

export function ResearchPanel({ projectId, onAdvanced }: { projectId: string; onAdvanced: () => void | Promise<void> }) {
  const [map, setMap] = useState<EvidenceMap | null>(null);
  const [verdicts, setVerdicts] = useState<Record<string, SubQuestionVerdict>>({});
  const [reviewing, setReviewing] = useState<string | null>(null);
  const [advancing, setAdvancing] = useState(false);

  const reload = useCallback(async () => {
    try {
      setMap(await getEvidenceMap(projectId));
    } catch {
      /* leave prior */
    }
  }, [projectId]);
  useEffect(() => {
    void reload();
  }, [reload]);

  async function review(sqId: string) {
    setReviewing(sqId);
    try {
      const v = await reviewSubQuestion(projectId, sqId);
      setVerdicts((c) => ({ ...c, [sqId]: v }));
    } catch {
      /* best-effort */
    } finally {
      setReviewing(null);
    }
  }

  async function advance(claimId?: string) {
    if (advancing) return;
    setAdvancing(true);
    try {
      await advanceEssayStage(projectId, "statement", claimId);
      await onAdvanced();
    } catch {
      setAdvancing(false);
    }
  }

  if (!map || map.subQuestions.length === 0) return null;
  const task = currentTask(map.subQuestions, verdicts);

  return (
    <div className="flex-none border-b border-mk-border bg-mk-paper px-5 py-3">
      <div className="flex items-center gap-2">
        <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">研究</span>
        <p className="text-[13px] text-mk-muted">
          先从「<span className="font-bold text-mk-ink">{task?.text}</span>」开始：找能<span className="text-mk-success">支持</span>或<span className="text-mk-danger">挑战</span>它的材料。
        </p>
      </div>

      <ul className="mt-3 flex flex-col gap-2">
        {map.subQuestions.map((sq) => {
          const v = verdicts[sq.id];
          const support = sq.papers.filter((p) => p.nature === "support").length;
          const challenge = sq.papers.filter((p) => p.nature === "challenge").length;
          return (
            <li key={sq.id} className="rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
              <div className="flex items-center gap-2">
                <span className="min-w-0 flex-1 truncate text-[13.5px] font-semibold text-mk-ink">{sq.text}</span>
                <span className="flex-none text-[12px] text-mk-faint">
                  <span className="text-mk-success">支持 {support}</span> · <span className="text-mk-danger">挑战 {challenge}</span>
                </span>
                <button
                  type="button"
                  disabled={reviewing === sq.id}
                  onClick={() => void review(sq.id)}
                  className="flex-none rounded-mk border border-mk-border px-2 py-0.5 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
                >
                  {reviewing === sq.id ? "印记在核对…" : "证据够了吗？"}
                </button>
              </div>
              {v && (
                <div className="mt-1.5 text-[12px] leading-relaxed">
                  <p className={v.saturated ? "font-semibold text-mk-success" : "font-semibold text-mk-danger"}>
                    {v.saturated ? "这条证据够扎实了" : "还不够饱和"} — {v.why}
                  </p>
                  {v.gaps.length > 0 && (
                    <ul className="mt-1 flex flex-col gap-0.5 pl-4 text-mk-muted">
                      {v.gaps.map((g, i) => (
                        <li key={i} className="list-disc">{g}</li>
                      ))}
                    </ul>
                  )}
                  {v.saturated && (
                    <button
                      type="button"
                      disabled={advancing}
                      onClick={() => void advance(sq.id)}
                      className="mt-1.5 rounded-mk bg-mk-accent px-2.5 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50"
                    >
                      去写这条论点
                    </button>
                  )}
                </div>
              )}
            </li>
          );
        })}
      </ul>

      <button
        type="button"
        disabled={advancing}
        onClick={() => void advance()}
        className="mt-3 text-[12px] font-semibold text-mk-faint underline decoration-dotted hover:text-mk-accent disabled:opacity-50"
      >
        {advancing ? "进入写作中…" : "研究做完了，开始写作"}
      </button>
    </div>
  );
}
