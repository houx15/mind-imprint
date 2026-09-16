import { Says, errorMarkdown } from "../../Says";
import { useCallback, useEffect, useState } from "react";
import { Clock, GraduationCap, Layers, Play } from "lucide-react";
import { Icon } from "@/ui";
import { apiErrorText } from "../../../api/errorText";
import { finishPblCourse, listPblCourses, type PblCourse } from "../../../api/pblCourses";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";
import { CourseModal } from "./CourseModal";

/**
 * Course —— 去上一课。
 *
 * 印记在项目里递出一门课时打开的界面：那门课是什么、为什么现在上、上完写下它
 * 对这个项目有什么用。
 *
 * 三件事按顺序发生，界面就按这三件事分成三块：
 *
 *   1. 这是哪一门课，印记为什么现在给它
 *   2. 打开它（浮层，上完回到这里）
 *   3. 写下这一课对这个项目有什么用
 *
 * 第 3 步是必填的，`ToolFrame` 的 todo 卡在这里。理由不是流程完整性：服务端
 * 的回灌读的就是这一句（pbl_course.go · gatherPblCourseWork），少了它，她花
 * 四十分钟上完的课在下一轮对话里等于没发生。
 *
 * 🚨 「上完课」和「写了收获」是两件事，界面必须分得清：进度来自
 * course_session（服务端），收获来自这个输入框。她可以上完不写，也可以没上完
 * 就写 —— 后者照样放行（铁律④：跳过也是数据，不拦她）。
 */
