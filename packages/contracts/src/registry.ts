import { CardSpec } from "./cardSpec";
import siftCraap from "../cards/sift_craap.json";
import concession from "../cards/concession.json";
// —— library cards (Slice 3a) ——
import aiBoundary from "../cards/ai-boundary.json";
import aiCollaboration from "../cards/ai-collaboration.json";
import aiDecisionTree from "../cards/ai-decision-tree.json";
import aokMethods from "../cards/aok-methods.json";
import argumentMap from "../cards/argument-map.json";
import beliefSpectrum from "../cards/belief-spectrum.json";
import cda from "../cards/cda.json";
import certaintySpectrum from "../cards/certainty-spectrum.json";
import checkpoint from "../cards/checkpoint.json";
import corpusHook from "../cards/corpus-hook.json";
import craap from "../cards/craap.json";
import dataLiteracy from "../cards/data-literacy.json";
import emotionalAlignment from "../cards/emotional-alignment.json";
import ethicsLenses from "../cards/ethics-lenses.json";
import ethicsRoleplay from "../cards/ethics-roleplay.json";
import factOpinionValue from "../cards/fact-opinion-value.json";
import framing from "../cards/framing.json";
import knowerPerspective from "../cards/knower-perspective.json";
import learningReport from "../cards/learning-report.json";
import metacognition from "../cards/metacognition.json";
import moneyTrail from "../cards/money-trail.json";
import multimodalDecode from "../cards/multimodal-decode.json";
import opcvl from "../cards/opcvl.json";
import pee from "../cards/pee.json";
import questionCard from "../cards/question-card.json";
import rabbitHole from "../cards/rabbit-hole.json";
import scienceKnowing from "../cards/science-knowing.json";
import sift from "../cards/sift.json";
import sourceMap from "../cards/source-map.json";
import spinDetector from "../cards/spin-detector.json";
import steelman from "../cards/steelman.json";

const DEFAULT_RAW: Record<string, unknown> = {
  // demo cards
  sift_craap: siftCraap,
  concession,
  // library cards
  "ai-boundary": aiBoundary,
  "ai-collaboration": aiCollaboration,
  "ai-decision-tree": aiDecisionTree,
  "aok-methods": aokMethods,
  "argument-map": argumentMap,
  "belief-spectrum": beliefSpectrum,
  cda,
  "certainty-spectrum": certaintySpectrum,
  checkpoint,
  "corpus-hook": corpusHook,
  craap,
  "data-literacy": dataLiteracy,
  "emotional-alignment": emotionalAlignment,
  "ethics-lenses": ethicsLenses,
  "ethics-roleplay": ethicsRoleplay,
  "fact-opinion-value": factOpinionValue,
  framing,
  "knower-perspective": knowerPerspective,
  "learning-report": learningReport,
  metacognition,
  "money-trail": moneyTrail,
  "multimodal-decode": multimodalDecode,
  opcvl,
  pee,
  "question-card": questionCard,
  "rabbit-hole": rabbitHole,
  "science-knowing": scienceKnowing,
  sift,
  "source-map": sourceMap,
  "spin-detector": spinDetector,
  steelman,
};

export type CatalogEntry = {
  id: string; category: string; name: string; trigger_condition: string;
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
    id: c.id, category: c.category, name: c.name, trigger_condition: c.trigger_condition,
    trigger_keywords: c.trigger_keywords, disclosure_tier: c.disclosure_tier,
    priority: c.priority, interaction_type: c.interaction_type,
  }));
}

export const CARD_REGISTRY = loadRegistry();
