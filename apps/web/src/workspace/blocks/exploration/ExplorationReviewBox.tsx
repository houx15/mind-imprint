import { useState } from "react";
import { reviewExploration } from "../../../api/explorationReview";
import { ReviewingHint } from "@/ui";

// ExplorationReviewBox — §5 (user follow-up) · once the student has collected +
// tagged materials, they can ask 印记 to review the whole exploration and suggest
// what's most correlated/important, what's weakly related (trim), and gaps.
// Student-initiated (a tap), flagship reviewer, advisory (铁律②).
export function ExplorationReviewBox({ projectId }: { projectId: string }) {
  const [review, setReview] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function run() {
    setLoading(true);
    try {
      setReview(await reviewExploration(projectId));
    } catch {
      setReview("");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
      <div className="flex items-center gap-2">
        <span className="rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">理一理材料</span>
        <p className="min-w-0 flex-1 truncate text-[13px] text-mk-muted">搜集了一些材料后，让印记帮你看看哪些更相关、更重要。</p>
        <button
          type="button"
          onClick={() => void run()}
          disabled={loading}
          className="flex-none rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
        >
          {review ? "再理一次" : "让印记帮我理一理"}
        </button>
      </div>
      {loading && <div className="mt-2.5"><ReviewingHint /></div>}
      {review && !loading && (
        <p className="mt-2.5 whitespace-pre-wrap rounded-mk border border-mk-border bg-mk-paper px-3 py-2 text-[13.5px] leading-relaxed text-mk-ink">{review}</p>
      )}
      {review === "" && !loading && (
        <p className="mt-2 text-[12px] text-mk-faint">这次没理出结果——可能材料还太少，先多搜几篇、标好支持/反驳再来。</p>
      )}
    </div>
  );
}
