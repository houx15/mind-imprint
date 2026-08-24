import { useEffect, useMemo, useState } from "react";
import type { CourseSummary } from "@mind-imprint/contracts";
import { api } from "@/api";
import { CourseCard, coursePct } from "@/catalog/CourseCard";
import { groupCoursesByCategory } from "./groupCoursesByCategory";
import { ALL_CATEGORIES, selectCourseList, type CourseSort } from "./selectCourseList";

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
  // Category filter: "all" is every course; a category slug narrows to that
  // category. A pure UI filter — no server round-trip. 铁律②: the only orders on
  // offer are the student's own recency and plain name — never popularity, never
  // a ranking of students against each other.
  const [filter, setFilter] = useState<string>(ALL_CATEGORIES);
  const [sort, setSort] = useState<CourseSort>("recent");
  const [query, setQuery] = useState("");

  // ONE request. `listCourses` carries each card's own `progress` (server-side,
  // per student) — this used to fire a /progress call per course plus a separate
  // /history call for the recency order, an N+1 that grew with the catalog and
  // let cards pop in one ring at a time.
  useEffect(() => {
    let cancelled = false;
    void api.listCourses()
      .then((cs) => { if (!cancelled) setCourses(cs); })
      .catch(() => { if (!cancelled) setCourses([]); });
    return () => { cancelled = true; };
  }, []);

  // Completion percentage per course, derived from the same response. A course
  // with no authored steps has no denominator, so it shows no ring rather than a
  // divide-by-zero; `completed` short-circuits to 100 so a finished course reads
  // 已学完 even if its step bookkeeping lags.
  const pctById = useMemo(() => {
    const out: Record<string, number | null> = {};
    for (const c of courses ?? []) {
      const pct = coursePct(c);
      if (pct != null) out[c.slug] = pct;
    }
    return out;
  }, [courses]);

  const lastLearnedBySlug = useMemo(() => {
    const out: Record<string, string> = {};
    for (const c of courses ?? []) {
      if (c.progress) out[c.slug] = c.progress.updatedAt;
    }
    return out;
  }, [courses]);

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
      <div style={{ padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 28, fontWeight: 800, color: "var(--mk-ink)", letterSpacing: "-0.01em" }}>
          <span style={{ color: "var(--mk-muted)" }}>课程：</span>系统地学会一种思考方式
        </div>
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
              <div style={{ display: "flex", flexWrap: "wrap", gap: 9, marginTop: 14 }} data-tour="courses-categories">
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
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))", gap: 20, marginTop: 24 }} data-tour="courses-grid">
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
