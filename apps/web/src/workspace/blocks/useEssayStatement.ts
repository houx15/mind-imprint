import { useCallback, useEffect, useState } from "react";
import type { ProposalGuideStep } from "@mind-imprint/contracts";
import {
  getEssayStatement,
  startEssayStatement,
  advanceEssayStatement,
} from "../../api/essayStatement";

// useEssayStatement — slice 4b · owns the essay statement-track walk (mirrors
// useProposalTrack). Injectable deps for testing.
export type EssayStatementDeps = {
  getStep: typeof getEssayStatement;
  start: typeof startEssayStatement;
  advance: typeof advanceEssayStatement;
};

const realDeps: EssayStatementDeps = {
  getStep: getEssayStatement,
  start: startEssayStatement,
  advance: advanceEssayStatement,
};

export function useEssayStatement(projectId: string, deps: EssayStatementDeps = realDeps) {
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
