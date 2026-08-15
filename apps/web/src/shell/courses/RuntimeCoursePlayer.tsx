import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { CoursePlayer } from "@mind-imprint/course-renderer";
import type { CourseRuntimeAdapters, SessionAdapter } from "@mind-imprint/course-runtime";
import { getCourseDefinition } from "@/api/courseDefinition";
import { ApiError } from "@/api/client";
import { makeCdnAssetResolver } from "@/course/assetResolver";
import { makeApiSessionAdapter } from "@/course/apiSessionAdapter";
import { makeApiSceneGenerator } from "@/course/apiSceneGenerator";

// RuntimeCoursePlayer — mounts the @mind-imprint/course-renderer CoursePlayer
// (Course Runtime 2.0) for a course that HAS a stored 2.0 definition. This is
// the HOST BOUNDARY: real time and ids enter here (idFactory/clock), keeping the
// runtime packages pure. It builds the three production adapters + the Slice 7
// scene generators, fetches the definition, and plays it. Completion (the
// session reaching `completed`) fires onFinish. A course WITHOUT a 2.0
// definition never reaches here — CoursesContainer routes it to the legacy
// player (see CoursesContainer).

// The runtime never dictates identity; the server mints the real studentId from
// the authed user on create. This value is advisory, so a placeholder is safe
// when the app has no user id handy.
const PLACEHOLDER_STUDENT_ID = "current-student";

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

  // Keep onFinish fresh without rebuilding the adapters (which own the live
  // session state) on every render.
  const onFinishRef = useRef(onFinish);
  onFinishRef.current = onFinish;

  // Built once per slug: the sessionAdapter holds the authoritative session
  // client-side, so it must survive re-renders. setStatus is wrapped so the
  // terminal `completed` transition surfaces as onFinish (the renderer has no
  // completion callback of its own).
  const adapters = useMemo<CourseRuntimeAdapters>(() => {
    const base = makeApiSessionAdapter(slug);
    const sessionAdapter: SessionAdapter = {
      ...base,
      async setStatus(sessionId, status) {
        await base.setStatus(sessionId, status);
        if (status === "completed") onFinishRef.current();
      },
    };
    return {
      assetResolver: makeCdnAssetResolver({ slug }),
      sessionAdapter,
      openingGenerator: makeApiSceneGenerator(slug),
      closingGenerator: makeApiSceneGenerator(slug),
    };
  }, [slug]);

  useEffect(() => {
    let cancelled = false;
    setDocument(null);
    setError(null);
    getCourseDefinition(slug)
      .then((doc) => {
        if (!cancelled) setDocument(doc);
      })
      .catch((e) => {
        if (cancelled) return;
        const msg = e instanceof ApiError ? e.message : "课程定义加载失败";
        setError(msg);
      });
    return () => {
      cancelled = true;
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
