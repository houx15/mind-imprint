import { useEffect, useRef, useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { RuntimeCoursePlayer } from "./RuntimeCoursePlayer";
import { CourseDetail } from "./CourseDetail";
import { getCourseDefinition } from "@/api/courseDefinition";
import { ApiError } from "@/api/client";
import { Button } from "@/ui";
import { api } from "@/api";
import { CourseReport } from "./CourseReport";
import { CourseLoading } from "./CourseLoading";

type View = { name: "grid" } | { name: "detail"; courseId: string } | { name: "player"; courseId: string } | { name: "report"; courseId: string; attemptId?: string };

// CourseOpenTarget is a deep-link INTO a course from elsewhere (home cards, 图鉴,
// or 学习记录). `mode` picks where it lands: "detail" is the universal browse
// landing (home / 图鉴); "player" resumes the course (学习记录 → 继续); "report"
// opens a finished attempt's frozen report (学习记录 → 查看报告), addressed by
// `attemptId`.
export type CourseOpenTarget = { slug: string; mode: "detail" | "player" | "report"; attemptId?: string };

// A stable identity for a deep-link target, so the post-mount effect below
// navigates once per genuinely-new target rather than on every object reference.
function openTargetKey(target: CourseOpenTarget): string {
  return `${target.slug}:${target.mode}:${target.attemptId ?? ""}`;
}

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
    return <CourseLoading caption="正在打开课程…" />;
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

export function CoursesContainer({ onGoPortal, initialOpen, onCourseConsumed, studentId, onImmersiveChange, onActiveCourseChange, closeSignal, onCloseSignalConsumed }: { onGoPortal?: () => void; initialOpen?: CourseOpenTarget | null; onCourseConsumed?: () => void; studentId?: string; onImmersiveChange?: (immersive: boolean) => void;
  /** Fired with the OPEN course's slug (detail / player / report), or null on
   * the grid — the shell mirrors it into the URL (`/courses/:slug`) so refresh,
   * copy-link, and Back/Forward land on the same course. Independent of
   * `onImmersiveChange`: detail is a non-immersive browse page that still owns a
   * URL. */
  onActiveCourseChange?: (slug: string | null) => void;
  /** A bumped nonce that asks the container to return to the grid — the shell
   * bumps it when browser Back lands on `/courses` while a course is open. A
   * one-shot: acted on whenever it changes, then `onCloseSignalConsumed`
   * clears it. */
  closeSignal?: number | null;
  onCloseSignalConsumed?: () => void;
 }) {
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

  // React to an `initialOpen` that changes AFTER mount — CoursesContainer stays
  // mounted across grid↔detail↔player navigation, so the mount initializer above
  // can't catch a later deep-link. This happens when the browser Back button
  // lands on `/courses/:slug` (the shell's popstate → pendingCourseId →
  // openTarget), or a tour/home open arrives while the container is already
  // showing. Ref-guarded on the target's identity so a repeat (or the post-open
  // null) never re-navigates. Mirrors WorkspaceContainer's `initialProjectId`.
  const lastOpenKey = useRef<string | null>(initialOpen ? openTargetKey(initialOpen) : null);
  useEffect(() => {
    if (!initialOpen) {
      // The host nulls the signal after each consume, so reset the guard — the
      // SAME target can be re-requested later (Back onto a course just left).
      lastOpenKey.current = null;
      return;
    }
    const key = openTargetKey(initialOpen);
    if (key !== lastOpenKey.current) {
      lastOpenKey.current = key;
      setView(initialViewFor(initialOpen));
      onCourseConsumed?.();
    }
  }, [initialOpen, onCourseConsumed]);

  // Tell the host (CoursesTab) when a course is open (player/report) so it can
  // go immersive — hide the 课程/学习记录/图鉴 segmented + the platform nav rail,
  // mirroring the 项目 tab's in-studio behavior. The grid AND the detail page
  // are NOT immersive — detail is a browse page, not the learning experience
  // itself — so "back from a course" (or landing on its detail) keeps the
  // courses page's chrome.
  useEffect(() => {
    onImmersiveChange?.(view.name === "player" || view.name === "report");
  }, [view.name, onImmersiveChange]);

  // Mirror the open course's slug up to the shell for the URL. Unlike immersive
  // above, this fires for DETAIL too (a browse page that still deserves a
  // `/courses/:slug` address). The grid reports null.
  const activeSlug = view.name === "grid" ? null : view.courseId;
  useEffect(() => {
    onActiveCourseChange?.(activeSlug);
    return () => onActiveCourseChange?.(null);
  }, [activeSlug, onActiveCourseChange]);

  // `closeSignal` deep-link: the shell bumps this nonce to return to the grid
  // (browser Back from `/courses/:slug` to `/courses`). Ref-guarded "only on
  // change" firing.
  const lastCloseSignal = useRef<number | null>(closeSignal ?? null);
  useEffect(() => {
    if (closeSignal != null && closeSignal !== lastCloseSignal.current) {
      lastCloseSignal.current = closeSignal;
      setView({ name: "grid" });
      onCloseSignalConsumed?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [closeSignal]);

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
