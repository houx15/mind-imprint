import { useEffect, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "../../api";
import { coverGradientStyle } from "@/ui";

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

function CourseCard({ course, pct, onOpen }: { course: CourseSummary; pct: number | null; onOpen: () => void }) {
  const done = pct != null && pct >= 100;
  const tone = pct == null ? "未开始" : done ? "已学完" : "进行中";
  const cta = pct == null ? "开始学习" : done ? "回顾" : "继续";
  const toneStyle: React.CSSProperties = {
    display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12, fontWeight: 700, padding: "5px 11px", borderRadius: 999,
    ...(done ? { color: "var(--mk-success)", background: "var(--mk-success-bg)" } : pct != null ? { color: "var(--mk-accent-500)", background: "var(--mk-accent-50)" } : { color: "var(--mk-muted)", background: "var(--mk-paper)" }),
  };
  return (
    <div onClick={onOpen} style={{ display: "flex", flexDirection: "column", background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 18, overflow: "hidden", boxShadow: "var(--mk-shadow-xs)", cursor: "pointer" }}>
      <div style={{ position: "relative", height: 120, ...coverGradientStyle(course.slug), display: "flex", alignItems: "center", justifyContent: "center" }}>
        <span style={{ position: "absolute", top: 14, left: 16, fontSize: 11, fontWeight: 700, color: "var(--mk-ink)", background: "rgba(255,255,255,.85)", padding: "4px 10px", borderRadius: 999 }}>{course.branch}</span>
        <div style={{ width: 56, height: 56, borderRadius: 16, background: "var(--mk-surface)", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "var(--mk-shadow-md)" }}>
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="var(--mk-accent-500)" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
        </div>
      </div>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: "18px 22px 20px" }}>
        <div style={{ fontSize: 18, fontWeight: 800, color: "var(--mk-ink)", lineHeight: 1.4 }}>{course.title}</div>
        <div style={{ fontSize: 13, color: "var(--mk-secondary)", lineHeight: 1.66, marginTop: 8 }}>{course.blurb}</div>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 14, fontSize: 12, color: "var(--mk-muted)", fontWeight: 600 }}>
          <span>{course.step_count} 个任务 · {course.card_ids.length} 个工具</span>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--mk-muted)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>
            {course.time_label}
          </span>
        </div>
        {pct != null && (
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
            <div style={{ flex: 1, height: 6, background: "var(--mk-border)", borderRadius: 999, overflow: "hidden" }}>
              <div style={{ width: `${pct}%`, height: "100%", background: "var(--mk-accent-500)" }} />
            </div>
            <span style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 700, flex: "none" }}>{pct}%</span>
          </div>
        )}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginTop: "auto", paddingTop: 16 }}>
          <span style={toneStyle}>{tone}</span>
          <button type="button" onClick={(e) => { e.stopPropagation(); onOpen(); }} style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "var(--mk-accent-500)", color: "var(--mk-surface)", border: "none", padding: "9px 15px", borderRadius: 10, fontSize: 13, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
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
        void Promise.resolve(api.getCourseProgress(c.slug)).then((p) => {
          if (cancelled || c.step_count === 0 || !p) return;
          const pct = Math.round((p.completed_ordinals.length / c.step_count) * 100);
          setPctById((m) => ({ ...m, [c.slug]: p.completed_ordinals.length > 0 ? pct : null }));
        }).catch(() => {});
      });
    }).catch(() => { if (!cancelled) setCourses([]); });
    return () => { cancelled = true; };
  }, []);

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "var(--mk-muted)", fontWeight: 600 }}>课程</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "var(--mk-ink)", marginTop: 6, letterSpacing: "-0.01em" }}>系统地学会一种思考方式</div>
        <div style={{ fontSize: 14, color: "var(--mk-secondary)", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去写作工作室把它用在你自己的问题上。</div>
        {courses != null && courses.length === 0 ? (
          <div style={{ fontSize: 14, color: "var(--mk-muted)", marginTop: 28 }}>课程正在准备中，很快上线。</div>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20, marginTop: 28 }}>
            {(courses ?? []).map((c) => <CourseCard key={c.slug} course={c} pct={pctById[c.slug] ?? null} onOpen={() => onOpenCourse?.(c.slug)} />)}
          </div>
        )}
      </div>
    </div>
  );
}
