import { PROJECT_STATUS_LABELS, kindLabel, projectTitle, type Project } from "../api/projects";
import { resolveCover } from "./covers";

/**
 * 看板/列表上的一个项目。
 *
 * 产品负责人 2026-09-02：卡片要更大，带上名字、日期、当前这一步、最近更新。
 *
 * 上面写的是**这个项目现在停在哪**，不是它完成了百分之几。没有进度条、没有
 * 百分比、没有连续天数——一块给项目互相打分的板，会把"我在做什么"变成"我哪个
 * 落后了"，而这正是铁律②要挡住的那种压力。
 *
 * 「第 2 步 / 共 4 步」是位置，不是分数：她要知道自己走到哪儿了，不需要被提醒
 * 还差多少。
 */

/** 「今天」「昨天」「3 天前」——具体到年月日对她没有意义。 */
function whenLabel(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const days = Math.floor((Date.now() - then) / 86_400_000);
  if (days <= 0) return "今天";
  if (days === 1) return "昨天";
  if (days < 30) return `${days} 天前`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months} 个月前`;
  return `${Math.floor(months / 12)} 年前`;
}

export function ProjectCard({ project, onOpen }: { project: Project; onOpen: (p: Project) => void }) {
  // 走 resolveCover，看板和别处不会对同一个还没设封面的项目给出两种颜色。
  const { ground, glyph } = resolveCover(project.kind, project.coverGround, project.coverGlyph);
  const kind = kindLabel(project.kind);
  const step = project.currentStep.trim();

  return (
    <button
      type="button"
      onClick={() => onOpen(project)}
      className="group flex w-full flex-col gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-4 text-left shadow-mk-xs transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <div className="flex items-start gap-3">
        <span
          aria-hidden
          className="flex h-12 w-12 shrink-0 items-center justify-center rounded-mk-md text-[21px] leading-none"
          style={{
            background: `linear-gradient(145deg, ${ground.from}, ${ground.to})`,
            color: ground.ink,
          }}
        >
          {glyph}
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-mk-body font-semibold text-mk-ink">
            {projectTitle(project)}
          </span>
          {/* 她自己最初那句话。名字是我们给的起点，这句才是她说的。 */}
          <span className="line-clamp-2 text-mk-small text-mk-secondary">{project.idea}</span>
        </span>
      </div>

      {/* 现在停在哪 */}
      <div className="rounded-mk-md bg-mk-paper px-3 py-2">
        {step ? (
          <>
            <p className="truncate text-mk-small text-mk-ink">{step}</p>
            <p className="mt-0.5 text-mk-small text-mk-muted">
              第 {project.stepsDone + 1} 步 / 共 {project.stepsTotal} 步
            </p>
          </>
        ) : (
          <p className="text-mk-small text-mk-muted">
            {project.stepsTotal > 0 ? "计划里的事都做完了" : "还没有计划，你们还在聊"}
          </p>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-mk-small text-mk-muted">
        <span
          className="rounded-mk-full px-2 py-0.5"
          style={{
            background: "color-mix(in srgb, var(--mk-accent-500) 12%, transparent)",
            color: "var(--mk-secondary)",
          }}
        >
          {PROJECT_STATUS_LABELS[project.status]}
        </span>
        {kind && <span>{kind}</span>}
        <span className="ml-auto">开始于 {whenLabel(project.createdAt)}</span>
        <span>· 最近 {whenLabel(project.lastActivityAt)}</span>
      </div>
    </button>
  );
}
