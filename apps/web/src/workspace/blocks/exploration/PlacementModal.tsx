import { useEffect, useState } from "react";
import { getExploration, suggestPlacement, attachReference } from "../../../api/exploration";
import { PlacementPicker, type PlacementQuestion } from "./PlacementPicker";

// PlacementModal — 加来源即归位. After a reference is created, ask 印记 for the
// best-fit question (pre-highlighted) and let the student place it (or 未归类).
// If the project has no questions yet, there's nothing to place under → this
// self-closes without showing anything (the source stays in 未归类).
export function PlacementModal({
  projectId,
  referenceId,
  onClose,
  onAttached,
}: {
  projectId: string;
  referenceId: string;
  onClose: () => void;
  onAttached?: () => void;
}) {
  const [questions, setQuestions] = useState<PlacementQuestion[]>([]);
  const [suggestedLeadId, setSuggestedLeadId] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const view = await getExploration(projectId);
        if (cancelled) return;
        const qs = view.leads
          .filter((l) => l.status !== "pruned" && l.connectedReferenceId == null)
          .map((l) => ({ id: l.id, text: l.text, parentId: l.parentLeadId }));
        // No questions yet (the common early-exploration state): there's nothing
        // to place under, so don't force an empty modal or spend a suggest call —
        // just close; the source stays in 未归类 (final-review #1).
        if (qs.length === 0) {
          onClose();
          return;
        }
        setQuestions(qs);
        // Suggestion is best-effort — a failure just means no pre-highlight.
        try {
          const s = await suggestPlacement(projectId, referenceId);
          if (!cancelled) {
            setSuggestedLeadId(s.leadId);
            setReason(s.reason);
          }
        } catch {
          /* no suggestion; manual pick still works */
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId, referenceId]);

  async function pick(leadId: string | null) {
    if (busy) return;
    if (leadId == null) {
      onClose(); // 未归类 — leave it unattached
      return;
    }
    setBusy(true);
    try {
      await attachReference(projectId, referenceId, leadId);
      onAttached?.();
      onClose();
    } catch {
      setBusy(false); // let the student retry / choose 未归类
    }
  }

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/40 px-6" onClick={onClose}>
      <div className="w-[440px] rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">这篇挂到哪个问题下？</h3>
          <button type="button" onClick={onClose} className="text-[18px] leading-none text-mk-faint hover:text-mk-ink">×</button>
        </div>
        {loading ? (
          <p className="py-6 text-center text-[13px] text-mk-faint">印记在想它属于哪儿…</p>
        ) : (
          <PlacementPicker questions={questions} suggestedLeadId={suggestedLeadId} reason={reason} busy={busy} onPick={pick} />
        )}
      </div>
    </div>
  );
}
