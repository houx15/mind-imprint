import {
  VideoInteractionDocument as VideoInteractionDocumentSchema,
  type VideoInteractionDocument,
} from "@mind-imprint/course-contract";
import { InteractionLoadError, type InteractionLoader } from "@mind-imprint/course-renderer";

// interactionLoader.ts — the production InteractionLoader (Course Runtime
// hardening Slice 3, P1-01). A Video Block's `interaction.source` is one of the
// course's assets (collectAssetPaths includes it), so its signed CDN URL is
// already in the live asset-url map. This loader resolves that URL, fetches +
// structurally parses the VideoInteractionDocument, and caches it by source.
//
// Division of labour: this host loader owns TRANSPORT + STRUCTURAL parse and
// maps failures to the typed error kinds `not-found` (missing URL / fetch
// failure / non-2xx incl. an expired 403) and `malformed` (non-JSON / schema
// mismatch). BLOCK-level referential validation (`invalid` / `mismatch` —
// does the doc's video.source match the owning block?) is done by the renderer's
// VideoInteractionController, which holds the owning VideoBlock; it runs
// validateVideoInteraction after this loader resolves. The loader therefore
// never needs the block, and never throws a raw error — always an
// InteractionLoadError the renderer surfaces as a recoverable error state.

/**
 * makeInteractionLoader builds the InteractionLoader over a live getter for the
 * signed asset-url map (so a refreshed URL is used when a failed load is
 * retried — a cached success is returned without refetching).
 */
export function makeInteractionLoader(getAssetUrls: () => Record<string, string>): InteractionLoader {
  const cache = new Map<string, VideoInteractionDocument>();
  return async (source: string): Promise<VideoInteractionDocument> => {
    const cached = cache.get(source);
    if (cached) return cached;

    const url = getAssetUrls()[source];
    if (!url) {
      throw new InteractionLoadError("not-found", `no signed URL for interaction asset "${source}"`);
    }

    let res: Response;
    try {
      res = await fetch(url);
    } catch (e) {
      throw new InteractionLoadError("not-found", `fetch failed for interaction asset "${source}": ${String(e)}`);
    }
    if (!res.ok) {
      throw new InteractionLoadError("not-found", `interaction asset "${source}" returned HTTP ${res.status}`);
    }

    let json: unknown;
    try {
      json = await res.json();
    } catch {
      throw new InteractionLoadError("malformed", `interaction asset "${source}" is not valid JSON`);
    }

    const parsed = VideoInteractionDocumentSchema.safeParse(json);
    if (!parsed.success) {
      throw new InteractionLoadError(
        "malformed",
        `interaction asset "${source}" does not match the schema: ${parsed.error.issues.map((i) => i.message).join("; ")}`,
      );
    }

    cache.set(source, parsed.data);
    return parsed.data;
  };
}
