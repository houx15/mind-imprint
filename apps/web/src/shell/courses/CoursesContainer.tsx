import { useEffect, useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { RuntimeCoursePlayer } from "./RuntimeCoursePlayer";
import { CourseDetail } from "./CourseDetail";
import { getCourseDefinition } from "@/api/courseDefinition";
import { ApiError } from "@/api/client";
import { Button } from "@/ui";
import { api } from "@/api";
import { CourseReport } from "./CourseReport";

type View = { name: "grid" } | { name: "detail"; courseId: string } | { name: "player"; courseId: string } | { name: "report"; courseId: string; attemptId?: string };

// CourseOpenTarget is a deep-link INTO a course from elsewhere (home cards, 图鉴,
// or 学习记录). `mode` picks where it lands: "detail" is the universal browse
// landing (home / 图鉴); "player" resumes the course (学习记录 → 继续); "report"
// opens a finished attempt's frozen report (学习记录 → 查看报告), addressed by
// `attemptId`.
export type CourseOpenTarget = { slug: string; mode: "detail" | "player" | "report"; attemptId?: string };

function initialViewFor(target: CourseOpenTarget | null | undefined): View {
  if (!target) return { name: "grid" };
  switch (target.mode) {
    case "player":
      return { name: "player", courseId: target.slug };
    case "report":
      return { name: "report", courseId: target.slug, attemptId: target.attemptId };
    default:
      return { name: "detail", courseId: target.slug };
  }
}

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

export function CoursesContainer({ onGoPortal, initialOpen, onCourseConsumed, studentId, onImmersiveChange }: { onGoPortal?: () => void; initialOpen?: CourseOpenTarget | null; onCourseConsumed?: () => void; studentId?: string; onImmersiveChange?: (immersive: boolean) => void }) {
  // Deep-link: a course opened from elsewhere lands per initialOpen.mode —
  // home cards / 图鉴 land on DETAIL (the universal landing, CTA → player);
  // 学习记录 lands straight on the PLAYER (继续) or a finished attempt's REPORT
  // (查看报告). Read once at mount.
  const [view, setView] = useState<View>(() => initialViewFor(initialOpen));

  // Consume the one-shot deep-link so re-entering 课程 later shows the grid, not
  // this same course again. `view` already captured the initial target above, so
  // clearing the parent's signal now never closes the just-opened course.
  useEffect(() => {
    if (initialOpen) onCourseConsumed?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Tell the host (CoursesTab) when a course is open (player/report) so it can
  // go immersive — hide the 课程/学习记录/图鉴 segmented + the platform nav rail,
  // mirroring the 项目 tab's in-studio behavior. The grid AND the detail page
  // are NOT immersive — detail is a browse page, not the learning experience
  // itself — so "back from a course" (or landing on its detail) keeps the
  // courses page's chrome.
  useEffect(() => {
    onImmersiveChange?.(view.name === "player" || view.name === "report");
  }, [view.name, onImmersiveChange]);

  // Restart: wipe server-side progress (both storage models), then re-enter the
  // player fresh. Best-effort on the wipe — even if it fails we re-mount the
  // player, which resumes rather than hard-fails.
  const restartAndPlay = async (slug: string) => {
    try {
      await api.restartCourse(slug);
    } catch {
      /* fall through — re-entering the player is still the right next step */
    }
    setView({ name: "player", courseId: slug });
  };

  if (view.name === "detail") {
    return (
      <CourseDetail
        slug={view.courseId}
        onStart={() => setView({ name: "player", courseId: view.courseId })}
        onBack={() => setView({ name: "grid" })}
        onOpenCourse={(id) => setView({ name: "detail", courseId: id })}
      />
    );
  }
  if (view.name === "player") {
    // Guided tour anchors (§P5 Task 7): RuntimeCoursePlayer's root carries
    // `data-testid="course-region"` (stable across loading/opening/playing —
    // unlike `.course-nav__next`, which only exists once the Opening scene's
    // own "开始" has been clicked) and AskPanel's input row carries
    // `data-tour="courses-ask-box"`. The tour targets these directly, so
    // don't remove/rename them.
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
        attemptId={view.attemptId}
        onBackToCourses={() => setView({ name: "grid" })}
        onRestart={() => void restartAndPlay(view.courseId)}
        onGoPortal={() => (onGoPortal ? onGoPortal() : setView({ name: "grid" }))}
      />
    );
  }
  return (
    <CoursesView
      onOpenCourse={(id) => setView({ name: "detail", courseId: id })}
      onRestartCourse={(id) => void restartAndPlay(id)}
    />
  );
}
