import { useEffect, useMemo, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "@/api";
import { coverGradientStyle } from "@/ui";
import { groupCoursesByCategory } from "./groupCoursesByCategory";
import { ALL_CATEGORIES, selectCourseList, type CourseSort } from "./selectCourseList";

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

function CourseCard({ course, pct, onOpen, onRestart }: { course: CourseSummary; pct: number | null; onOpen: () => void; onRestart?: () => void }) {
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

function FilterChip({ label, count, active, onClick }: { label: string; count: number; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      style={{
        display: "inline-flex", alignItems: "center", gap: 6, padding: "7px 14px", borderRadius: 999,
        fontSize: 13, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", transition: "background .12s,border-color .12s,color .12s",
        ...(active
          ? { background: "var(--mk-accent-500)", color: "var(--mk-surface)", border: "1px solid var(--mk-accent-500)" }
          : { background: "var(--mk-surface)", color: "var(--mk-secondary)", border: "1px solid var(--mk-border)" }),
      }}
    >
      {label}
      <span style={{ fontSize: 11.5, fontWeight: 700, opacity: active ? 0.85 : 0.55 }}>{count}</span>
    </button>
  );
}

/** Search by course NAME. Filters as you type — no submit, nothing to wait for. */
function SearchField({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div style={{ position: "relative", flex: "1 1 260px", maxWidth: 340 }}>
      <svg
        width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="var(--mk-faint)" strokeWidth="2.2" strokeLinecap="round"
        style={{ position: "absolute", left: 12, top: "50%", transform: "translateY(-50%)", pointerEvents: "none" }}
      >
        <circle cx="11" cy="11" r="7" />
        <path d="M20 20l-3.6-3.6" />
      </svg>
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="搜索课程名称"
        aria-label="搜索课程名称"
        style={{
          width: "100%", padding: "8px 12px 8px 33px", borderRadius: 999, fontFamily: "inherit", fontSize: 13,
          color: "var(--mk-ink)", background: "var(--mk-surface)", border: "1px solid var(--mk-input-border)", outline: "none",
        }}
      />
    </div>
  );
}

const SORT_OPTIONS: { value: CourseSort; label: string }[] = [
  { value: "recent", label: "最近学习" },
  { value: "name", label: "按名称" },
];

/** Two-way order toggle. 最近学习 is the default — you land on what you were mid-way through. */
function SortToggle({ sort, onChange }: { sort: CourseSort; onChange: (s: CourseSort) => void }) {
  return (
    <div role="group" aria-label="课程排序" style={{ display: "inline-flex", padding: 3, gap: 3, borderRadius: 999, background: "var(--mk-paper)", border: "1px solid var(--mk-border)" }}>
      {SORT_OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          onClick={() => onChange(o.value)}
          aria-pressed={sort === o.value}
          style={{
            padding: "5px 12px", borderRadius: 999, border: "none", cursor: "pointer", fontFamily: "inherit",
            fontSize: 12.5, fontWeight: 700, transition: "background .12s,color .12s",
            ...(sort === o.value
              ? { background: "var(--mk-surface)", color: "var(--mk-ink)", boxShadow: "var(--mk-shadow-xs)" }
              : { background: "transparent", color: "var(--mk-muted)" }),
          }}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function CoursesView({ onOpenCourse, onRestartCourse }: { onOpenCourse?: (id: string) => void; onRestartCourse?: (id: string) => void } = {}) {
  const [courses, setCourses] = useState<CourseSummary[] | null>(null);
  const [pctById, setPctById] = useState<Record<string, number | null>>({});
  // slug → ISO timestamp of the student's last activity, from the same history
  // endpoint 学习记录 uses. Drives the default "最近学习" order; an empty map (no
  // history yet, or the call failed) simply leaves the catalog order alone.
  const [lastLearnedBySlug, setLastLearnedBySlug] = useState<Record<string, string>>({});
  // Category filter: "all" is every course; a category slug narrows to that
  // category. A pure UI filter — no server round-trip. 铁律②: the only orders on
  // offer are the student's own recency and plain name — never popularity, never
  // a ranking of students against each other.
  const [filter, setFilter] = useState<string>(ALL_CATEGORIES);
  const [sort, setSort] = useState<CourseSort>("recent");
  const [query, setQuery] = useState("");

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
    // Recency is a nice-to-have on top of the catalog: if it fails the list
    // still renders, just in catalog order.
    void api.getCourseHistory().then((items) => {
      if (cancelled) return;
      setLastLearnedBySlug(Object.fromEntries(items.map((i) => [i.slug, i.updatedAt])));
    }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

  // Groups feed the chips only (label + count) — the list itself is flat.
  const groups = useMemo(() => groupCoursesByCategory(courses ?? []), [courses]);
  // If the active chip names a category that no longer exists (the catalog
  // changed under it), fall back to 全部 rather than a blank page.
  const activeFilter = filter === ALL_CATEGORIES || groups.some((g) => g.slug === filter) ? filter : ALL_CATEGORIES;
  const shown = useMemo(
    () => selectCourseList({ courses: courses ?? [], category: activeFilter, query, sort, lastLearnedBySlug }),
    [courses, activeFilter, query, sort, lastLearnedBySlug],
  );

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "var(--mk-muted)", fontWeight: 600 }}>课程</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "var(--mk-ink)", marginTop: 6, letterSpacing: "-0.01em" }}>系统地学会一种思考方式</div>
        <div style={{ fontSize: 14, color: "var(--mk-secondary)", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去写作工作室把它用在你自己的问题上。</div>
        {courses != null && courses.length === 0 ? (
          <div style={{ fontSize: 14, color: "var(--mk-muted)", marginTop: 28 }}>课程正在准备中，很快上线。</div>
        ) : (
          <>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 26, flexWrap: "wrap" }}>
              <SearchField value={query} onChange={setQuery} />
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginLeft: "auto" }}>
                <span style={{ fontSize: 12.5, color: "var(--mk-muted)", fontWeight: 600 }}>排序</span>
                <SortToggle sort={sort} onChange={setSort} />
              </div>
            </div>
            {groups.length > 0 && (
              <div style={{ display: "flex", flexWrap: "wrap", gap: 9, marginTop: 14 }}>
                <FilterChip label="全部" count={courses?.length ?? 0} active={activeFilter === ALL_CATEGORIES} onClick={() => setFilter(ALL_CATEGORIES)} />
                {groups.map((g) => (
                  <FilterChip key={g.slug} label={g.label} count={g.courses.length} active={activeFilter === g.slug} onClick={() => setFilter(g.slug)} />
                ))}
              </div>
            )}
            {shown.length === 0 ? (
              <div style={{ fontSize: 14, color: "var(--mk-muted)", marginTop: 28 }}>
                没有名字里含「{query.trim()}」的课程{activeFilter === ALL_CATEGORIES ? "" : "（当前分类下）"}。
              </div>
            ) : (
              <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20, marginTop: 24 }}>
                {shown.map((c) => (
                  <CourseCard key={c.slug} course={c} pct={pctById[c.slug] ?? null} onOpen={() => onOpenCourse?.(c.slug)} onRestart={onRestartCourse ? () => onRestartCourse(c.slug) : undefined} />
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
