import { useEffect, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "../../api";

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

function CourseCard({ course, pct, onOpen }: { course: CourseSummary; pct: number | null; onOpen: () => void }) {
  const done = pct != null && pct >= 100;
  const tone = pct == null ? "未开始" : done ? "已学完" : "进行中";
  const cta = pct == null ? "开始学习" : done ? "回顾" : "继续";
  const toneStyle: React.CSSProperties = {
    display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12, fontWeight: 700, padding: "5px 11px", borderRadius: 999,
    ...(done ? { color: "#4C9A82", background: "#E7F3EE" } : pct != null ? { color: "#2A3B7A", background: "#EDEFF9" } : { color: "#8A92A3", background: "#F1F2F5" }),
  };
  return (
    <div onClick={onOpen} style={{ display: "flex", flexDirection: "column", background: "#fff", border: "1px solid #EAECF2", borderRadius: 18, overflow: "hidden", boxShadow: "0 1px 3px rgba(20,30,60,.04)", cursor: "pointer" }}>
      <div style={{ position: "relative", height: 120, background: "linear-gradient(135deg,#EDEFF9,#F3F0EC)", display: "flex", alignItems: "center", justifyContent: "center" }}>
        <span style={{ position: "absolute", top: 14, left: 16, fontSize: 11, fontWeight: 700, color: "#2A3B7A", background: "rgba(255,255,255,.85)", padding: "4px 10px", borderRadius: 999 }}>{course.branch}</span>
        <div style={{ width: 56, height: 56, borderRadius: 16, background: "#fff", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 6px 16px rgba(42,59,122,.12)" }}>
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
        </div>
      </div>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: "18px 22px 20px" }}>
        <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333", lineHeight: 1.4 }}>{course.title}</div>
        <div style={{ fontSize: 13, color: "#6B7384", lineHeight: 1.66, marginTop: 8 }}>{course.blurb}</div>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 14, fontSize: 12, color: "#8A92A3", fontWeight: 600 }}>
          <span>{course.tasks_count} 个任务 · {course.tools_count} 个工具</span>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>
            {course.time_label}
          </span>
        </div>
        {pct != null && (
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
            <div style={{ flex: 1, height: 6, background: "#EEF0F4", borderRadius: 999, overflow: "hidden" }}>
              <div style={{ width: `${pct}%`, height: "100%", background: "#2A3B7A" }} />
            </div>
            <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 700, flex: "none" }}>{pct}%</span>
          </div>
        )}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginTop: "auto", paddingTop: 16 }}>
          <span style={toneStyle}>{tone}</span>
          <button type="button" onClick={(e) => { e.stopPropagation(); onOpen(); }} style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "#2A3B7A", color: "#fff", border: "none", padding: "9px 15px", borderRadius: 10, fontSize: 13, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
            {cta}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  );
}

export function CoursesView({ onOpenCourse }: { onOpenCourse?: (id: string) => void } = {}) {
  const [courses, setCourses] = useState<CourseSummary[] | null>(null);
  const [pctById, setPctById] = useState<Record<string, number | null>>({});

  useEffect(() => {
    let cancelled = false;
    void api.listCourses().then((cs) => {
      if (cancelled) return;
      setCourses(cs);
      cs.forEach((c) => {
        void Promise.resolve(api.getCourseProgress(c.id)).then((p) => {
          if (cancelled || c.step_count === 0 || !p) return;
          const pct = Math.round((p.completed_ordinals.length / c.step_count) * 100);
          setPctById((m) => ({ ...m, [c.id]: p.completed_ordinals.length > 0 ? pct : null }));
        }).catch(() => {});
      });
    }).catch(() => { if (!cancelled) setCourses([]); });
    return () => { cancelled = true; };
  }, []);

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>课程</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6, letterSpacing: "-0.01em" }}>系统地学会一种思考方式</div>
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去「批判思维」工作台把它用在你自己的问题上。</div>
        {courses != null && courses.length === 0 ? (
          <div style={{ fontSize: 14, color: "#9AA1B0", marginTop: 28 }}>课程正在准备中，很快上线。</div>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20, marginTop: 28 }}>
            {(courses ?? []).map((c) => <CourseCard key={c.id} course={c} pct={pctById[c.id] ?? null} onOpen={() => onOpenCourse?.(c.id)} />)}
          </div>
        )}
      </div>
    </div>
  );
}
