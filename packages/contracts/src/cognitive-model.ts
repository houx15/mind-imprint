import type { Evaluation } from "./evaluation";
import type { SoloLevel } from "./rubric";
import { FULL_RUBRIC } from "./rubric";

export interface Category { id: string; label: string; dimIds: string[]; }
export interface Face { id: string; label: string; icon: string; categories: Category[]; }

// Locked mapping (eval-model spec §2). Every D1..D10 appears exactly once.
export const COGNITIVE_MODEL: Face[] = [
  {
    id: "driving", label: "生成式驾驭", icon: "🚀",
    categories: [
      { id: "intent", label: "意图与编排", dimIds: ["D1", "D10"] },
      { id: "reasoning", label: "推理与论证", dimIds: ["D4", "D5", "D7"] },
    ],
  },
  {
    id: "guarding", label: "批判式防护", icon: "🛡️",
    categories: [
      { id: "literacy", label: "信息素养", dimIds: ["D2", "D3"] },
      { id: "metacognition", label: "AI 元认知与边界", dimIds: ["D6", "D8", "D9"] },
    ],
  },
];

export interface AssembledDim { dimId: string; name: string; level: SoloLevel; note: string; }
export interface AssembledCategory { id: string; label: string; dims: AssembledDim[]; scored: number; na: number; }
export interface AssembledFace { id: string; label: string; icon: string; categories: AssembledCategory[]; scored: number; na: number; }
export interface AssembledImprint { faces: AssembledFace[]; }

const NAME_BY_ID = new Map(FULL_RUBRIC.map((d) => [d.id, d.name]));

export function assembleImprint(evaluation: Evaluation): AssembledImprint {
  const scoreById = new Map(evaluation.scores.map((s) => [s.dim_id, s]));

  const faces: AssembledFace[] = COGNITIVE_MODEL.map((face) => {
    const categories: AssembledCategory[] = face.categories.map((cat) => {
      const dims: AssembledDim[] = cat.dimIds.map((dimId) => {
        const score = scoreById.get(dimId);
        return {
          dimId,
          name: NAME_BY_ID.get(dimId) ?? dimId,
          level: score ? score.level : "NA",   // absent dim → NA
          note: score ? score.note : "",
        };
      });
      const scored = dims.filter((d) => d.level !== "NA").length;
      return { id: cat.id, label: cat.label, dims, scored, na: dims.length - scored };
    });
    const scored = categories.reduce((sum, c) => sum + c.scored, 0);
    const na = categories.reduce((sum, c) => sum + c.na, 0);
    return { id: face.id, label: face.label, icon: face.icon, categories, scored, na };
  });

  return { faces };
}
