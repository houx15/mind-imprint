import type { ReactNode } from "react";
import { Minimize2, Maximize2, X } from "lucide-react";
import { Icon } from "@/ui";
import { useWidePane } from "../wide";
import { MarkerUnderline } from "./sketch";

/**
 * Stage —— 可以动手的那一类工具的外壳。
 *
 * ToolFrame 是「一栏里的一件工具」：标题小、正文夹在 900px 里、底下一个"完成"。
 * 它对读一份文档、填一句话是对的。对**一块要用手摆的板**是错的：板需要整块
 * 屏幕，而且它的终点不叫"完成"，叫"形成 How might we"。
 *
 * 四张图（2026-09-03）共有的骨架，就是这个文件：
 *
 *   大标题 + 马克笔划线      这一步在做什么，一眼看得见
 *   第 N 步 / 共 M 步         有进度，有终点（铁律②「有没有进度和终点」）
 *   一句话的指令              「把你观察到的现象，放到合适的位置」
 *   ——————————————————
 *   整幅的板                  没有 900px 的阅读宽度，板要多宽给多宽
 *   ——————————————————
 *   一句发现 + 次要 + 主按钮   主按钮说的是下一件事的名字，不是"完成"
 *
 * 🚨 产品负责人 2026-09-03：「whenever we want students interact something,
 * make it wide, because we are transiting our focus. don't be mean on using the
 * right wide interactive panel.」所以正文**不夹阅读宽度**，而且用 Stage 的工具
 * 一打开就是铺开的（见 ProjectRoom 的 setWideTool）。
 *
 * ToolFrame 的三条义务一条不少地留着：她要做什么（task）、印记为什么这时候递它
 * （why）、还差什么（todo，空了才能收工）。少了 todo，"还差一步"就得每件工具
 * 自己想办法暗示，十件工具会有十种暗示法。
 */
export function Stage({
  title,
  task,
  why,
  step,
  todo,
  insight,
  children,
  onFinish,
  onClose,
  finishLabel = "完成",
  secondary,
  busy,
}: {
  title: string;
  /** 一句话：她现在要做什么。 */
  task: string;
  /** 印记递这件工具的理由，用它自己的话。 */
  why?: string;
  /** 进度。不给就不显示——有些板（比如便签板）本来就没有"第几步"。 */
  step?: { now: number; total: number };
  /** 还差什么。空 = 齐了，主按钮才亮。 */
  todo?: string;
  /** 底栏左边那句话：她刚刚做出来的东西说明了什么。 */
  insight?: ReactNode;
  children: ReactNode;
  onFinish: () => void;
  onClose: () => void;
  /** 主按钮上的字 = 下一件事的名字（「形成 How might we」），不是"完成"。 */
  finishLabel?: string;
  /** 一条退路：换个角度、先回去观察。不是每块板都有。 */
  secondary?: { label: string; onClick: () => void };
  busy?: boolean;
}) {
  const ready = !todo;
  const { wide, toggle } = useWidePane();

  return (
    <div className="flex h-full flex-col" style={{ background: "var(--mk-paper)" }}>
      <header className="shrink-0 px-5 pb-3 pt-4 sm:px-7">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            {/* 铺开的板配 mk-h1（24px）。mk-h2 是一栏里的标题，在整幅屏上小得
                像个标签——四张图里的标题都是这一步最响的那行字。 */}
            <h2 className="truncate text-mk-h1 text-mk-ink">{title}</h2>
            <MarkerUnderline width={Math.min(240, Math.max(96, title.length * 26))} />
          </div>

          <div className="flex shrink-0 items-center gap-2">
            {step && (
              <span className="rounded-mk-full px-2.5 py-1 text-mk-small tabular-nums"
                style={{ background: "var(--mk-surface)", color: "var(--mk-secondary)" }}
              >
                第 {step.now} 步 / 共 {step.total} 步
              </span>
            )}
            {/* 窄屏本来就是整屏浮层，铺开没有任何变化——那时候 toggle 不给。 */}
            {toggle && (
              <button
                type="button"
                onClick={toggle}
                aria-label={wide ? "还原宽度" : "铺开"}
                title={wide ? "还原宽度" : "铺开"}
                className="hidden rounded-mk-full p-1 text-mk-faint hover:text-mk-secondary lg:block"
              >
                <Icon icon={wide ? Minimize2 : Maximize2} size={16} />
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              aria-label="收起"
              className="rounded-mk-full p-1 text-mk-faint hover:text-mk-secondary"
            >
              <Icon icon={X} size={16} />
            </button>
          </div>
        </div>

        <p className="mt-2 flex items-start gap-2 text-mk-body text-mk-ink">
          <span
            className="mt-[7px] h-1.5 w-1.5 shrink-0 rounded-mk-full"
            style={{ background: "var(--mk-accent-500)" }}
          />
          {task}
        </p>

        {why && (
          <p
            className="mt-2 rounded-mk-md px-2.5 py-1.5 text-mk-small text-mk-secondary"
            style={{ background: "var(--mk-surface)" }}
          >
            {why}
          </p>
        )}
      </header>

      {/* 🚨 板要多宽给多宽。这里没有 max-w——那正是 ToolFrame 和 Stage 的分界。 */}
      <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4 sm:px-7">{children}</div>

      <footer
        className="shrink-0 border-t px-5 py-3 sm:px-7"
        style={{ borderColor: "var(--mk-border)", background: "var(--mk-surface)" }}
      >
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="min-w-0 flex-1 text-mk-small text-mk-secondary">
            {todo ? <span className="text-mk-muted">还差：{todo}</span> : insight}
          </p>
          <div className="flex shrink-0 items-center gap-2">
            {secondary && (
              <button
                type="button"
                onClick={secondary.onClick}
                className="rounded-mk-full border px-3.5 py-2 text-mk-small text-mk-secondary"
                style={{ borderColor: "var(--mk-border)", background: "var(--mk-paper)" }}
              >
                {secondary.label}
              </button>
            )}
            <button
              type="button"
              onClick={onFinish}
              disabled={!ready || busy}
              className="rounded-mk-full px-5 py-2 text-mk-body text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              {finishLabel} →
            </button>
          </div>
        </div>
      </footer>
    </div>
  );
}
