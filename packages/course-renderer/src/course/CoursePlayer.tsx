import { useEffect, useMemo, useRef, useState } from "react";
import {
  validateCourseDefinition,
  type CourseDefinition,
  type RuntimeSceneResult,
  type SliceDefinition,
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

  // Init once: create/restore the session, build the bus, generate the opening.
  const initRan = useRef(false);
  useEffect(() => {
    if (!course || initRan.current) return;
    initRan.current = true;
    let cancelled = false;

    void (async () => {
      const session = sessionId
        ? (await adapters.sessionAdapter.load(sessionId)) ??
          (await adapters.sessionAdapter.create({ courseId: course.id, studentId }))
        : await adapters.sessionAdapter.create({ courseId: course.id, studentId });
      if (cancelled) return;

      activeSessionId.current = session.id;
      const newBus = new RuntimeEventBus({ courseId: course.id, sessionId: session.id, idFactory, clock });
      setBus(newBus);
      onBusReady?.(newBus);

      await adapters.sessionAdapter.setStatus(session.id, "opening");

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
      />
    </div>
  );
}
