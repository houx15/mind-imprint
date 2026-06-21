import type { Evaluation, RubricDimension, SoloLevel } from "@mind-imprint/contracts";
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

const LEVEL_NUMBER: Record<SoloLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };

export function evalView(evaluation: Evaluation, rubric: RubricDimension[]): EvalView {
  const dims: EvalDimView[] = evaluation.scores.map((score) => {
    const rubricEntry = rubric.find((r) => r.id === score.dim_id);
    const dim = rubricEntry ? rubricEntry.name : score.dim_id;
    const levelLabel = `${score.level} · ${SOLO_LABELS[score.level]}`;
    const n = LEVEL_NUMBER[score.level];
    const segs = [1, 2, 3, 4].map((i) => ({ filled: i <= n }));
    return { dim, levelLabel, segs, note: score.note };
  });

  return { dims };
}
