import { useEffect, useMemo, useRef, useState } from "react";
import {
  validateCourseDefinition,
  CourseSession,
  type CourseDefinition,
  type RuntimeSceneResult,
  type SliceDefinition,
  type SliceSessionState,
  type ValidationIssue,
} from "@mind-imprint/course-contract";
import {
  RuntimeEventBus,
  type Clock,
  type ClosingSceneInput,
  type CourseRuntimeAdapters,
  type IdFactory,
  type OpeningSceneInput,
} from "@mind-imprint/course-runtime";
import { SlicePlayer } from "../slice/SlicePlayer";
import { OpeningScene } from "../scenes/OpeningScene";
import { ClosingScene } from "../scenes/ClosingScene";

export interface CoursePlayerProps {
  document: unknown;
  adapters: CourseRuntimeAdapters;
  studentId: string;
  /** Restore an existing session instead of creating one. */
  sessionId?: string;
  /** Deterministic factories for the event bus (§16 replayability). */
  idFactory: IdFactory;
  clock: Clock;
  /** Fires once the event bus is built. Test/host seam for observing the bus. */
  onBusReady?: (bus: RuntimeEventBus) => void;
}

type Phase = "loading" | "error" | "opening" | "playing" | "closing";

interface SliceEntry {
  partId: string;
  slice: SliceDefinition;
}

function flattenSlices(course: CourseDefinition): SliceEntry[] {
  const entries: SliceEntry[] = [];
  for (const part of course.parts) {
    for (const slice of part.slices) entries.push({ partId: part.id, slice });
  }
  return entries;
}

