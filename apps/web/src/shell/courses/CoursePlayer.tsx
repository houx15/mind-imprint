import { useEffect, useRef, useState } from "react";
import type { Course, CourseSession, RenderedStep } from "@mind-imprint/contracts";
import { INFO_LITERACY_COURSE_SKILL } from "@mind-imprint/contracts";
import { api } from "../../api";
import { TeachingTemplate, type TeachingContent } from "./TeachingTemplate";
import { ChallengeTemplate, type ChallengeContent } from "./ChallengeTemplate";
import { AskPanel, type AskMessage } from "./AskPanel";

let localAskIdSeq = 0;
function nextAskId(prefix: string) {
  localAskIdSeq += 1;
  return `${prefix}-${Date.now()}-${localAskIdSeq}`;
}

// The phase's step ordinals, per the single-source-of-truth skill JSON
// (packages/contracts/skills/info-literacy-course.json). Step-less phases
// (guided, reflect) render their authored `page` block instead of a
// course_step render — pages are content, phases are runtime.
function phaseSteps(phase: string): number[] {
  return INFO_LITERACY_COURSE_SKILL.contracts[phase]?.steps ?? [];
}

export function CoursePlayer({ courseId, onExit, onFinish }: { courseId: string; onExit: () => void; onFinish: () => void }) {
  const [course, setCourse] = useState<Course | null>(null);
  const [ordinal, setOrdinal] = useState(0);
  const [rendered, setRendered] = useState<RenderedStep | null>(null);
  const [completed, setCompleted] = useState<number[]>([]);

  const [session, setSession] = useState<CourseSession | null>(null);
  const [askExpanded, setAskExpanded] = useState(true);
  const [askMessages, setAskMessages] = useState<AskMessage[]>([]);
  const [askPending, setAskPending] = useState(false);
  const askMessagesRef = useRef<AskMessage[]>([]);
  askMessagesRef.current = askMessages;

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const c = await api.getCourse(courseId);
      let startOrd = 0;
      let comp: number[] = [];
      try {
        const p = await api.getCourseProgress(courseId);
        startOrd = Math.min(p.current_ordinal, c.steps.length - 1);
        comp = p.completed_ordinals;
      } catch {
        /* no progress yet → default to step 0 */
      }
      if (cancelled) return;
      // Set course LAST so the render effect first fires with the resumed ordinal
      // already applied — avoids a stale step-0 flash + a wasted render on resume.
      setCompleted(comp);
      setOrdinal(startOrd);
      setCourse(c);
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const s = await api.startCourseSession(courseId);
        if (cancelled) return;
        setSession(s);
        // Restore any card offer the session still has open (status
        // proposed/active) as a synthetic assistant message carrying the
        // offer — this is what lets a page reload during `guided` recover
        // an offer that otherwise lived only in React state (Critical-2):
        // without it, the card_dispositioned floor could never be met again
        // after a reload, dead-ending the course forever.
        const restored: AskMessage[] = s.messages.map((m) => ({ id: m.id, role: m.role, text: m.content }));
        for (const oc of s.openCards ?? []) {
          restored.push({
            id: `offer-${oc.cardInstanceId}`,
            role: "assistant",
            text: "",
            offer: { cardInstanceId: oc.cardInstanceId, cardId: oc.cardId, materialId: oc.materialId },
            offerPhase: "offered",
          });
        }
        setAskMessages(restored);
      } catch {
        /* the session runtime is additive on top of the page layer below —
           its unavailability must not block page rendering. */
      }
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  useEffect(() => {
    if (!course) return;
    // The session runtime is additive on top of the page layer: when it is
    // unavailable (session null — the start call failed), pages still render
    // like the pre-Slice-12 player did — content survives the runtime being
    // down. Only a step-less phase (session PRESENT and phase-less) skips the
    // course_step render in favor of the authored `page` block.
    if (session && phaseSteps(session.phase).length === 0) return;
    let cancelled = false;
    setRendered(null);
    void api.renderCourseStep(courseId, ordinal).then((r) => { if (!cancelled) setRendered(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [course, session, courseId, ordinal]);

  if (!course) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在载入课程…</div>;

  function go(next: number) {
    const nextCompleted = Array.from(new Set([...completed, ordinal])).sort((a, b) => a - b);
    setCompleted(nextCompleted);
    void api.saveCourseProgress(courseId, { current_ordinal: next, completed_ordinals: nextCompleted }).catch(() => {});
    setOrdinal(next);
  }

  function appendAskMessage(msg: AskMessage) {
    setAskMessages((prev) => [...prev, msg]);
  }

  // Consumes one courseAsk/courseAdvance SSE stream, threading a running
  // assistant message + any card offer + a phase move — mirrors ChatContainer's
  // handleSend, plus the new `phase` frame (Slice 12). `done` is NOT handled
  // here: the server emits it unconditionally at the end of EVERY turn
  // (including the error path), so it is a stream terminator, not a
  // completion signal — exactly like ChatContainer, which handles only
  // reply/card/error and lets the `for await` loop's natural end be the
  // "done" semantic. The course's actual completion is learned from the
  // session's own `status` field (see handleNext), never from this frame.
  async function consumeCourseTurn(stream: AsyncGenerator<import("../../api").CourseTurnEvent>): Promise<{ phaseMoved: boolean }> {
    const assistantId = nextAskId("assistant");
    let phaseMoved = false;
    let started = false;
    for await (const event of stream) {
      if (event.type === "reply") {
        if (!started) {
          started = true;
          appendAskMessage({ id: assistantId, role: "assistant", text: event.body });
        } else {
          setAskMessages((prev) => prev.map((m) => (m.id === assistantId ? { ...m, text: event.body } : m)));
        }
      } else if (event.type === "card") {
        const offer = { cardInstanceId: event.cardInstanceId, cardId: event.cardId, materialId: event.materialId };
        if (!started) {
          started = true;
          appendAskMessage({ id: assistantId, role: "assistant", text: "", offer, offerPhase: "offered" });
        } else {
          setAskMessages((prev) => prev.map((m) => (m.id === assistantId ? { ...m, offer, offerPhase: "offered" } : m)));
        }
      } else if (event.type === "phase") {
        phaseMoved = true;
        const fresh = await api.getCourseSession(courseId);
        setSession(fresh);
        const nextSteps = phaseSteps(event.to);
        if (nextSteps.length > 0) go(nextSteps[0]!);
      } else if (event.type === "error") {
        if (!started) {
          started = true;
          appendAskMessage({ id: assistantId, role: "assistant", text: event.message || "出错了，请重试" });
        } else {
          setAskMessages((prev) => prev.map((m) => (m.id === assistantId ? { ...m, text: event.message || "出错了，请重试" } : m)));
        }
      }
    }
    return { phaseMoved };
  }

  async function handleAsk(text: string) {
    appendAskMessage({ id: nextAskId("student"), role: "student", text });
    setAskPending(true);
    try {
      await consumeCourseTurn(api.courseAsk(courseId, text));
    } finally {
      setAskPending(false);
    }
  }

  // The next-arrow: inside a phase it pages locally with no network call
  // (DEC-12.5); at a phase boundary it asks the coach instead. A refused
  // advance is never a modal or a lock (铁律 2) — it is a sentence in the
  // ask panel, which auto-expands because an unanswered request is worse
  // than an expanded panel.
  //
  // Without a session (the start call failed), the runtime layer is down but
  // the content layer must survive it: page locally like the pre-Slice-12
  // player, and never call courseAdvance against a session that does not
  // exist.
  async function handleNext() {
    if (!session) {
      if (course && ordinal < course.steps.length - 1) go(ordinal + 1);
      return;
    }
    const steps = phaseSteps(session.phase);
    const nextOrdinal = ordinal + 1;
    if (steps.includes(nextOrdinal)) {
      go(nextOrdinal);
      return;
    }
    setAskPending(true);
    try {
      const { phaseMoved } = await consumeCourseTurn(api.courseAdvance(courseId));
      if (!phaseMoved) setAskExpanded(true);
    } finally {
      setAskPending(false);
    }
    // The course's only legitimate exit is minted by the backend, never a
    // frame: after the stream settles, refetch the session and learn whether
    // it is now finished (the terminal branch sets status without emitting a
    // `phase` frame, since the phase itself does not move).
    const fresh = await api.getCourseSession(courseId).catch(() => null);
    if (fresh) {
      setSession(fresh);
      if (fresh.status === "finished") onFinish();
    }
  }

  function handleAcceptOffer(messageId: string) {
    setAskMessages((prev) => prev.map((m) => (m.id === messageId ? { ...m, offerPhase: "accepted" } : m)));
  }

  function handleDismissOffer(messageId: string) {
    const msg = askMessagesRef.current.find((m) => m.id === messageId);
    if (msg?.offer) void api.skipCourseCard(courseId, msg.offer.cardInstanceId);
    setAskMessages((prev) => prev.map((m) => (m.id === messageId ? { ...m, offerPhase: "resolved" } : m)));
  }

  function handleCardSubmit(messageId: string, env: import("@mind-imprint/contracts").CardInstance) {
    const msg = askMessagesRef.current.find((m) => m.id === messageId);
    if (!msg?.offer) return;
    void api.submitCourseCard(courseId, msg.offer.cardInstanceId, { field_values: env.field_values, event_trace: env.event_trace, anchors: [] });
    setAskMessages((prev) => prev.map((m) => (m.id === messageId ? { ...m, offerPhase: "resolved" } : m)));
  }

  function handleCardSkip(messageId: string) {
    handleDismissOffer(messageId);
  }

  const phaseTitle = session?.phaseTitle ?? "";
  const currentContract = session ? INFO_LITERACY_COURSE_SKILL.contracts[session.phase] : undefined;
  const askChips = currentContract?.ask_chips ?? [];
  const pageBlock = currentContract?.page;
  // I2 (whole-branch Important-2): a step-less phase (guided/reflect) has no
  // course_step render to page through — its render effect early-returns and
  // the authored `page` block just stays put, so 上一步 would visibly do
  // nothing but change the `n / total` counter. Hide it there; backward
  // paging within a step-ful phase stays ungated (gates govern unlock, never
  // revisit — DEC-12.5).
  const stepless = !!session && phaseSteps(session.phase).length === 0;
  const showAuthoredPage = stepless && !!pageBlock;
  const showBackArrow = ordinal > 0 && !stepless;

  const total = course.steps.length;
  // The Finish control is gated on the SESSION's own status, not on ordinal
  // position — the linear phase chain (demonstrate → guided → independent →
  // reflect) means the last course_step ordinal is reached mid-chain
  // (independent), well before the backend mints the terminal (reflect's
  // floor met); ordinal math alone would let a student skip straight past
  // 回看 (reflect). The backend is the only writer of `status: "finished"`
  // (see handleNext) — never client-side ordinal arithmetic.
  const courseFinished = session?.status === "finished";

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
          <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{course.title}</span>
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 9 }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: "#6B7384" }}>{phaseTitle}</span>
            <span style={{ fontSize: 11.5, fontWeight: 600, color: "#AEB4C2", background: "#F2F3F8", padding: "2px 9px", borderRadius: 999 }}>{ordinal + 1} / {total}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4, padding: "0 20px 11px" }}>
          {course.steps.map((s, i) => (
            <div key={s.id} style={{ flex: 1, height: 4, borderRadius: 3, background: i < ordinal || completed.includes(i) ? "#4C9A82" : i === ordinal ? "#2A3B7A" : "#E7E9F0" }} />
          ))}
        </div>
      </div>

      {/* body: content + ask panel — dc.html 236-397 */}
      <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
        <div style={{ flex: 1, minWidth: 0, position: "relative", display: "flex", flexDirection: "column", background: "#F3F4F8", overflowY: "auto" }}>
          <div style={{ flex: 1, padding: "36px 40px 60px" }}>
            <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
              <div style={{ fontSize: 12, fontWeight: 700, color: "#9AA1B0", letterSpacing: ".04em" }}>{phaseTitle}</div>
            </div>
            {showAuthoredPage && pageBlock ? (
              <TeachingTemplate content={{ title: pageBlock.title, subtitle: pageBlock.subtitle, body: pageBlock.body, foreground_asset_id: null }} />
            ) : !rendered ? (
              <div style={{ maxWidth: 700, margin: "0 auto", color: "#9AA1B0" }}>印记正在为你准备这一页…</div>
            ) : rendered.template === "challenge" ? (
              <ChallengeTemplate content={rendered.content as ChallengeContent} />
            ) : (
              <TeachingTemplate content={rendered.content as TeachingContent} />
            )}
          </div>

          {/* nav */}
          {showBackArrow && (
            <div aria-label="上一步" onClick={() => setOrdinal(ordinal - 1)} style={{ position: "absolute", left: 14, top: "44%", width: 40, height: 40, borderRadius: "50%", background: "#fff", border: "1px solid #E7E9F0", boxShadow: "0 3px 12px rgba(20,30,60,.10)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", color: "#6B7384" }}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
            </div>
          )}
          {!courseFinished ? (
            <div aria-label="下一步" onClick={() => void handleNext()} style={{ position: "absolute", right: 14, top: "44%", width: 44, height: 44, borderRadius: "50%", background: "#2A3B7A", boxShadow: "0 5px 16px rgba(42,59,122,.28)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6" /></svg>
            </div>
          ) : (
            <div aria-label="完成课程" onClick={onFinish} style={{ position: "absolute", right: 14, top: "44%", width: 44, height: 44, borderRadius: "50%", background: "#4C9A82", boxShadow: "0 5px 16px rgba(76,154,130,.30)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
            </div>
          )}
        </div>

        <AskPanel
          expanded={askExpanded}
          onToggle={() => setAskExpanded((e) => !e)}
          branchColor="#2A3B7A"
          context={phaseTitle}
          chips={askChips}
          messages={askMessages}
          pending={askPending}
          onSend={(text) => void handleAsk(text)}
          onAcceptOffer={handleAcceptOffer}
          onDismissOffer={handleDismissOffer}
          onCardSubmit={handleCardSubmit}
          onCardSkip={handleCardSkip}
        />
      </div>
    </div>
  );
}
