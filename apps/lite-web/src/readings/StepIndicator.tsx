import { useEffect, useRef, useState } from "react";
import { Check, Flag, MapPin, SkipForward, X } from "lucide-react";
import type { ReadingTask } from "../api/readingRoom";
import { studentArtwork } from "../learning/StudentArtwork";

/** Task navigation previews real server tasks; it never advances or completes them. */
export function StepIndicator({ tasks, onLocate }: { tasks: ReadingTask[]; onLocate?: (blockId: string) => void }) {
  const index = tasks.findIndex(task => task.status === "pending");
  const current = index < 0 ? null : tasks[index]!;
  const [previewId, setPreviewId] = useState<string | null>(null);
  const [transition, setTransition] = useState<string | null>(null);
  const previous = useRef<string | null>(current?.id ?? null);
  useEffect(() => {
    const previousTask = tasks.find(task => task.id === previous.current);
    if (previous.current && previous.current !== current?.id && previousTask && previousTask.status !== "pending") {
      setTransition(`${previousTask.status === "done" ? "已完成" : "已跳过"} · ${previousTask.label}`);
      setPreviewId(null);
    }
    previous.current = current?.id ?? null;
  }, [current?.id, tasks]);
  useEffect(() => {
    if (!transition) return;
    const timer = window.setTimeout(() => setTransition(null), 4500);
    return () => window.clearTimeout(timer);
  }, [transition]);
  if (!tasks.length) return null;
  const preview = tasks.find(task => task.id === previewId);
  return <section className="mk-stepnow reading-quest" aria-label="阅读任务">
    <div className="reading-quest__heading" role="status" aria-live="polite">
      <span className="reading-quest__emblem" aria-hidden="true">{current ? String(index+1).padStart(2,"0") : <Flag size={23}/>}</span>
      <div><span className="reading-quest__position">{current ? `第 ${index+1} 步 / 共 ${tasks.length} 步` : `带读走完了 · 共 ${tasks.length} 步`}</span>
        {current && <p key={current.id} className="mk-stepnow__label">{current.label}</p>}
      </div>
      {current?.blockId && onLocate && <button className="reading-quest__locate" type="button" onClick={()=>onLocate(current.blockId)}><MapPin size={14}/>定位原文</button>}
    </div>
    <ol className="reading-quest__path" aria-label="任务路线">
      {tasks.map((task,i)=><li key={task.id} data-state={task.status} data-current={task.id===current?.id || undefined}>
        <button type="button" onClick={()=>setPreviewId(previewId===task.id ? null : task.id)} aria-expanded={previewId===task.id} aria-label={`查看任务 ${i+1}：${task.label}`} title={task.label}>
          {task.status==="done" ? <Check size={15}/> : task.status==="skipped" ? <SkipForward size={13}/> : i+1}
        </button>
      </li>)}
    </ol>
    {transition && <p className="reading-quest__transition" role="status">{transition}</p>}
    {preview && <div className="reading-quest__preview">
      <img className="reading-quest__art" src={studentArtwork.quest} alt="" />
      <button className="reading-quest__close" type="button" aria-label="关闭任务预览" onClick={()=>setPreviewId(null)}><X size={14}/></button>
      <span>{preview.status==="done" ? "已完成" : preview.status==="skipped" ? "已跳过" : preview.id===current?.id ? "当前任务" : "待完成 · 任务预览"}</span>
      <strong>{preview.label}</strong>{preview.detail && <p>{preview.detail}</p>}
      {preview.blockId && onLocate && <button className="reading-quest__source" type="button" onClick={()=>onLocate(preview.blockId)}>查看对应原文 ↗</button>}
    </div>}
  </section>;
}
