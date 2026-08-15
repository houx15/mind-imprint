import type {
  RuntimeSceneGenerator,
  SceneGenerationInput,
} from "@mind-imprint/course-runtime";
import type { RuntimeSceneResult } from "@mind-imprint/course-contract";
import { apiScene, type SceneRequestBody } from "@/api/courseScene";

// apiSceneGenerator.ts — the API-backed RuntimeSceneGenerator (Course Runtime
// Slice 7, implementing the Slice 2 interface). It translates a CourseDefinition
// Opening/Closing input plus the permitted session signals into the scene
// endpoint's request body and returns the server's RuntimeSceneResult. The
// server already handles LLM/TTS failure by returning the authored fallback
// (fallbackUsed=true); this adapter only has to catch a HARD network failure —
// which it turns into the same fallback result rather than throwing into the
// player (the runtime must never stall on a failed scene, §6.1/§6.2).

/**
 * Flattens the permitted signal map into string evidence, dropping any signal
 * with no usable value. Only `allowedSignals` are considered (§20 signal
 * minimization) — a value present in the map but not permitted never leaks.
 */
function toEvidence(allowed: string[], values: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const key of allowed) {
    const raw = values[key];
    if (raw === undefined || raw === null) continue;
    const str = typeof raw === "string" ? raw : JSON.stringify(raw);
    if (str.trim() === "") continue;
    out[key] = str;
  }
  return out;
}

/** Builds the endpoint request body from the runtime's scene input. */
function toRequestBody(input: SceneGenerationInput): SceneRequestBody {
  if (input.which === "opening") {
    return {
      which: "opening",
      facts: {
        title: input.title,
        estimatedMinutes: input.estimatedMinutes,
        objectives: input.objectives,
        learningPreview: input.learningPreview,
      },
      allowedSignals: input.allowedSignals,
      signalEvidence: toEvidence(input.allowedSignals, input.signalValues),
      fallback: { text: input.fallback.text, audioUrl: input.fallback.audioUrl },
    };
  }
  return {
    which: "closing",
    facts: {
      preparedSummary: input.preparedSummary,
      takeaways: input.takeaways,
      transferApplications: input.transferApplications,
    },
    allowedSignals: input.allowedSignals,
    signalEvidence: toEvidence(input.allowedSignals, input.sessionEvidence),
    fallback: { text: input.fallback.text, audioUrl: input.fallback.audioUrl },
  };
}

/** The authored fallback result, used only when the network call itself fails. */
function fallbackResult(input: SceneGenerationInput): RuntimeSceneResult {
  const result: RuntimeSceneResult = {
    text: input.fallback.text,
    generatedAt: new Date().toISOString(),
    usedSignalTypes: [],
    fallbackUsed: true,
  };
  if (input.fallback.audioUrl !== undefined) result.audioUrl = input.fallback.audioUrl;
  return result;
}

/**
 * makeApiSceneGenerator returns a RuntimeSceneGenerator bound to `slug`.
 * generate() never rejects: a network failure resolves to the authored
 * fallback (fallbackUsed=true) so the player always has usable narration.
 */
export function makeApiSceneGenerator(slug: string): RuntimeSceneGenerator {
  return {
    async generate(input: SceneGenerationInput): Promise<RuntimeSceneResult> {
      try {
        const { result } = await apiScene(slug, toRequestBody(input));
        return result;
      } catch {
        return fallbackResult(input);
      }
    },
  };
}
