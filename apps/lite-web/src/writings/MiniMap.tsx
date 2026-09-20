import { useState } from "react";
import { Icon } from "@/ui";
import { Maximize2, X } from "lucide-react";
import { MindMap } from "./MindMap";
import type { WritingOutlineItem } from "../api/writingRoom";

/**
 * MiniMap —— 段落那一步左栏顶上那张缩略的思维导图。
 *
 * # 为什么留着它（同事 2026-09-20 的意见 5）
 *
 * 他在截图上画了一个箭头指着左栏顶部，写着：
 *
 *	「我觉得可以在这里保留刚刚的思维导图，然后把引导往下放」
 *
 * 到了段落那一步，她眼前只剩一张卡片和一张纸 —— 前面想了半天的那张图不见了。
 * 于是「这一段在整篇里是第几块、它两边是什么」这件事，她得靠记。
 *
 * # 只读
 *
 * 不传 `onMove` / `onRemove` / `onEdit`：这一步不是改结构的地方。
 * 要改回「结构」那一步去改 —— 在两个地方都能改同一样东西，是让她不知道
 * 哪一处才算数的最快办法。
 */
export function MiniMap({ outline }: { outline: WritingOutlineItem[] }) {
  const [open, setOpen] = useState(false);

  if (outline.length === 0) return null;

  return (
    <>
      <section className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-mk-label font-semibold text-mk-accent-700">这一篇的结构</h2>
          <button
            type="button"
            onClick={() => setOpen(true)}
            aria-label="放大看这张图"
            title="放大看这张图"
            className="flex items-center gap-1 text-mk-small text-mk-muted hover:text-mk-ink"
          >
            <Icon icon={Maximize2} size={13} />
            放大
          </button>
        </div>
        {/* 缩略图整体缩到 62%：一屏里看得见形状就够，读字回去放大。 */}
        <div
          className="h-[168px] overflow-hidden rounded-mk-md border"
          style={{ borderColor: "var(--mk-border)" }}
        >
          <div
            className="pointer-events-none origin-top-left"
            style={{ transform: "scale(0.62)", width: "161%", height: "161%" }}
          >
            <MindMap items={outline} justAdded={[]} />
          </div>
        </div>
      </section>

      {open && (
        <div
          className="fixed inset-0 z-50 flex flex-col"
          style={{ background: "color-mix(in srgb, var(--mk-ink) 55%, transparent)" }}
          onClick={() => setOpen(false)}
        >
          <div
            className="m-auto flex h-[80vh] w-[min(1100px,92vw)] flex-col rounded-mk-lg border shadow-mk-lg"
            style={{ borderColor: "var(--mk-border)", background: "var(--mk-paper)" }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--mk-border)" }}>
              <h2 className="text-mk-body font-semibold text-mk-ink">这一篇的结构</h2>
              <button type="button" onClick={() => setOpen(false)} aria-label="关闭" className="text-mk-muted hover:text-mk-ink">
                <Icon icon={X} size={18} />
              </button>
            </div>
            <div className="min-h-0 flex-1">
              {/* 放大之后仍然只读 —— 见文件顶上那段。 */}
              <MindMap items={outline} justAdded={[]} />
            </div>
          </div>
        </div>
      )}
    </>
  );
}