function ErrorSurface({ issues }: { issues: ValidationIssue[] }) {
  return (
    <section className="course-error" role="alert" aria-label="课程无法播放" data-course-error="true">
      <h2>课程定义无法播放</h2>
      <ul className="course-error__issues">
        {issues.map((issue, i) => (
          <li key={i} data-issue-layer={issue.layer}>
            <code>{issue.path || "(root)"}</code>: {issue.message}
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * §17.3 / §6 / §13 — the course-level lifecycle: validate → Opening → the linear
 * Part/Slice walk → Closing. Opening and Closing text come through the injected
 * scene generators (fallback path is exercised in this slice). Structurally
 * invalid documents render a diagnostic surface and never mount a SlicePlayer.
 */
export function CoursePlayer({ document, adapters, studentId, sessionId, idFactory, clock, onBusReady }: CoursePlayerProps) {
  const validation = useMemo(() => validateCourseDefinition(document), [document]);

  const [phase, setPhase] = useState<Phase>(validation.ok ? "loading" : "error");
  const [opening, setOpening] = useState<RuntimeSceneResult | null>(null);
  const [closing, setClosing] = useState<RuntimeSceneResult | null>(null);
  const [currentIndex, setCurrentIndex] = useState(0);
  const indexRef = useRef(0);
  const [bus, setBus] = useState<RuntimeEventBus | null>(null);
  const activeSessionId = useRef<string | null>(null);

  const course = validation.ok ? validation.course : null;
  const entries = useMemo(() => (course ? flattenSlices(course) : []), [course]);

  // A one-shot resume instruction for the FIRST SlicePlayer mount after init,
  // set only when a validated existing session had `status:"in-progress"` and
  // a `current` Slice we can still locate in this course. Consumed by index
  // match in render — subsequent forward navigation always starts a slice fresh.
  const resumeRef = useRef<{ index: number; stepId?: string; state?: SliceSessionState } | null>(null);

  // Init once: restore (validated) or create the session, build the bus,
  // resolve Opening/Closing without a redundant generator call when the
  // session already carries them, and land on the right phase/position.
  const initRan = useRef(false);
  useEffect(() => {
    if (!course || initRan.current) return;
    initRan.current = true;
    let cancelled = false;

    const placeholderScene = (): RuntimeSceneResult => ({
      text: "",
      generatedAt: clock(),
      usedSignalTypes: [],
      fallbackUsed: true,
    });

    void (async () => {
      const loaded = sessionId ? await adapters.sessionAdapter.load(sessionId) : null;
      // Validate at the boundary (§16): a malformed/incompatible stored session
      // must degrade to a fresh start, never crash the player.
      const parsed = loaded ? CourseSession.safeParse(loaded) : null;
      const restored = parsed?.success ? parsed.data : null;

      const session = restored ?? (await adapters.sessionAdapter.create({ courseId: course.id, studentId }));
      if (cancelled) return;

      activeSessionId.current = session.id;
      const newBus = new RuntimeEventBus({ courseId: course.id, sessionId: session.id, idFactory, clock });
      setBus(newBus);
      onBusReady?.(newBus);

      // Resume mid-course: a validated session that was actively playing and
      // whose current Slice still exists in this course definition — skip
      // Opening entirely and land straight back on that Slice.
      const resumeIndex =
        restored && restored.status === "in-progress" && restored.current
          ? entries.findIndex((e) => e.partId === restored.current!.partId && e.slice.id === restored.current!.sliceId)
          : -1;
      if (resumeIndex >= 0) {
        const current = restored!.current!;
        const sliceState = restored!.sliceStates[current.sliceId];
        resumeRef.current = {
          index: resumeIndex,
          stepId: sliceState?.currentWorkflowStepId ?? current.workflowStepId,
          state: sliceState,
        };
        // The loading guard below requires a non-null `opening`, even though
        // the Opening scene itself is never shown on this path.
        setOpening(restored!.opening ?? placeholderScene());
        indexRef.current = resumeIndex;
        setCurrentIndex(resumeIndex);
        setPhase("playing");
        return;
      }

      // Resume into a completed session's Closing, restored (not regenerated).
      if (restored && restored.status === "completed" && restored.closing) {
        setOpening(restored.opening ?? placeholderScene());
        setClosing(restored.closing);
        setPhase("closing");
        return;
      }

      await adapters.sessionAdapter.setStatus(session.id, "opening");

      // A session that already has a saved Opening (e.g. reloaded before
      // clicking start) restores it instead of paying for regeneration.
      if (restored?.opening) {
        setOpening(restored.opening);
        setPhase("opening");
        return;
      }

      const input: OpeningSceneInput = {
        which: "opening",
        title: course.title,
        estimatedMinutes: course.estimatedMinutes,
        objectives: course.objectives.map((o) => o.text),
        learningPreview: course.opening.learningPreview,
        allowedSignals: course.opening.personalization.enabled ? course.opening.personalization.allowedSignals : [],
        signalValues: {},
        fallback: {
          text: course.opening.fallback.text,
          audioUrl: course.opening.fallback.audio ? adapters.assetResolver.resolve(course.opening.fallback.audio) : undefined,
        },
      };
      const result = await adapters.openingGenerator.generate(input);
      if (cancelled) return;
      await adapters.sessionAdapter.saveScene(session.id, "opening", result);
      setOpening(result);
      setPhase("opening");
    })();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // §16 resume — record which Slice is active whenever it changes, so a later
  // reload's `current` lookup (above) has somewhere to land.
  useEffect(() => {
    if (phase !== "playing") return;
    const sid = activeSessionId.current;
    const entry = entries[currentIndex];
    if (!sid || !entry) return;
    void adapters.sessionAdapter.setCurrent(sid, {
      partId: entry.partId,
      sliceId: entry.slice.id,
      workflowStepId: entry.slice.workflow.initialStepId,
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, currentIndex]);

  const handleStart = () => {
    const sid = activeSessionId.current;
    if (sid) void adapters.sessionAdapter.setStatus(sid, "in-progress");
    indexRef.current = 0;
    setCurrentIndex(0);
    setPhase("playing");
  };

  const runClosing = async () => {
    if (!course) return;
    const sid = activeSessionId.current;
    const input: ClosingSceneInput = {
      which: "closing",
      preparedSummary: course.closing.preparedSummary,
      takeaways: course.closing.takeaways,
      transferApplications: course.closing.transferApplications,
      allowedSignals: course.closing.personalization.enabled ? course.closing.personalization.allowedSignals : [],
      sessionEvidence: {},
      fallback: {
        text: course.closing.fallback.text,
        audioUrl: course.closing.fallback.audio ? adapters.assetResolver.resolve(course.closing.fallback.audio) : undefined,
      },
    };
    const result = await adapters.closingGenerator.generate(input);
    if (sid) {
      await adapters.sessionAdapter.saveScene(sid, "closing", result);
      await adapters.sessionAdapter.setStatus(sid, "completed");
    }
    setClosing(result);
    setPhase("closing");
  };

  const handleNavigateNext = () => {
    const next = indexRef.current + 1;
    if (next >= entries.length) {
      void runClosing();
      return;
    }
    indexRef.current = next;
    setCurrentIndex(next);
  };

  if (phase === "error" || !course) {
    return <ErrorSurface issues={validation.ok ? [] : validation.issues} />;
  }

  if (phase === "loading" || !opening || !bus) {
    return <div className="course-loading" aria-busy="true" />;
  }

  if (phase === "opening") {
    return (
      <OpeningScene
        scene={opening}
        title={course.title}
        estimatedMinutes={course.estimatedMinutes}
        objectives={course.objectives.map((o) => o.text)}
        learningPreview={course.opening.learningPreview}
        onStart={handleStart}
      />
    );
  }

  if (phase === "closing" && closing) {
    return (
      <ClosingScene
        scene={closing}
        summary={course.closing.preparedSummary}
        takeaways={course.closing.takeaways}
        transferApplications={course.closing.transferApplications}
      />
    );
  }

  const entry = entries[currentIndex]!;
  // The resume instruction only applies to the exact Slice it was computed
  // for — once navigation moves past it, later Slices always start fresh.
  const resume = resumeRef.current?.index === currentIndex ? resumeRef.current : null;
  return (
    <div className="course-player" data-phase="playing">
      <SlicePlayer
        key={entry.slice.id}
        slice={entry.slice}
        partId={entry.partId}
        sessionId={activeSessionId.current!}
        adapters={adapters}
        bus={bus}
        onSliceComplete={() => {}}
        onNavigateNext={handleNavigateNext}
        restoreStepId={resume?.stepId}
        restoreState={resume?.state}
      />
    </div>
  );
}
