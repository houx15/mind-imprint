import { useCallback, useEffect, useState } from "react";
import type { ProposalGuideStep } from "@mind-imprint/contracts";
import {
  getEssaySubmission,
  startEssaySubmission,
  advanceEssaySubmission,
} from "../../api/essaySubmission";

// useEssaySubmission — slice 4c · owns the essay submission-track walk (mirrors
// useEssayStatement). Injectable deps for testing.
export type EssaySubmissionDeps = {
  getStep: typeof getEssaySubmission;
  start: typeof startEssaySubmission;
  advance: typeof advanceEssaySubmission;
};

const realDeps: EssaySubmissionDeps = {
  getStep: getEssaySubmission,
  start: startEssaySubmission,
  advance: advanceEssaySubmission,
};

export function useEssaySubmission(projectId: string, deps: EssaySubmissionDeps = realDeps) {
  const [step, setStep] = useState<ProposalGuideStep | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    try {
      setStep(await deps.getStep(projectId));
    } catch {
      /* leave prior */
    } finally {
      setLoading(false);
    }
  }, [deps, projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const start = useCallback(async () => setStep(await deps.start(projectId)), [deps, projectId]);
  const next = useCallback(async () => setStep(await deps.advance(projectId, "next")), [deps, projectId]);
  const prev = useCallback(async () => setStep(await deps.advance(projectId, "prev")), [deps, projectId]);

  return { step, loading, start, next, prev, reload };
}
