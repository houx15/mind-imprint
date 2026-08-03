import { useEffect, useMemo, useState } from "react";
import type { CourseAsset, CoursePlayerPayload } from "@mind-imprint/contracts";
import { api } from "../../api";
import { SegmentTimeline } from "./SegmentTimeline";
import { AskPanel, type AskMessage } from "./AskPanel";

let localAskIdSeq = 0;
function nextAskId(prefix: string) {
  localAskIdSeq += 1;
  return `${prefix}-${Date.now()}-${localAskIdSeq}`;
}

// Task 10: CoursePlayer is now a LINEAR self-paced player — no phase runtime,
// no session, no card offers. It pages through `payload.renderCache.steps`
// by ordinal alone; the AI bar is a free helper (courseAsk), never a
// decision-maker over navigation (铁律 2: 下一步 is never gated).
export function CoursePlayer({ courseId, onExit, onFinish }: { courseId: string; onExit: () => void; onFinish: () => void }) {
  const [payload, setPayload] = useState<CoursePlayerPayload | null>(null);
  const [ordinal, setOrdinal] = useState(0);
  const [completed, setCompleted] = useState<number[]>([]);

  const [askExpanded, setAskExpanded] = useState(true);
  const [askMessages, setAskMessages] = useState<AskMessage[]>([]);
  const [askPending, setAskPending] = useState(false);

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

  if (!payload) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在载入课程…</div>;

  const steps = payload.renderCache.steps;
  const total = steps.length;
  const currentStep = steps[ordinal]!;
  const isLast = ordinal === total - 1;

  function appendAskMessage(msg: AskMessage) {
    setAskMessages((prev) => [...prev, msg]);
  }

  function go(next: number) {
    const nextCompleted = Array.from(new Set([...completed, ordinal])).sort((a, b) => a - b);
    setCompleted(nextCompleted);
    void api.saveCourseProgress(courseId, { current_ordinal: next }).catch(() => {});
    setOrdinal(next);
  }

  function handleNext() {
    if (isLast) {
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
          {/* scroll area */}
          <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
            <div style={{ padding: "36px 40px 40px" }}>
              <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
                <SegmentTimeline
                  key={ordinal}
                  stepId={currentStep.stepId}
                  content={currentStep.content}
                  assetsById={assetsById}
                  onQuizAnswer={(event) => void api.answerCourseQuiz(courseId, event).catch(() => {})}
                />
              </div>
            </div>
          </div>

          {/* nav — pinned to the page bottom so 下一步 is always reachable
              regardless of scroll (铁律 2: never gated) */}
          <div style={{ flex: "none", borderTop: "1px solid #EAECF2", background: "#fff", padding: "11px 40px" }}>
            <div style={{ maxWidth: 700, margin: "0 auto", width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
              {ordinal > 0 ? (
                <button
                  type="button"
                  aria-label="上一步"
                  onClick={() => setOrdinal(ordinal - 1)}
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
                style={{ display: "inline-flex", alignItems: "center", gap: 7, background: isLast ? "#4C9A82" : "#2A3B7A", border: "none", color: "#fff", borderRadius: 10, padding: "10px 20px", fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", boxShadow: isLast ? "0 4px 14px rgba(76,154,130,.26)" : "0 4px 14px rgba(42,59,122,.24)" }}
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
