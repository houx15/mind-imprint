import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { CoursePlayer } from "@mind-imprint/course-renderer";
import type { CourseRuntimeAdapters } from "@mind-imprint/course-runtime";
import { collectAssetPaths } from "@mind-imprint/course-contract";
import type { CourseDefinitionDocument } from "@mind-imprint/course-contract";
import { getCourseDefinition } from "@/api/courseDefinition";
import { fetchCourseAssetUrls } from "@/api/courseAssetUrls";
import { ApiError } from "@/api/client";
import { makeCdnAssetResolver } from "@/course/assetResolver";
import { makeApiSessionAdapter } from "@/course/apiSessionAdapter";
import { makeApiSceneGenerator } from "@/course/apiSceneGenerator";

// RuntimeCoursePlayer — mounts the @mind-imprint/course-renderer CoursePlayer
// (Course Runtime 2.0) for a course that HAS a stored 2.0 definition. This is
// the HOST BOUNDARY: real time and ids enter here (idFactory/clock), keeping the
// runtime packages pure. It builds the three production adapters + the Slice 7
// scene generators, fetches the definition, and plays it. Completion fires
// onFinish ONLY via CoursePlayer's own `onComplete` callback (P1-03) — the
// learner dismissing the Closing scene, not the session status flip, which
// the player can reach well before the learner has actually seen/heard
// Closing. A course WITHOUT a 2.0 definition never reaches here —
// CoursesContainer routes it to the legacy player (see CoursesContainer).

// The runtime never dictates identity; the server mints the real studentId from
// the authed user on create. This value is advisory, so a placeholder is safe
// when the app has no user id handy.
const PLACEHOLDER_STUDENT_ID = "current-student";

// P2-05 — resolves the Opening's allowed history signals (the course's own
// "recent-course-topics" / "prior-objective-performance") at this
// authenticated host boundary. There is no dedicated Course Runtime 2.0
// history-signals endpoint yet, so this intentionally returns no values today
// rather than mining an unrelated endpoint (e.g. the legacy course catalog)
// into a signal it doesn't actually represent — a fabricated value would be
// worse than none. This IS the typed extension point CoursePlayer calls: once
// a real signals endpoint exists, resolve it here and CoursePlayer itself
// stays signal-source-agnostic.
function resolveOpeningSignals(_allowedSignals: string[]): Record<string, unknown> {
  return {};
}

