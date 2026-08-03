import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { CourseAsset, CoursePlayerPayload } from "@mind-imprint/contracts";
import { api } from "../../api";
import { SegmentTimeline, buildTimeline } from "./SegmentTimeline";
import { AskPanel, type AskMessage } from "./AskPanel";

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

  if (!payload) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在载入课程…</div>;

  const steps = payload.renderCache.steps;
  const total = steps.length;
  const currentStep = steps[ordinal]!;
  const isLast = ordinal === total - 1;
  const hasMore = revealed < timelineItems.length;

  // Advance the reveal when the student clicks anywhere in the reading pane,
  // except on actual interactive controls (quiz buttons, links, inputs) — so a
  // single tap anywhere continues, mirroring the reference's revealNextSegment.
  function handleRevealClick(event: React.MouseEvent<HTMLDivElement>) {
    if (!hasMore) return;
    if ((event.target as HTMLElement).closest("button, a, input, textarea, select")) return;
    setRevealed((v) => Math.min(v + 1, timelineItems.length));
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
      <div style={{ flex: "none", background: "#fff", borderBottom: "1px solid #EFF0F5" }}>
        <div style={{ height: 50, display: "flex", alignItems: "center", padding: "0 20px", gap: 13 }}>
          <div onClick={onExit} style={{ display: "flex", alignItems: "center", gap: 6, color: "#6B7384", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: "6px 10px", borderRadius: 8 }}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
            课程
          </div>
          <div style={{ width: 1, height: 20, background: "#EAECF2" }} />
          <span style={{ flex: "none", width: 8, height: 8, borderRadius: 3, background: "#2A3B7A" }} />
          <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{payload.title}</span>
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 9 }}>
            <span style={{ fontSize: 11.5, fontWeight: 600, color: "#AEB4C2", background: "#F2F3F8", padding: "2px 9px", borderRadius: 999 }}>{ordinal + 1} / {total}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4, padding: "0 20px 11px" }}>
          {steps.map((s, i) => (
            <div key={s.stepId} style={{ flex: 1, height: 4, borderRadius: 3, background: i < ordinal || completed.includes(i) ? "#4C9A82" : i === ordinal ? "#2A3B7A" : "#E7E9F0" }} />
          ))}
        </div>
      </div>

      {/* body: content + ask panel — dc.html 236-397 */}
      <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
        <div style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", background: "#F3F4F8" }}>
          {/* scroll area — the whole pane is the reveal click target so a tap
              anywhere continues (铁律 2: never gated) */}
          <div onClick={handleRevealClick} style={{ flex: 1, minHeight: 0, overflowY: "auto", cursor: hasMore ? "pointer" : "default" }}>
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
          <div style={{ flex: "none", borderTop: "1px solid #EAECF2", background: "#fff", padding: "11px 40px" }}>
            <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
              {!stepDone && (
                <div style={{ fontSize: 12, fontWeight: 600, color: "#B08150", background: "#FBF3E9", border: "1px solid #F0E0C8", borderRadius: 8, padding: "6px 11px", marginBottom: 9, textAlign: "center" }}>
                  {!allRevealed ? "先看完本页内容，再继续" : "先回答本页的问题，再继续"}
                </div>
              )}
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                {ordinal > 0 ? (
                  <button
                    type="button"
                    aria-label="上一步"
                    onClick={() => go(ordinal - 1)}
                    style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "#fff", border: "1px solid #E1E4ED", color: "#6B7384", borderRadius: 10, padding: "9px 15px", fontSize: 13.5, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}
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
                  style={{ display: "inline-flex", alignItems: "center", gap: 7, background: !stepDone ? "#C7CCDA" : isLast ? "#4C9A82" : "#2A3B7A", border: "none", color: "#fff", borderRadius: 10, padding: "10px 20px", fontSize: 14, fontWeight: 700, cursor: stepDone ? "pointer" : "not-allowed", fontFamily: "inherit", boxShadow: !stepDone ? "none" : isLast ? "0 4px 14px rgba(76,154,130,.26)" : "0 4px 14px rgba(42,59,122,.24)" }}
                >
                  {isLast ? (
                    <>
                      完成课程
                      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
                    </>
                  ) : (
                    <>
                      下一步
                      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6" /></svg>
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
          branchColor="#2A3B7A"
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
