import { useEffect, useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { RuntimeCoursePlayer } from "./RuntimeCoursePlayer";
import { getCourseDefinition } from "@/api/courseDefinition";
import { ApiError } from "@/api/client";
import { Button } from "@/ui";
import { CourseReport } from "./CourseReport";

type View = { name: "grid" } | { name: "player"; courseId: string } | { name: "report"; courseId: string };

// PlayerRouter decides PER COURSE which player to mount: a course that HAS a
// 2.0 definition plays through the new runtime (RuntimeCoursePlayer); a course
// whose definition endpoint returns a genuine 404 (no 2.0 definition — legacy
// render_cache content) falls back to the existing linear CoursePlayer.
// P2-10: ONLY a 404 falls back to legacy. A non-404 error (auth 401/403,
// network, 5xx, a malformed 422 definition) is NOT a "this is a legacy course"
// signal — masking it by mounting an unrelated legacy player hides the real
// failure. Those surface a retryable error state instead. The slug is the
// course id used everywhere in the course tab (getCourse is slug-keyed).
function PlayerRouter({ slug, studentId, onExit, onFinish }: { slug: string; studentId?: string; onExit: () => void; onFinish: () => void }) {
  const [kind, setKind] = useState<"loading" | "runtime" | "legacy" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setKind("loading");
    getCourseDefinition(slug)
      .then(() => {
        if (!cancelled) setKind("runtime");
      })
      .catch((e) => {
        if (cancelled) return;
        // A 404 is the authoritative "no 2.0 definition → legacy course" signal.
        // Everything else is a real error, not a routing hint.
        if (e instanceof ApiError && e.status === 404) setKind("legacy");
        else setKind("error");
      });
    return () => {
      cancelled = true;
    };
  }, [slug, attempt]);

  if (kind === "loading") {
    return (
      <div aria-busy="true" style={{ flex: 1, minHeight: 0, display: "flex", alignItems: "center", justifyContent: "center", background: "var(--mk-paper)", color: "var(--mk-faint)", fontSize: 14 }}>
        正在加载课程…
      </div>
    );
  }
  if (kind === "error") {
    return (
      <div role="alert" style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12, background: "var(--mk-paper)", color: "var(--mk-secondary)", fontSize: 14 }}>
        <div>课程加载失败，请重试。</div>
        <div style={{ display: "flex", gap: 8 }}>
          <Button onClick={() => setAttempt((n) => n + 1)}>重试</Button>
          <Button variant="ghost" onClick={onExit}>返回课程</Button>
        </div>
      </div>
    );
  }
  if (kind === "runtime") {
    return <RuntimeCoursePlayer slug={slug} studentId={studentId} onExit={onExit} onFinish={onFinish} />;
  }
  return <CoursePlayer courseId={slug} onExit={onExit} onFinish={onFinish} />;
}

export function CoursesContainer({ onGoPortal, initialCourseId, studentId }: { onGoPortal?: () => void; initialCourseId?: string | null; studentId?: string }) {
  // Deep-link: opening a course from anywhere (home's course cards, the
  // gallery's "去学这张卡的课程" link) lands in the PLAYER so the student can
  // actually learn it (the player resumes at their saved step). Finishing the
  // course lands on the completion view (currently an "即将上线" placeholder).
  // Read once at mount — this component is remounted on every tab switch into 课程.
  const [view, setView] = useState<View>(initialCourseId ? { name: "player", courseId: initialCourseId } : { name: "grid" });

  if (view.name === "player") {
    return (
      <PlayerRouter
        slug={view.courseId}
        studentId={studentId}
        onExit={() => setView({ name: "grid" })}
        onFinish={() => setView({ name: "report", courseId: view.courseId })}
      />
    );
  }
  if (view.name === "report") {
    return (
      <CourseReport
        courseId={view.courseId}
        onBackToCourses={() => setView({ name: "grid" })}
        onGoPortal={() => (onGoPortal ? onGoPortal() : setView({ name: "grid" }))}
      />
    );
  }
  return <CoursesView onOpenCourse={(id) => setView({ name: "player", courseId: id })} />;
}
