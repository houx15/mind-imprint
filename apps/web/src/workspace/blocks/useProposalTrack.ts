import { useCallback, useEffect, useState } from "react";
import type { ProposalGuideStep, SubQuestion } from "@mind-imprint/contracts";
import {
  getProposalTrack,
  setProposalMode,
  startProposalGuide,
  setSubQuestions,
  advanceProposalStep,
  jumpProposalStep,
} from "../../api/proposalTrack";

// useProposalTrack — slice 3a · owns the proposal guide-step track for one
// project. Every action returns the fresh ProposalGuideStep (server-derived), so
// the hook just stores what the server sends. The api functions are injectable
// (deps) so the hook is testable without a network or vi.mock.
export type ProposalTrackDeps = {
  getTrack: typeof getProposalTrack;
  setMode: typeof setProposalMode;
  start: typeof startProposalGuide;
  saveSubQuestions: typeof setSubQuestions;
  advance: typeof advanceProposalStep;
};

const realDeps: ProposalTrackDeps = {
  getTrack: getProposalTrack,
  setMode: setProposalMode,
  start: startProposalGuide,
  saveSubQuestions: setSubQuestions,
  advance: advanceProposalStep,
};

export function useProposalTrack(projectId: string, deps: ProposalTrackDeps = realDeps) {
  const [step, setStep] = useState<ProposalGuideStep | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    try {
      setStep(await deps.getTrack(projectId));
    } catch {
      /* leave prior step; the surface degrades to the free writing pane */
    } finally {
      setLoading(false);
    }
  }, [deps, projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const chooseMode = useCallback(
    async (m: "free" | "guided") => setStep(await deps.setMode(projectId, m)),
    [deps, projectId],
  );
  const start = useCallback(async () => setStep(await deps.start(projectId)), [deps, projectId]);
  const saveSubQuestions = useCallback(
    async (list: SubQuestion[]) => setStep(await deps.saveSubQuestions(projectId, list)),
    [deps, projectId],
  );
  const next = useCallback(async () => setStep(await deps.advance(projectId, "next")), [deps, projectId]);
  const prev = useCallback(async () => setStep(await deps.advance(projectId, "prev")), [deps, projectId]);
  const jump = useCallback(async (to: number) => setStep(await jumpProposalStep(projectId, to)), [projectId]);

  return { step, loading, chooseMode, start, saveSubQuestions, next, prev, jump, reload };
}
