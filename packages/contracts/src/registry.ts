import { CardSpec } from "./cardSpec";
import concession from "../cards/concession.json";
// —— library cards (Slice 3a) ——
import aiBoundary from "../cards/ai-boundary.json";
import aiDecisionTree from "../cards/ai-decision-tree.json";
import argumentMap from "../cards/argument-map.json";
import beliefSpectrum from "../cards/belief-spectrum.json";
import cda from "../cards/cda.json";
import craap from "../cards/craap.json";
import dataLiteracy from "../cards/data-literacy.json";
import emotionalAlignment from "../cards/emotional-alignment.json";
import ethicsLenses from "../cards/ethics-lenses.json";
import factOpinionValue from "../cards/fact-opinion-value.json";
import knowerPerspective from "../cards/knower-perspective.json";
import learningReport from "../cards/learning-report.json";
import metacognition from "../cards/metacognition.json";
import moneyTrail from "../cards/money-trail.json";
import multimodalDecode from "../cards/multimodal-decode.json";
import opcvl from "../cards/opcvl.json";
import pee from "../cards/pee.json";
import perspectiveMatrix from "../cards/perspective-matrix.json";
import questionCard from "../cards/question-card.json";
import rabbitHole from "../cards/rabbit-hole.json";
import searchPlan from "../cards/search-plan.json";
import sift from "../cards/sift.json";
import spinDetector from "../cards/spin-detector.json";
import toulmin from "../cards/toulmin.json";
// —— reading lenses (学科透镜): the reading-room deck; see 2026-07-29-reading-lenses-adopt-demo.md ——
import lensLogic from "../cards/lens-logic.json";
import lensMethods from "../cards/lens-methods.json";
import lensSociety from "../cards/lens-society.json";
import lensLaw from "../cards/lens-law.json";
import lensEconomics from "../cards/lens-economics.json";
import lensEthics from "../cards/lens-ethics.json";
import lensHistory from "../cards/lens-history.json";
import lensCommunication from "../cards/lens-communication.json";
import lensSystems from "../cards/lens-systems.json";

const DEFAULT_RAW: Record<string, unknown> = {
  // demo cards
  concession,
  // library cards
  "ai-boundary": aiBoundary,
  "ai-decision-tree": aiDecisionTree,
  "argument-map": argumentMap,
  "belief-spectrum": beliefSpectrum,
  cda,
  craap,
  "data-literacy": dataLiteracy,
  "emotional-alignment": emotionalAlignment,
  "ethics-lenses": ethicsLenses,
  "fact-opinion-value": factOpinionValue,
  "knower-perspective": knowerPerspective,
  "learning-report": learningReport,
  metacognition,
  "money-trail": moneyTrail,
  "multimodal-decode": multimodalDecode,
  opcvl,
  pee,
  "perspective-matrix": perspectiveMatrix,
  "question-card": questionCard,
  "rabbit-hole": rabbitHole,
  "search-plan": searchPlan,
  sift,
  "spin-detector": spinDetector,
  toulmin,
  // reading lenses
  "lens-logic": lensLogic,
  "lens-methods": lensMethods,
  "lens-society": lensSociety,
  "lens-law": lensLaw,
  "lens-economics": lensEconomics,
  "lens-ethics": lensEthics,
  "lens-history": lensHistory,
  "lens-communication": lensCommunication,
  "lens-systems": lensSystems,
};

export type CatalogEntry = {
  id: string; category: string; name: string; purpose: string; trigger_condition: string;
  trigger_keywords?: string[]; disclosure_tier?: string; priority?: string; interaction_type?: string;
};
export type Catalog = CatalogEntry[];

export function loadRegistry(raw: Record<string, unknown> = DEFAULT_RAW): Record<string, CardSpec> {
  const out: Record<string, CardSpec> = {};
  for (const [key, data] of Object.entries(raw)) {
    const parsed = CardSpec.safeParse(data);
    if (!parsed.success) {
      const detail = parsed.error.issues.map((i) => `${i.path.join(".")}: ${i.message}`).join("; ");
      throw new Error(`Invalid card "${key}": ${detail}`);
    }
    if (parsed.data.id !== key) {
      throw new Error(`Card key "${key}" does not match card.id "${parsed.data.id}"`);
    }
    out[key] = parsed.data;
  }
  return out;
}

export function deriveCatalog(registry: Record<string, CardSpec>): Catalog {
  return Object.values(registry).map((c) => ({
    id: c.id, category: c.category, name: c.name, purpose: c.purpose, trigger_condition: c.trigger_condition,
    trigger_keywords: c.trigger_keywords, disclosure_tier: c.disclosure_tier,
    priority: c.priority, interaction_type: c.interaction_type,
  }));
}

export const CARD_REGISTRY = loadRegistry();
