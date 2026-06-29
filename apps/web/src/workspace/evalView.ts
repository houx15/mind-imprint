import type { Evaluation, RubricDimension, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";

export interface EvalDimView {
  dim: string;
  levelLabel: string;
  segs: Array<{ filled: boolean }>;
  note: string;
}

export interface EvalView {
  dims: EvalDimView[];
}

const LEVEL_NUMBER: Record<ScoredLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };

export function evalView(evaluation: Evaluation, rubric: RubricDimension[]): EvalView {
  const dims: EvalDimView[] = evaluation.scores
    .filter((score) => score.level !== "NA")
    .map((score) => {
      const level = score.level as ScoredLevel;
      const rubricEntry = rubric.find((r) => r.id === score.dim_id);
      const dim = rubricEntry ? rubricEntry.name : score.dim_id;
      const levelLabel = `${level} · ${SOLO_LABELS[level]}`;
      const n = LEVEL_NUMBER[level];
      const segs = [1, 2, 3, 4].map((i) => ({ filled: i <= n }));
      return { dim, levelLabel, segs, note: score.note };
    });

  return { dims };
}
