import { useEffect, useState } from "react";
import type { Course, RenderedStep } from "@mind-imprint/contracts";
import { api } from "../../api";
import { TeachingTemplate, type TeachingContent } from "./TeachingTemplate";
import { ChallengeTemplate, type ChallengeContent } from "./ChallengeTemplate";

export function CoursePlayer({ courseId, onExit }: { courseId: string; onExit: () => void }) {
  const [course, setCourse] = useState<Course | null>(null);
  const [ordinal, setOrdinal] = useState(0);
  const [rendered, setRendered] = useState<RenderedStep | null>(null);
  const [completed, setCompleted] = useState<number[]>([]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const c = await api.getCourse(courseId);
      if (cancelled) return;
      setCourse(c);
      try {
        const p = await api.getCourseProgress(courseId);
        if (cancelled) return;
        setCompleted(p.completed_ordinals);
        setOrdinal(Math.min(p.current_ordinal, c.steps.length - 1));
      } catch { /* default 0 */ }
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  useEffect(() => {
    if (!course) return;
    let cancelled = false;
    setRendered(null);
    void api.renderCourseStep(courseId, ordinal).then((r) => { if (!cancelled) setRendered(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [course, courseId, ordinal]);

  if (!course) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在载入课程…</div>;

  const total = course.steps.length;
  const isLast = ordinal >= total - 1;

  function go(next: number) {
    const nextCompleted = Array.from(new Set([...completed, ordinal])).sort((a, b) => a - b);
    setCompleted(nextCompleted);
    void api.saveCourseProgress(courseId, { current_ordinal: next, completed_ordinals: nextCompleted }).catch(() => {});
    setOrdinal(next);
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
          <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{course.title}</span>
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 9 }}>
            <span style={{ fontSize: 11.5, fontWeight: 600, color: "#AEB4C2", background: "#F2F3F8", padding: "2px 9px", borderRadius: 999 }}>{ordinal + 1} / {total}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 4, padding: "0 20px 11px" }}>
          {course.steps.map((s, i) => (
            <div key={s.id} style={{ flex: 1, height: 4, borderRadius: 3, background: i < ordinal || completed.includes(i) ? "#4C9A82" : i === ordinal ? "#2A3B7A" : "#E7E9F0" }} />
          ))}
        </div>
      </div>

      {/* body */}
      <div style={{ flex: 1, minHeight: 0, position: "relative", display: "flex", flexDirection: "column", background: "#F3F4F8", overflowY: "auto" }}>
        <div style={{ flex: 1, padding: "36px 40px 60px" }}>
          {!rendered ? (
            <div style={{ maxWidth: 700, margin: "0 auto", color: "#9AA1B0" }}>印记正在为你准备这一页…</div>
          ) : rendered.template === "challenge" ? (
            <ChallengeTemplate content={rendered.content as ChallengeContent} />
          ) : (
            <TeachingTemplate content={rendered.content as TeachingContent} />
          )}
        </div>

        {/* nav */}
        {ordinal > 0 && (
          <div aria-label="上一步" onClick={() => setOrdinal(ordinal - 1)} style={{ position: "absolute", left: 14, top: "44%", width: 40, height: 40, borderRadius: "50%", background: "#fff", border: "1px solid #E7E9F0", boxShadow: "0 3px 12px rgba(20,30,60,.10)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", color: "#6B7384" }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
          </div>
        )}
        {!isLast ? (
          <div aria-label="下一步" onClick={() => go(ordinal + 1)} style={{ position: "absolute", right: 14, top: "44%", width: 44, height: 44, borderRadius: "50%", background: "#2A3B7A", boxShadow: "0 5px 16px rgba(42,59,122,.28)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6" /></svg>
          </div>
        ) : (
          <div aria-label="完成课程" onClick={() => { go(ordinal); onExit(); }} style={{ position: "absolute", right: 14, top: "44%", width: 44, height: 44, borderRadius: "50%", background: "#4C9A82", boxShadow: "0 5px 16px rgba(76,154,130,.30)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
        )}
      </div>
    </div>
  );
}