export function Course({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [courses, setCourses] = useState<PblCourse[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [playing, setPlaying] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    try {
      const rows = await listPblCourses(projectId);
      setCourses(rows);
      // 已经写过的收获回填进输入框，这样她再打开这件工具看到的是自己写的话，
      // 而不是一个空框加一句「还差」。
      setDrafts((prev) => {
        const next = { ...prev };
        for (const c of rows) {
          if (next[c.id] === undefined) next[c.id] = c.takeaway;
        }
        return next;
      });
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  // 这件工具这一轮要处理的：还没写过收获的那几门。写过的留在下面当记录。
  const pending = courses.filter((c) => !c.finishedAt);
  const done = courses.filter((c) => c.finishedAt);

  const missing = pending.filter((c) => !(drafts[c.id] ?? "").trim());
  const todo =
    pending.length === 0
      ? ""
      : missing.length > 0
        ? "写下这一课对你的项目有什么用"
        : "";

  async function finish() {
    setBusy(true);
    setError(null);
    try {
      let latest = courses;
      for (const c of pending) {
        const text = (drafts[c.id] ?? "").trim();
        if (!text) continue;
        latest = await finishPblCourse(projectId, c.id, text);
      }
      setCourses(latest);
      // 交回对话的是她自己写的那句话，一个字不改 —— 和别的工具一样
      // （见 ProjectRoom.finishToolInstance 的注释）。
      const note = pending
        .map((c) => (drafts[c.id] ?? "").trim())
        .filter(Boolean)
        .join("；");
      onFinish({ courses: pending.map((c) => c.slug) }, note);
    } catch (err) {
      setError(apiErrorText(err));
      setBusy(false);
    }
  }

  return (
    <>
      <ToolFrame
        title="去上一课"
        task="上完印记给你的这一课，写下它对你的项目有什么用"
        why={tool.reason}
        todo={todo}
        onFinish={() => void finish()}
        onClose={onClose}
        busy={busy}
      >
        {error && (
          <div className="mb-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            <Says content={errorMarkdown(error)} />
          </div>
        )}

        {courses.length === 0 && (
          <p className="text-mk-small text-mk-muted">
            印记还没有给这个项目安排课程。
          </p>
        )}

        <div className="space-y-3">
          {pending.map((c) => (
            <CourseCard
              key={c.id}
              course={c}
              takeaway={drafts[c.id] ?? ""}
              onTakeaway={(v) => setDrafts((d) => ({ ...d, [c.id]: v }))}
              onPlay={() => setPlaying(c.slug)}
            />
          ))}
        </div>

        {done.length > 0 && (
          <div className="mt-5 border-t border-mk-border pt-3">
            <p className="text-mk-small font-semibold text-mk-ink">已完成</p>
            <div className="mt-2 space-y-2">
              {done.map((c) => (
                <div key={c.id} className="rounded-mk-md border border-mk-border px-3 py-2">
                  <p className="text-mk-small font-medium text-mk-ink">
                    {c.title || c.slug}
                  </p>
                  <p className="mt-0.5 text-mk-small text-mk-muted">{c.takeaway}</p>
                </div>
              ))}
            </div>
          </div>
        )}
      </ToolFrame>

      {playing && (
        <CourseModal
          slug={playing}
          onClose={() => {
            setPlaying(null);
            // 进度活在服务端，浮层关掉之后重拉一次才知道她上到哪儿了。
            void reload();
          }}
        />
      )}
    </>
  );
}

/** 一门课：封面、标题、简介、时长、她的进度，和那个输入框。 */
function CourseCard({
  course,
  takeaway,
  onTakeaway,
  onPlay,
}: {
  course: PblCourse;
  takeaway: string;
  onTakeaway: (value: string) => void;
  onPlay: () => void;
}) {
  const started = course.courseStatus !== "";
  const completed = course.courseStatus === "completed";

  return (
    <div className="rounded-mk-lg border border-mk-border p-3">
      <div className="flex gap-3">
        {course.coverUrl ? (
          <img
            src={course.coverUrl}
            alt=""
            className="h-16 w-24 shrink-0 rounded-mk-md object-cover"
          />
        ) : (
          <span
            className="flex h-16 w-24 shrink-0 items-center justify-center rounded-mk-md"
            style={{ background: "var(--mk-paper)" }}
          >
            <Icon icon={GraduationCap} size={22} className="text-mk-faint" />
          </span>
        )}
        <div className="min-w-0 flex-1">
          <p className="text-mk-body font-semibold text-mk-ink">
            {course.title || course.slug}
          </p>
          {course.blurb && (
            <p className="mt-0.5 text-mk-small text-mk-secondary">{course.blurb}</p>
          )}
          <div className="mt-1 flex flex-wrap items-center gap-3 text-mk-small text-mk-muted">
            {course.timeLabel && (
              <span className="inline-flex items-center gap-1">
                <Icon icon={Clock} size={13} />
                {course.timeLabel}
              </span>
            )}
            {course.stepCount > 0 && (
              <span className="inline-flex items-center gap-1">
                <Icon icon={Layers} size={13} />
                {course.stepCount} 步
              </span>
            )}
            <span>
              {completed
                ? "已完成"
                : started
                  ? `进行中 · ${course.completedSteps}/${course.stepCount}`
                  : "未开始"}
            </span>
          </div>
        </div>
      </div>

      {!course.available ? (
        // 课下线了，或者不再给这个版本。照实说，不画一张点不开的卡。
        <p className="mt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          这门课已经不在课程库里了。可以直接写下你从别处学到的，或者告诉印记。
        </p>
      ) : (
        <button
          type="button"
          onClick={onPlay}
          className="mt-3 flex w-full items-center justify-center gap-1.5 rounded-mk-full py-2 text-mk-body text-white"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Play} size={15} />
          {completed ? "再看一遍" : started ? "继续学习" : "开始学习"}
        </button>
      )}

      <label className="mt-3 block">
        <span className="text-mk-small font-medium text-mk-ink">
          这一课对你的项目有什么用
        </span>
        <textarea
          value={takeaway}
          onChange={(e) => onTakeaway(e.target.value)}
          rows={3}
          placeholder="请输入"
          className="mt-1 w-full resize-y rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none"
        />
      </label>
    </div>
  );
}
