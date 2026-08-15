import type { RuntimeSceneResult } from "@mind-imprint/course-contract";
import { apiFetch } from "./client";

// courseScene.ts — client for the runtime Opening/Closing scene generation
// endpoint (Course Runtime Slice 7). The server generates the narration text,
// synthesizes audio when configured, and — critically — echoes the caller's
// authored `fallback` text (fallbackUsed=true) instead of erroring when
// generation fails, so this client only has to translate a hard network
// failure into a fallback (see makeApiSceneGenerator).

/** Teacher-approved facts for one scene; only the slot's fields are populated. */
export interface SceneFactsBody {
  title?: string;
  estimatedMinutes?: number;
  objectives?: string[];
  learningPreview?: string[];
  preparedSummary?: string;
  takeaways?: string[];
  transferApplications?: string[];
}

/** POST body for /courses/{slug}/scene — matches the Go handler's sceneRequest. */
export interface SceneRequestBody {
  which: "opening" | "closing";
  facts: SceneFactsBody;
  allowedSignals: string[];
  signalEvidence: Record<string, string>;
  fallback: { text: string; audioUrl?: string };
}

/** POST /api/v1/courses/{slug}/scene → { result: RuntimeSceneResult }. */
export async function apiScene(slug: string, body: SceneRequestBody): Promise<{ result: RuntimeSceneResult }> {
  return apiFetch<{ result: RuntimeSceneResult }>(`/api/v1/courses/${slug}/scene`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}
