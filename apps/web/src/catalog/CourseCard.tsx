import type { CourseSummary } from "@mind-imprint/contracts";
import { coverGradientStyle } from "@/ui";

/**
 * CourseCard — the single course catalog card, shared by the 课程 list
 * (CoursesView) and the 首页 推荐课程 grid (HomePage). One rich look everywhere:
 * a 16:9 cover with the branch pill, title + blurb, three coloured meta chips
 * (任务/工具/时长), an optional progress bar, and a status pill + CTA.
 *
 * `pct` is the student's completion percentage (null = not started / no
 * denominator) — derive it with `coursePct` so both call sites compute it the
 * same way. `onRestart`, when provided, adds the 重新学 button on a finished
 * course (a list-page power action; home leaves it unwired).
 */

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

/** The student's completion % for a course, or null when there's no progress or
 * no authored steps to divide by. `completed` short-circuits to 100 so a
 * finished course reads 已学完 even if its step bookkeeping lags. */
export function coursePct(course: CourseSummary): number | null {
  const p = course.progress;
  if (!p) return null;
  if (p.status === "completed") return 100;
  if (course.step_count <= 0 || p.completedSteps <= 0) return null;
  return Math.min(100, Math.round((p.completedSteps / course.step_count) * 100));
}

/**
 * One metadata fact on a course card, styled as a soft coloured tag rather than
 * a run of grey text — so 任务/工具/时长 read as three scannable chips. Uses the
 * macaron `-bg`/`-fg` token pairs (theme-defined) so the tint and its text stay
 * legible together; the icon inherits the `-fg` colour via `currentColor`.
 */
function MetaTag({ tone, icon, children }: { tone: "lake" | "taro" | "peach"; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <span
      style={{
        display: "inline-flex", alignItems: "center", gap: 5, padding: "4px 9px", borderRadius: 999,
        fontSize: 12, fontWeight: 700, lineHeight: 1,
        background: `var(--mk-${tone}-bg)`, color: `var(--mk-${tone}-fg)`,
      }}
    >
      {icon}
      {children}
    </span>
  );
}

export function CourseCard({ course, pct, onOpen, onRestart }: { course: CourseSummary; pct: number | null; onOpen: () => void; onRestart?: () => void }) {
  const done = pct != null && pct >= 100;
  const tone = pct == null ? "未开始" : done ? "已学完" : "进行中";
  const cta = pct == null ? "开始学习" : done ? "回顾" : "继续";
  const toneStyle: React.CSSProperties = {
    display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12, fontWeight: 700, padding: "5px 11px", borderRadius: 999,
    ...(done ? { color: "var(--mk-success)", background: "var(--mk-success-bg)" } : pct != null ? { color: "var(--mk-accent-500)", background: "var(--mk-accent-50)" } : { color: "var(--mk-muted)", background: "var(--mk-paper)" }),
  };
  return (
    <div data-course-card={course.slug} onClick={onOpen} style={{ display: "flex", flexDirection: "column", background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: 18, overflow: "hidden", boxShadow: "var(--mk-shadow-xs)", cursor: "pointer" }}>
      <div style={{ position: "relative", aspectRatio: "16 / 9", ...(course.coverUrl ? {} : coverGradientStyle(course.slug)), display: "flex", alignItems: "center", justifyContent: "center", overflow: "hidden" }}>
        {course.coverUrl ? (
          <img src={course.coverUrl} alt={course.title} loading="lazy" style={{ position: "absolute", inset: 0, width: "100%", height: "100%", objectFit: "cover" }} />
        ) : (
          <div style={{ width: 56, height: 56, borderRadius: 16, background: "var(--mk-surface)", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "var(--mk-shadow-md)" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="var(--mk-accent-500)" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
          </div>
        )}
        <span style={{ position: "absolute", top: 14, left: 16, fontSize: 11, fontWeight: 700, color: "var(--mk-ink)", background: "rgba(255,255,255,.85)", padding: "4px 10px", borderRadius: 999 }}>{course.branch}</span>
      </div>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: "18px 22px 20px" }}>
        <div style={{ fontSize: 18, fontWeight: 800, color: "var(--mk-ink)", lineHeight: 1.4 }}>{course.title}</div>
        <div style={{ fontSize: 13, color: "var(--mk-secondary)", lineHeight: 1.66, marginTop: 8 }}>{course.blurb}</div>
        <div style={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 7, marginTop: 14 }}>
          <MetaTag tone="lake" icon={
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M8 6h13M8 12h13M8 18h13" /><path d="M3 6h.01M3 12h.01M3 18h.01" /></svg>
          }>{course.step_count} 个任务</MetaTag>
          <MetaTag tone="taro" icon={
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2 2 7l10 5 10-5-10-5z" /><path d="m2 17 10 5 10-5" /><path d="m2 12 10 5 10-5" /></svg>
          }>{course.card_ids.length} 个工具</MetaTag>
          <MetaTag tone="peach" icon={
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>
          }>{course.time_label}</MetaTag>
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
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            {done && onRestart && (
              <button type="button" title="重新学一遍" onClick={(e) => { e.stopPropagation(); onRestart(); }} style={{ display: "inline-flex", alignItems: "center", gap: 5, background: "var(--mk-surface)", color: "var(--mk-secondary)", border: "1px solid var(--mk-input-border)", padding: "8px 12px", borderRadius: 10, fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" /></svg>
                重新学
              </button>
            )}
            <button type="button" onClick={(e) => { e.stopPropagation(); onOpen(); }} style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "var(--mk-accent-500)", color: "var(--mk-surface)", border: "none", padding: "9px 15px", borderRadius: 10, fontSize: 13, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>
              {cta}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
