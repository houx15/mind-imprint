import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { CourseAsset, CoursePlayerPayload } from "@mind-imprint/contracts";
import { api } from "../../api";
import { SegmentTimeline, buildTimeline, pieceIdFor } from "./SegmentTimeline";
import { AskPanel, type AskMessage } from "./AskPanel";

const MUTED_STORAGE_KEY = "course-audio-muted";

let localAskIdSeq = 0;
function nextAskId(prefix: string) {
  localAskIdSeq += 1;
  return `${prefix}-${Date.now()}-${localAskIdSeq}`;
}

// CoursePlayer is a LINEAR self-paced player — no phase runtime, no session,
// no card offers. It pages through `payload.renderCache.steps` by ordinal; the
// AI bar is a free helper (courseAsk). A course is meant to be COMPLETED, so
// 下一步/完成课程 gate per step: the student must reveal everything and answer
// every quiz on the page before advancing (any answer — never gated on
// correctness). Within a step, revealing is still free (tap anywhere).
export function CoursePlayer({ courseId, onExit, onFinish }: { courseId: string; onExit: () => void; onFinish: () => void }) {
  const [payload, setPayload] = useState<CoursePlayerPayload | null>(null);
  const [ordinal, setOrdinal] = useState(0);
  const [completed, setCompleted] = useState<number[]>([]);

  const [askExpanded, setAskExpanded] = useState(true);
  const [askMessages, setAskMessages] = useState<AskMessage[]>([]);
  const [askPending, setAskPending] = useState(false);

  // Reveal-on-click state lives here (not in SegmentTimeline) so the click
  // target can be the WHOLE reading pane below — clicking anywhere advances,
  // not just a small hint. Reset to 1 whenever the step changes.
  const [revealed, setRevealed] = useState(1);
  useEffect(() => {
    setRevealed(1);
  }, [ordinal]);
  const timelineItems = useMemo(() => {
    if (!payload) return [];
    const step = payload.renderCache.steps[ordinal];
    return step ? buildTimeline(step.content) : [];
  }, [payload, ordinal]);

  // Narration (Task 6): "current piece" is the LATEST-revealed timeline item —
  // `revealed` is a count, so index `revealed - 1`. Only teaching segments are
  // narrated (v1; matches GenerateCourseAudio server-side). `pieceId` is the
  // exact key into `payload.audioKeys` the server populated it under.
  const currentTeachingPiece = useMemo(() => {
    if (!payload) return null;
    const step = payload.renderCache.steps[ordinal];
    if (!step) return null;
    const item = timelineItems[revealed - 1];
    if (!item || item.type !== "segment" || item.segment.kind !== "teaching") return null;
    return { pieceId: pieceIdFor(step.stepId, item.segIdx) };
  }, [payload, ordinal, timelineItems, revealed]);

  // The NEXT teaching piece (not yet revealed) — background-prefetched once
  // the current one is revealed, so the following click has no audio latency.
  const nextTeachingPieceId = useMemo(() => {
    if (!payload) return null;
    const step = payload.renderCache.steps[ordinal];
    if (!step) return null;
    const item = timelineItems[revealed];
    if (!item || item.type !== "segment" || item.segment.kind !== "teaching") return null;
    return pieceIdFor(step.stepId, item.segIdx);
  }, [payload, ordinal, timelineItems, revealed]);

  // --- Per-piece narration playback (Task 6) ---------------------------------
  // One reused <audio> element (never mounted in the DOM — HTMLAudioElement
  // plays fine headless). `resolveTokenRef` guards the resolve→play race: an
  // in-flight `api.resolveUrl` whose token has since been superseded by a
  // later reveal/step-change is discarded rather than played.
  const audioElRef = useRef<HTMLAudioElement | null>(null);
  const resolveTokenRef = useRef(0);
  // Which pieceId is currently loaded into audioElRef (whatever its playing/
  // paused/ended state) — lets the Task 7 fall-through guard distinguish "this
  // piece hasn't started yet" (must play, not advance) from "already started"
  // (fall through to pause/resume/continue as usual). Reset on step change.
  const loadedPieceRef = useRef<string | null>(null);
  // Browsers block audio-with-sound before a user gesture. This starts false
  // and flips true on the first reveal-click or ▶ press; once true, later
  // reveals are allowed to auto-play (the gesture already unlocked audio).
  const hasGestureRef = useRef(false);
  const [muted, setMuted] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(MUTED_STORAGE_KEY) === "1";
    } catch {
      return false;
    }
  });
  const [isPlaying, setIsPlaying] = useState(false);

  // Task 7: the Space-key listener is registered once (window-level, mount
  // only) but must always act on THIS render's latest values — `muted` and
  // `continueCourse` (the latter closes over `hasMore`/`stepDone`/`ordinal`
  // etc., only available in scope after the `if (!payload) return` below).
  // Refs sidestep the stale-closure trap: both are written on every render,
  // and the long-lived listener always reads `.current` at keypress time.
  const continueRef = useRef<() => void>(() => {});
  // Task 7 bugfix: guard used by BOTH the Space fall-through and the
  // content-pane click path — if the current teaching piece has audio that
  // hasn't started yet, START it (and report true so the caller returns
  // instead of advancing). Only when there's genuinely nothing to start
  // (no key / muted / already started) does it return false, letting the
  // caller fall through to continueCourse().
  const tryStartAudioRef = useRef<() => boolean>(() => false);
  const mutedRef = useRef(muted);
  mutedRef.current = muted;

  function ensureAudioEl(): HTMLAudioElement {
    if (!audioElRef.current) {
      const el = new Audio();
      el.addEventListener("play", () => setIsPlaying(true));
      el.addEventListener("pause", () => setIsPlaying(false));
      el.addEventListener("ended", () => setIsPlaying(false));
      audioElRef.current = el;
    }
    return audioElRef.current;
  }

  // Resolve an object key to a signed URL and play it, honoring the race
  // token: if a later piece has superseded this resolve by the time it
  // returns, do nothing (the stale audio must never start playing).
  const playObjectKey = useCallback(async (pieceId: string, key: string) => {
    loadedPieceRef.current = pieceId; // mark "started" synchronously — before the await, so a rapid second gesture can't re-trigger a start for this same piece
    const token = ++resolveTokenRef.current;
    let url: string;
    try {
      url = await api.resolveUrl(key);
    } catch {
      return;
    }
    if (resolveTokenRef.current !== token) return; // superseded — discard
    const audio = ensureAudioEl();
    audio.pause();
    audio.src = url;
    audio.currentTime = 0;
    audio.play().catch(() => {
      /* autoplay rejected (e.g. no gesture yet) — not fatal */
    });
  }, []);

  // Auto-play the current teaching piece whenever it changes, but only after
  // the student's first gesture (mount's initial reveal is not a gesture).
  useEffect(() => {
    if (!payload || !currentTeachingPiece || muted || !hasGestureRef.current) return;
    const key = payload.audioKeys?.[currentTeachingPiece.pieceId];
    if (!key) return;
    void playObjectKey(currentTeachingPiece.pieceId, key);
  }, [payload, currentTeachingPiece, muted, playObjectKey]);

  // Prefetch the next teaching piece's audio in the background (best-effort —
  // a throwaway <audio> just primes the browser's cache, never played here).
  useEffect(() => {
    if (!payload || !nextTeachingPieceId) return;
    const key = payload.audioKeys?.[nextTeachingPieceId];
    if (!key) return;
    let cancelled = false;
    void (async () => {
      try {
        const url = await api.resolveUrl(key);
        if (cancelled) return;
        const preload = new Audio(url);
        preload.load();
      } catch {
        /* best-effort prefetch — ignore failures */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [payload, nextTeachingPieceId]);

  // Stop + release audio and invalidate any in-flight resolve on step change
  // (this cleanup also fires once on unmount, covering that case too).
  useEffect(() => {
    return () => {
      resolveTokenRef.current += 1;
      loadedPieceRef.current = null;
      if (audioElRef.current) {
        audioElRef.current.pause();
        audioElRef.current.removeAttribute("src");
      }
    };
  }, [ordinal]);

  function toggleMuted() {
    setMuted((prev) => {
      const next = !prev;
      try {
        window.localStorage.setItem(MUTED_STORAGE_KEY, next ? "1" : "0");
      } catch {
        /* localStorage unavailable — mute still works for this session */
      }
      if (next) {
        resolveTokenRef.current += 1; // discard any resolve still in flight
        audioElRef.current?.pause();
      }
      return next;
    });
  }

  // ▶/⏸ controls the current piece's audio. A press here is itself the first
  // gesture if the student hasn't clicked/revealed anything yet.
  function togglePlayPause() {
    hasGestureRef.current = true;
    const audio = audioElRef.current;
    if (audio && !audio.paused) {
      audio.pause();
      return;
    }
    if (muted || !payload || !currentTeachingPiece) return;
    if (audio && audio.src && !audio.ended) {
      audio.play().catch(() => {});
      return;
    }
    const key = payload.audioKeys?.[currentTeachingPiece.pieceId];
    if (key) void playObjectKey(currentTeachingPiece.pieceId, key);
  }

  // Space state machine (Task 7): 1) audio playing → pause; 2) audio paused
  // (not ended) → resume; 3) current piece has audio that hasn't started yet
  // → START it (bugfix: at mount `audioElRef.current` is null, so without
  // this branch the very first Space/click fell straight through to
  // continueCourse and the opening block's narration was skipped); 4)
  // otherwise (no audio / ended / muted) → fall through to `continueCourse()`
  // (reveal next piece, or cross into the next step once the page is done).
  // Registered once at mount — reads `continueRef.current`/`tryStartAudioRef.current`
  // rather than closing over per-render state.
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.code !== "Space" && e.key !== " ") return;
      const target = e.target as HTMLElement | null;
      const tag = (target?.tagName || "").toUpperCase();
      if (tag === "INPUT" || tag === "TEXTAREA" || target?.isContentEditable) return; // 问印记 box
      if (tag === "BUTTON") return; // let the browser activate the focused button once; don't also fire our own continue
      e.preventDefault(); // stop page scroll
      hasGestureRef.current = true;
      const audio = mutedRef.current ? null : audioElRef.current; // muted → no audio to control, straight to fallback
      if (audio && !audio.paused && !audio.ended) {
        audio.pause();
        return;
      }
      if (audio && audio.paused && !audio.ended && audio.src) {
        audio.play().catch(() => {});
        return;
      }
      if (tryStartAudioRef.current()) return; // fresh block with audio → start it, don't advance
      continueRef.current();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  // Per-step gate (point 3): a step is "done" only when every timeline item is
  // revealed AND every quiz in it has been answered (any answer — never gated
  // on correctness). Reset the answered set whenever the step changes.
  const [answered, setAnswered] = useState<Set<string>>(new Set());
  useEffect(() => {
    setAnswered(new Set());
  }, [ordinal]);

  // Active-focus time (point 2b): accrue seconds only while the course page is
  // visible AND focused; takeActiveDelta hands back the whole seconds not yet
  // flushed to the server (which accumulates them additively across visits).
  const activeAccrued = useRef(0);
  const activeFlushed = useRef(0);
  useEffect(() => {
    let last = Date.now();
    const tick = () => {
      const now = Date.now();
      if (document.visibilityState === "visible" && document.hasFocus()) {
        activeAccrued.current += (now - last) / 1000;
      }
      last = now;
    };
    const iv = window.setInterval(tick, 1000);
    return () => window.clearInterval(iv);
  }, []);
  const takeActiveDelta = useCallback(() => {
    const delta = Math.floor(activeAccrued.current - activeFlushed.current);
    if (delta > 0) activeFlushed.current += delta;
    return delta > 0 ? delta : 0;
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const p = await api.getCourse(courseId);
      let startOrd = 0;
      let comp: number[] = [];
      try {
        const prog = await api.getCourseProgress(courseId);
        startOrd = Math.min(prog.current_ordinal, p.renderCache.steps.length - 1);
        comp = prog.completed_ordinals;
      } catch {
        /* no progress yet → default to step 0 */
      }
      if (cancelled) return;
      setCompleted(comp);
      setOrdinal(Math.max(0, startOrd));
      setPayload(p);
    })();
    return () => {
      cancelled = true;
    };
  }, [courseId]);

  // Later entries (a step's own `materials`) override the shared
  // asset_library on id collision — a step can point at a fresh/edited copy
  // of an asset without touching the library. Memoized on payload so this
  // rebuild only happens when a new course loads, not on every ordinal/ask
  // re-render.
  const assetsById: Record<string, CourseAsset> = useMemo(() => {
    const map: Record<string, CourseAsset> = {};
    if (!payload) return map;
    for (const a of payload.structure.asset_library) map[a.id] = a;
    for (const step of payload.structure.steps) {
      for (const a of step.materials) map[a.id] = a;
    }
    return map;
  }, [payload]);

  // Every authored interaction in the current step ends up in the timeline
  // (matched inline or appended), so the step's own interaction ids are exactly
  // what must be answered to clear the gate.
  const requiredInteractionIds = useMemo(() => {
    if (!payload) return [] as string[];
    const step = payload.renderCache.steps[ordinal];
    return step ? (step.content.interactions || []).map((i) => i.id) : [];
  }, [payload, ordinal]);
  const allRevealed = revealed >= timelineItems.length;
  const allAnswered = requiredInteractionIds.every((id) => answered.has(id));
  const stepDone = allRevealed && allAnswered;

  // When the current step becomes done, mark it complete on the server (and
  // locally) and flush the active-time delta. Idempotent — the server union
  // no-ops if the ordinal is already recorded. Fires once per step (deps reset
  // when the ordinal changes → stepDone drops back to false, then true again).
  useEffect(() => {
    if (!payload || !stepDone) return;
    setCompleted((prev) => (prev.includes(ordinal) ? prev : [...prev, ordinal].sort((a, b) => a - b)));
    void api.saveCourseProgress(courseId, {
      current_ordinal: ordinal,
      completed_ordinal: ordinal,
      active_seconds_delta: takeActiveDelta(),
    }).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stepDone, ordinal, payload, courseId]);

  // Periodic + on-hide + on-unmount flush of accrued active time, so a long
  // dwell isn't lost if the student never navigates.
  const ordinalRef = useRef(ordinal);
  useEffect(() => {
    ordinalRef.current = ordinal;
  }, [ordinal]);
  useEffect(() => {
    const flush = () => {
      const d = takeActiveDelta();
      if (d > 0) void api.saveCourseProgress(courseId, { current_ordinal: ordinalRef.current, active_seconds_delta: d }).catch(() => {});
    };
    const iv = window.setInterval(flush, 20000);
    const onVis = () => { if (document.visibilityState === "hidden") flush(); };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(iv);
      document.removeEventListener("visibilitychange", onVis);
      flush();
    };
  }, [courseId, takeActiveDelta]);

  if (!payload) return <div style={{ padding: 40, color: "var(--mk-muted)" }}>正在载入课程…</div>;

  const steps = payload.renderCache.steps;
  const total = steps.length;
  const currentStep = steps[ordinal]!;
  const isLast = ordinal === total - 1;
  const hasMore = revealed < timelineItems.length;

  // Task 7 bugfix guard: if the current teaching piece has audio and it
  // hasn't started yet (never loaded into audioElRef), start it and report
  // true so the caller returns instead of advancing. Shared by the Space
  // fall-through and the content-pane click path — both must PLAY a fresh
  // block's narration on the first gesture rather than reveal/cross past it.
  // Already-started pieces (playing / paused-mid / ended) fall through
  // (false) to the normal pause/resume/continue behavior.
  function tryStartCurrentPieceAudio(): boolean {
    if (mutedRef.current || !currentTeachingPiece) return false;
    const pieceId = currentTeachingPiece.pieceId;
    if (loadedPieceRef.current === pieceId) return false; // already started this piece
    const key = payload!.audioKeys?.[pieceId]; // payload is non-null here (component returned above otherwise)
    if (!key) return false;
    hasGestureRef.current = true;
    void playObjectKey(pieceId, key);
    return true;
  }
  tryStartAudioRef.current = tryStartCurrentPieceAudio;

  // Handle a click anywhere in the reading pane, except on actual interactive
  // controls (quiz buttons, links, inputs) — a single tap anywhere continues,
  // mirroring the reference's revealNextSegment.
  function handleRevealClick(event: React.MouseEvent<HTMLDivElement>) {
    if ((event.target as HTMLElement).closest("button, a, input, textarea, select")) return;
    hasGestureRef.current = true; // this click is itself a user gesture — unlocks auto-play
    if (tryStartCurrentPieceAudio()) return; // fresh block with audio → start it, don't advance
    continueCourse();
  }

  function appendAskMessage(msg: AskMessage) {
    setAskMessages((prev) => [...prev, msg]);
  }

  // Record an answered quiz (clears part of the per-step gate) and log it.
  function handleQuizAnswer(event: { stepId: string; interactionId: string; selected: string[]; correct: boolean }) {
    setAnswered((prev) => {
      if (prev.has(event.interactionId)) return prev;
      const n = new Set(prev);
      n.add(event.interactionId);
      return n;
    });
    void api.answerCourseQuiz(courseId, event).catch(() => {});
  }

  // Navigation persists the resume position and flushes active time; it does
  // NOT mark completion (the step-done effect owns that, so navigating never
  // over-marks a step you merely passed through).
  function go(next: number) {
    void api.saveCourseProgress(courseId, { current_ordinal: next, active_seconds_delta: takeActiveDelta() }).catch(() => {});
    setOrdinal(next);
  }

  function handleNext() {
    if (!stepDone) return; // gated: finish the page's content + questions first
    if (isLast) {
      // Ensure the last step is recorded complete + flush the remaining active
      // time before leaving to the report (idempotent with the step-done save).
      void api.saveCourseProgress(courseId, { current_ordinal: ordinal, completed_ordinal: ordinal, active_seconds_delta: takeActiveDelta() }).catch(() => {});
      onFinish();
      return;
    }
    go(ordinal + 1);
  }

  // Single continue() action shared by Space's fall-through and content-pane
  // clicks (Task 7): reveal the next piece while the step still has more, else
  // cross into the next step (or finish) once it's done, else no-op (blocked
  // by an unanswered quiz — the "先回答本页的问题" hint already covers that).
  function continueCourse() {
    if (hasMore) {
      setRevealed((v) => Math.min(v + 1, timelineItems.length));
      return;
    }
    if (stepDone) {
      handleNext();
    }
  }
  continueRef.current = continueCourse;

  async function handleAsk(text: string) {
    appendAskMessage({ id: nextAskId("student"), role: "student", text });
    setAskPending(true);
    const assistantId = nextAskId("assistant");
    let started = false;
    try {
      for await (const event of api.courseAsk(courseId, text, ordinal)) {
        if (event.type === "reply") {
          if (!started) {
            started = true;
            appendAskMessage({ id: assistantId, role: "assistant", text: event.body });
          } else {
            setAskMessages((prev) => prev.map((m) => (m.id === assistantId ? { ...m, text: event.body } : m)));
          }
        } else if (event.type === "error") {
          if (!started) {
            started = true;
            appendAskMessage({ id: assistantId, role: "assistant", text: event.message || "出错了，请重试" });
          } else {
            setAskMessages((prev) => prev.map((m) => (m.id === assistantId ? { ...m, text: event.message || "出错了，请重试" } : m)));
          }
        }
      }
    } finally {
      setAskPending(false);
    }
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%" }}>
      {/* header */}
      <div style={{ flex: "none", background: "var(--mk-surface)", borderBottom: "1px solid var(--mk-border)" }}>
        <div style={{ height: 50, display: "flex", alignItems: "center", padding: "0 20px", gap: 13 }}>
          <div onClick={onExit} style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--mk-secondary)", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: "6px 10px", borderRadius: 8 }}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
            课程
          </div>
          <div style={{ width: 1, height: 20, background: "var(--mk-border)" }} />
          <span style={{ flex: "none", width: 8, height: 8, borderRadius: 3, background: "var(--mk-accent-500)" }} />
          <span style={{ fontSize: 14, fontWeight: 700, color: "var(--mk-ink)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{payload.title}</span>
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 9 }}>
            <button
              type="button"
              aria-label={isPlaying ? "暂停" : "播放"}
              onClick={togglePlayPause}
              disabled={!currentTeachingPiece || !payload.audioKeys?.[currentTeachingPiece.pieceId]}
              style={{
                display: "inline-flex", alignItems: "center", justifyContent: "center", width: 28, height: 28,
                border: "1px solid var(--mk-input-border)", background: "var(--mk-surface)", borderRadius: 8, cursor: "pointer",
                opacity: !currentTeachingPiece || !payload.audioKeys?.[currentTeachingPiece.pieceId] ? 0.4 : 1,
              }}
            >
              {isPlaying ? (
                <svg width="14" height="14" viewBox="0 0 24 24" fill="var(--mk-accent-500)" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1" /><rect x="14" y="5" width="4" height="14" rx="1" /></svg>
              ) : (
                <svg width="14" height="14" viewBox="0 0 24 24" fill="var(--mk-accent-500)" aria-hidden="true"><path d="M8 5.14v13.72a1 1 0 0 0 1.5.86l11.5-6.86a1 1 0 0 0 0-1.72L9.5 4.28A1 1 0 0 0 8 5.14z" /></svg>
              )}
            </button>
            <button
              type="button"
              aria-label={muted ? "取消静音" : "静音"}
              onClick={toggleMuted}
              style={{
                display: "inline-flex", alignItems: "center", justifyContent: "center", width: 28, height: 28,
                border: "1px solid var(--mk-input-border)", background: "var(--mk-surface)", borderRadius: 8, cursor: "pointer",
              }}
            >
              {muted ? (
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--mk-muted)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M11 5 6 9H3v6h3l5 4V5z" /><line x1="16" y1="9" x2="22" y2="15" /><line x1="22" y1="9" x2="16" y2="15" /></svg>
              ) : (
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--mk-accent-500)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M11 5 6 9H3v6h3l5 4V5z" /><path d="M15.5 8.5a5 5 0 0 1 0 7" /><path d="M18.5 5.5a9 9 0 0 1 0 13" /></svg>
              )}
            </button>
            <span style={{ fontSize: 11.5, fontWeight: 600, color: "var(--mk-faint)", background: "var(--mk-paper)", padding: "2px 9px", borderRadius: 999 }}>{ordinal + 1} / {total}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4, padding: "0 20px 11px" }}>
          {steps.map((s, i) => (
            <div key={s.stepId} style={{ flex: 1, height: 4, borderRadius: 3, background: i < ordinal || completed.includes(i) ? "var(--mk-success)" : i === ordinal ? "var(--mk-accent-500)" : "var(--mk-border)" }} />
          ))}
        </div>
      </div>

      {/* body: content + ask panel — dc.html 236-397 */}
      <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
        <div style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", background: "var(--mk-paper)" }}>
          {/* scroll area — the whole pane is the reveal click target so a tap
              anywhere continues (铁律 2: never gated) */}
          <div onClick={handleRevealClick} style={{ flex: 1, minHeight: 0, overflowY: "auto", cursor: hasMore || stepDone ? "pointer" : "default" }}>
            <div style={{ padding: "36px 40px 40px", minHeight: "100%", boxSizing: "border-box" }}>
              <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
                <SegmentTimeline
                  key={ordinal}
                  stepId={currentStep.stepId}
                  content={currentStep.content}
                  assetsById={assetsById}
                  revealedCount={revealed}
                  onQuizAnswer={handleQuizAnswer}
                />
              </div>
            </div>
          </div>

          {/* nav — pinned to the page bottom. 下一步/完成课程 gate per step:
              disabled until the page is fully revealed and its quizzes answered
              (point 3). Within a step, revealing is still free (tap anywhere). */}
          <div style={{ flex: "none", borderTop: "1px solid var(--mk-border)", background: "var(--mk-surface)", padding: "11px 40px" }}>
            <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
              {!stepDone && (
                <div style={{ fontSize: 12, fontWeight: 600, color: "var(--mk-warning)", background: "var(--mk-warning-bg)", border: "1px solid var(--mk-warning-bg)", borderRadius: 8, padding: "6px 11px", marginBottom: 9, textAlign: "center" }}>
                  {!allRevealed ? "先看完本页内容，再继续" : "先回答本页的问题，再继续"}
                </div>
              )}
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                {ordinal > 0 ? (
                  <button
                    type="button"
                    aria-label="上一步"
                    onClick={() => go(ordinal - 1)}
                    style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "var(--mk-surface)", border: "1px solid var(--mk-input-border)", color: "var(--mk-secondary)", borderRadius: 10, padding: "9px 15px", fontSize: 13.5, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}
                  >
                    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
                    上一步
                  </button>
                ) : (
                  <span />
                )}
                <button
                  type="button"
                  aria-label={isLast ? "完成课程" : "下一步"}
                  onClick={handleNext}
                  disabled={!stepDone}
                  style={{ display: "inline-flex", alignItems: "center", gap: 7, background: !stepDone ? "var(--mk-faint)" : isLast ? "var(--mk-success)" : "var(--mk-accent-500)", border: "none", color: "var(--mk-surface)", borderRadius: 10, padding: "10px 20px", fontSize: 14, fontWeight: 700, cursor: stepDone ? "pointer" : "not-allowed", fontFamily: "inherit", boxShadow: !stepDone ? "none" : isLast ? "0 4px 14px rgba(95,169,126,.26)" : "0 4px 14px rgba(234,81,64,.24)" }}
                >
                  {isLast ? (
                    <>
                      完成课程
                      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="var(--mk-surface)" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
                    </>
                  ) : (
                    <>
                      下一步
                      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="var(--mk-surface)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6" /></svg>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>

        <AskPanel
          expanded={askExpanded}
          onToggle={() => setAskExpanded((e) => !e)}
          branchColor="#EA5140"
          context={payload.title}
          chips={[]}
          messages={askMessages}
          pending={askPending}
          onSend={(text) => void handleAsk(text)}
        />
      </div>
    </div>
  );
}
