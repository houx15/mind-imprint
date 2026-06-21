import type { Evaluation, RubricDimension, SoloLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";

const LEVEL_NUMBER: Record<SoloLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };
const SEG_FILLED = "flex:1; height:6px; border-radius:999px; background:#2A3B7A;";
const SEG_EMPTY = "flex:1; height:6px; border-radius:999px; background:#EEF0F4;";
const DOT = "width:9px; height:9px; border-radius:50%; background:#2A3B7A; display:inline-block;";

export interface AbilityDim {
  dimId: string; dim: string; level: SoloLevel; levelLabel: string;
  segs: { style: string }[]; dotStyle: string;
}

export function deriveAbility(evaluations: Evaluation[], rubric: RubricDimension[]): AbilityDim[] {
  // latest score per dim id
  const latest = new Map<string, { level: SoloLevel; at: string }>();
  for (const e of evaluations) {
    for (const s of e.scores) {
      const prev = latest.get(s.dim_id);
      if (!prev || e.created_at > prev.at) latest.set(s.dim_id, { level: s.level, at: e.created_at });
    }
  }
  return rubric.map((d) => {
    const level = latest.get(d.id)?.level ?? "L1";
    const n = LEVEL_NUMBER[level];
    const segs = [1, 2, 3, 4].map((i) => ({ style: i <= n ? SEG_FILLED : SEG_EMPTY }));
    return { dimId: d.id, dim: d.name, level, levelLabel: `${level} · ${SOLO_LABELS[level]}`, segs, dotStyle: DOT };
  });
}
