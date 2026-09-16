import { LearningSnapshot } from "./LearningSnapshot";
import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type MouseEvent } from "react";
import { getRoster, type RosterRow } from "../api/teacher";
import { postClassSummary } from "../api/classSummary";
import { failText } from "./assignmentLogic";

type SummaryState = { status: "idle" | "loading" } | { status: "ready"; text: string } | { status: "failed"; message: string };

/** Load a class preview only when its card is near the viewport. The existing
 * teacher roster endpoint remains the source of every count and name. The
 * summary above it calls a model (cached per class per day on the server), so
 * it is requested once, when the card first comes into view, and again only
 * on 重试. */
export function ClassPreview({ classId, onOpenChat }: { classId: string; onOpenChat: () => void }) {
  const container = useRef<HTMLDivElement>(null);
  const [rows, setRows] = useState<RosterRow[] | null>(null);
  const [error, setError] = useState("");
  const [summary, setSummary] = useState<SummaryState>({ status: "idle" });
  // Bumped on every class change and unmount; a summary response carrying an
  // older value is dropped.
  const summaryGen = useRef(0);

  const loadSummary = useCallback(() => {
    const gen = summaryGen.current;
    setSummary({ status: "loading" });
    postClassSummary(classId).then(
      (res) => {
        if (gen !== summaryGen.current) return;
        setSummary(
          res.summary.trim()
            ? { status: "ready", text: res.summary }
            : { status: "failed", message: "摘要生成失败：后台返回的摘要为空" },
        );
      },
      (e: unknown) => {
        if (gen === summaryGen.current) setSummary({ status: "failed", message: failText("摘要生成", e) });
      },
    );
  }, [classId]);

  useEffect(() => {
    let cancelled = false;
    setRows(null);
    setError("");
    setSummary({ status: "idle" });
    const observer = new IntersectionObserver(entries => {
      if (!entries.some(entry => entry.isIntersecting)) return;
      observer.disconnect();
      loadSummary();
      getRoster(classId).then(data => { if (!cancelled) setRows(data); })
        .catch((e: unknown) => { if (!cancelled) setError(e instanceof Error ? e.message : String(e)); });
    }, { rootMargin: "160px" });
    if (container.current) observer.observe(container.current);
    return () => { cancelled = true; summaryGen.current += 1; observer.disconnect(); };
  }, [classId, loadSummary]);

  // The card itself opens the class page. Every click or key press inside the
  // summary stops here, so it opens the conversation only. 重试 stops its own
  // key presses too, so Enter on it retries instead of opening the chat.
  const openChat = (e: MouseEvent) => { e.stopPropagation(); onOpenChat(); };
  const openChatByKey = (e: KeyboardEvent) => {
    e.stopPropagation();
    if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpenChat(); }
  };
  const retry = (e: MouseEvent) => { e.stopPropagation(); loadSummary(); };

  // Idle (the card has not come into view yet): the line is blank, keeping
  // its height, since no request has started.
  const recent = [...(rows ?? [])].filter(row => row.lastActiveAt)
    .sort((a, b) => (b.lastActiveAt ?? "").localeCompare(a.lastActiveAt ?? "")).slice(0, 3);
  return <div className="teacher-class-preview" ref={container}>
    <div className={`teacher-class-summary${summary.status === "failed" ? " is-failed" : ""}`} role="button" tabIndex={0} onClick={openChat} onKeyDown={openChatByKey}>
      <p className="teacher-class-summary-label">本周摘要</p>
      {summary.status === "failed" ? <>
        <p role="alert">{summary.message}</p>
        <button type="button" className="teacher-class-summary-retry" onClick={retry} onKeyDown={e => e.stopPropagation()}>重试</button>
      </> : <p>{summary.status === "ready" ? summary.text : summary.status === "loading" ? "摘要生成中" : " "}</p>}
      <span className="teacher-class-summary-link">进入班级对话 ↗</span>
    </div>
    {error ? <p className="teacher-preview-error">概况加载失败：{error}</p> : rows === null ? <p className="teacher-preview-loading">学习概况加载中…</p> : <>
      <LearningSnapshot rows={rows} compact />
      <div className="teacher-preview-recent"><p>近期活跃学生</p>{recent.length ? recent.map(row => <div key={row.id}><span className="teacher-avatar">{Array.from(row.displayName)[0]}</span><span>{row.displayName}</span><small>本周 {row.activeDaysThisWeek} 天</small></div>) : <p className="teacher-preview-empty">{rows.length ? "暂无活跃记录" : "请通过邀请码邀请学生加入"}</p>}</div>
    </>}
  </div>;
}
