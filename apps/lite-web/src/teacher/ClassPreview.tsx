import { useEffect, useRef, useState } from "react";
import { getRoster, type RosterRow } from "../api/teacher";

/** Load a class preview only when its card is near the viewport. The existing
 * teacher roster endpoint remains the source of every count and name. */
export function ClassPreview({ classId }: { classId: string }) {
  const container = useRef<HTMLDivElement>(null);
  const [rows, setRows] = useState<RosterRow[] | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelled = false;
    setRows(null);
    setError("");
    const observer = new IntersectionObserver(entries => {
      if (!entries.some(entry => entry.isIntersecting)) return;
      observer.disconnect();
      getRoster(classId).then(data => { if (!cancelled) setRows(data); })
        .catch((e: unknown) => { if (!cancelled) setError(e instanceof Error ? e.message : String(e)); });
    }, { rootMargin: "160px" });
    if (container.current) observer.observe(container.current);
    return () => { cancelled = true; observer.disconnect(); };
  }, [classId]);
  const recent = [...(rows ?? [])].filter(row => row.lastActiveAt)
    .sort((a, b) => (b.lastActiveAt ?? "").localeCompare(a.lastActiveAt ?? "")).slice(0, 3);
  return <div className="teacher-class-preview" ref={container}>
    {error ? <p className="teacher-preview-error">概况加载失败：{error}</p> : rows === null ? <p className="teacher-preview-loading">学习概况加载中…</p> : <>
      <dl className="teacher-preview-stats">
        <div><dd>{rows.length}</dd><dt>学生</dt></div>
        <div><dd>{rows.filter(row => row.activeDaysThisWeek > 0).length}</dd><dt>本周活跃</dt></div>
        <div><dd>{rows.reduce((sum, row) => sum + row.readingsDone + row.writingsDone + row.projectsDone, 0)}</dd><dt>累计完成</dt></div>
      </dl>
      <div className="teacher-preview-recent"><p>近期活跃学生</p>{recent.length ? recent.map(row => <div key={row.id}><span className="teacher-avatar">{Array.from(row.displayName)[0]}</span><span>{row.displayName}</span><small>本周 {row.activeDaysThisWeek} 天</small></div>) : <p className="teacher-preview-empty">{rows.length ? "暂无活跃记录" : "请通过邀请码邀请学生加入"}</p>}</div>
    </>}
  </div>;
}