export function RuntimeCoursePlayer({
  slug,
  studentId,
  onExit,
  onFinish,
}: {
  slug: string;
  studentId?: string;
  onExit: () => void;
  onFinish: () => void;
}) {
  const [document, setDocument] = useState<unknown | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Bumped on asset-url refresh so consumers (e.g. adapters.assetResolver
  // callers inside the renderer) re-resolve; the map itself lives in the ref
  // below so refreshing it never rebuilds `adapters`.
  const [, setRefreshTick] = useState(0);

  // Keep onFinish fresh without rebuilding the adapters (which own the live
  // session state) on every render.
  const onFinishRef = useRef(onFinish);
  onFinishRef.current = onFinish;

  // The sessionAdapter field on `adapters` is typed to the runtime's narrower
  // SessionAdapter (no `flush`) — this ref keeps the concrete adapter's flush
  // reachable for the lifecycle-exit handling below (P2-07). Reassigned inside
  // the `adapters` useMemo, which always runs before this ref is read.
  const flushRef = useRef<() => Promise<void>>(() => Promise.resolve());

  // Signed CDN asset-URL map, read live by the resolver via a ref getter so a
  // refresh (re-signing before `expiresAt`) never rebuilds `adapters` below.
  const assetUrlsRef = useRef<Record<string, string>>({});

  // Built once per slug: the sessionAdapter holds the authoritative session
  // client-side, so it must survive re-renders. Persists status normally,
  // unwrapped (P1-03) — completion is surfaced ONLY via CoursePlayer's own
  // `onComplete` prop below, never inferred from a `setStatus("completed")`
  // write. The assetResolver reads assetUrlsRef live, so refreshing the
  // signed map never rebuilds this object.
  const adapters = useMemo<CourseRuntimeAdapters>(() => {
    const sessionAdapter = makeApiSessionAdapter(slug);
    flushRef.current = () => sessionAdapter.flush();
    return {
      assetResolver: makeCdnAssetResolver(() => assetUrlsRef.current),
      sessionAdapter,
      openingGenerator: makeApiSceneGenerator(slug),
      closingGenerator: makeApiSceneGenerator(slug),
    };
  }, [slug]);

  // P2-07: flush the session adapter's pending snapshot on every lifecycle
  // exit a student might take instead of a clean unmount — backgrounding the
  // tab (`visibilitychange` → hidden), closing/navigating away (`pagehide`),
  // and this component's own unmount. `window.document` (not the bare
  // `document` identifier) because the `document` local state var above
  // shadows the global DOM `document` within this component's scope.
  useEffect(() => {
    const flushNow = () => {
      void flushRef.current().catch(() => {
        // Best-effort on an exit path — the adapter's own retry/backoff
        // keeps trying; there's no UI left to report to here.
      });
    };
    const handleVisibility = () => {
      if (window.document.visibilityState === "hidden") flushNow();
    };
    window.addEventListener("pagehide", flushNow);
    window.document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      window.removeEventListener("pagehide", flushNow);
      window.document.removeEventListener("visibilitychange", handleVisibility);
      flushNow();
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    setDocument(null);
    setError(null);
    assetUrlsRef.current = {};

    const REFRESH_LEAD_MS = 5 * 60_000; // re-sign 5 min before the window lapses
    const REFRESH_RETRY_MS = 60_000; // after a transient failure, retry this soon
    // Schedule the next re-sign `delayMs` from now. Both the success and the
    // failure branch re-arm the timer, so a single transient refresh failure
    // retries instead of giving up — otherwise every asset would 403 for the
    // rest of a session that outlives the URL鉴权 window.
    const armRefresh = (assetSlug: string, paths: string[], delayMs: number) => {
      refreshTimer = setTimeout(() => {
        void (async () => {
          try {
            const next = await fetchCourseAssetUrls(assetSlug, paths);
            if (cancelled) return;
            assetUrlsRef.current = next.assetUrls;
            setRefreshTick((t) => t + 1); // re-render so renderers re-resolve
            const lead = new Date(next.expiresAt).getTime() - Date.now() - REFRESH_LEAD_MS;
            armRefresh(assetSlug, paths, Math.max(lead, REFRESH_RETRY_MS));
          } catch {
            if (cancelled) return;
            armRefresh(assetSlug, paths, REFRESH_RETRY_MS); // keep the stale map, retry soon
          }
        })();
      }, delayMs);
    };

    (async () => {
      try {
        const doc = (await getCourseDefinition(slug)) as CourseDefinitionDocument;
        if (cancelled) return;
        const paths = collectAssetPaths(doc);
        if (paths.length > 0) {
          const signed = await fetchCourseAssetUrls(slug, paths);
          if (cancelled) return;
          assetUrlsRef.current = signed.assetUrls;
          const lead = new Date(signed.expiresAt).getTime() - Date.now() - REFRESH_LEAD_MS;
          armRefresh(slug, paths, Math.max(lead, REFRESH_RETRY_MS));
        }
        setDocument(doc);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof ApiError ? e.message : "课程定义加载失败");
      }
    })();

    return () => {
      cancelled = true;
      if (refreshTimer) clearTimeout(refreshTimer);
    };
  }, [slug]);

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%", background: "var(--mk-paper)" }}>
      <div style={{ flex: "none", background: "var(--mk-surface)", borderBottom: "1px solid var(--mk-border)" }}>
        <div style={{ height: 50, display: "flex", alignItems: "center", padding: "0 20px", gap: 8 }}>
          <button
            type="button"
            onClick={onExit}
            style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--mk-secondary)", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: "6px 10px", borderRadius: 8, background: "transparent", border: "none", fontFamily: "inherit" }}
          >
            <ArrowLeft size={15} strokeWidth={2.2} />
            返回课程
          </button>
        </div>
      </div>

      <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
        {error ? (
          <div style={{ padding: 40, color: "var(--mk-secondary)", fontSize: 14 }}>{error}</div>
        ) : document ? (
          <CoursePlayer
            document={document}
            adapters={adapters}
            studentId={studentId ?? PLACEHOLDER_STUDENT_ID}
            idFactory={() => crypto.randomUUID()}
            clock={() => new Date().toISOString()}
            onComplete={() => onFinishRef.current()}
            signalResolver={resolveOpeningSignals}
          />
        ) : (
          <div aria-busy="true" style={{ padding: 40, color: "var(--mk-faint)", fontSize: 14 }}>
            正在加载课程…
          </div>
        )}
      </div>
    </div>
  );
}
