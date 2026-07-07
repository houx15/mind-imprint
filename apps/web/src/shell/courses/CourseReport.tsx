import { useEffect, useState } from "react";
import type { Course } from "@mind-imprint/contracts";
import { api } from "../../api";

function Stat({ value, label, color }: { value: string; label: string; color?: string }) {
  return (
    <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "16px 14px", textAlign: "center" }}>
      <div style={{ fontSize: 22, fontWeight: 800, color: color ?? "#1C2333" }}>{value}</div>
      <div style={{ fontSize: 12, color: "#8A92A3", fontWeight: 600, marginTop: 3 }}>{label}</div>
    </div>
  );
}

export function CourseReport({ courseId, onBackToCourses, onGoPortal }: { courseId: string; onBackToCourses: () => void; onGoPortal: () => void }) {
  const [course, setCourse] = useState<Course | null>(null);
  const [completed, setCompleted] = useState<number[]>([]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const c = await api.getCourse(courseId);
      let comp: number[] = [];
      try { comp = (await api.getCourseProgress(courseId)).completed_ordinals; } catch { /* none */ }
      if (cancelled) return;
      setCompleted(comp);
      setCourse(c);
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  if (!course) return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的学习报告…</div>;

  const challenges = course.steps.filter((s) => s.kind === "challenge");
  const learnings = course.steps.filter((s) => s.kind === "teaching" && s.purpose).map((s) => s.purpose);

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        {/* hero */}
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>学习报告 · 课程完成</div>
            <div style={{ fontSize: 24, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>{course.title}</div>
            <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>{course.branch} · 恭喜你走完这一程，下面是你留下的印记。</div>
          </div>
        </div>

        {/* stats */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 12, marginTop: 16 }}>
          <Stat value={course.time_label} label="用时" />
          <Stat value={`${completed.length} / ${course.steps.length}`} label="阶段完成" />
          <Stat value={`${challenges.length}`} label="挑战通过" color="#D98263" />
          <Stat value={`${course.tools_count}`} label="工具收集" color="#4C9A82" />
        </div>

        {/* learned */}
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>你学到了什么</div>
          {learnings.map((l, i) => (
            <div key={i} style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 10 }}>
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 2 }}><path d="M20 6L9 17l-5-5" /></svg>
              <span style={{ fontSize: 14.5, color: "#2B3346", lineHeight: 1.6 }}>{l}</span>
            </div>
          ))}
        </div>

        {/* challenges */}
        {challenges.length > 0 && (
          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "22px 24px", marginTop: 16 }}>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>挑战回顾</div>
            {challenges.map((c) => (
              <div key={c.id} style={{ display: "flex", gap: 12, alignItems: "flex-start", padding: "12px 0", borderBottom: "1px solid #F3F4F7" }}>
                <div style={{ flex: "none", width: 30, height: 30, borderRadius: 9, background: "#FBEEE7", display: "flex", alignItems: "center", justifyContent: "center" }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#D98263" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M13 2L3 14h7l-1 8 10-12h-7z" /></svg>
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{c.purpose || "挑战"}</div>
                </div>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 5 }}><path d="M20 6L9 17l-5-5" /></svg>
              </div>
            ))}
          </div>
        )}

        {/* actions */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 22 }}>
          <button type="button" onClick={onBackToCourses} style={{ flex: "none", background: "#fff", border: "1px solid #E1E4ED", color: "#6B7384", fontSize: 14, fontWeight: 700, padding: "13px 20px", borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>返回课程</button>
          <button type="button" onClick={onGoPortal} style={{ flex: 1, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: "#4C9A82", color: "#fff", border: "none", fontSize: 14.5, fontWeight: 700, padding: 13, borderRadius: 12, cursor: "pointer", fontFamily: "inherit" }}>
            去批判思维工作台，用起来
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}
