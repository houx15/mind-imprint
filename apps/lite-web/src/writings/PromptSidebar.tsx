import { useEffect, useState } from "react";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { Icon } from "@/ui";
import { assignedPromptOf, type Writing } from "../api/writings";

/**
 * PromptSidebar —— 写作时挂在左边的那一栏题目，可以折起来。
 *
 * # 为什么从顶上挪下来
 *
 * 产品负责人 2026-09-21：
 *
 *   「during writing. the prompt should be a left sidebar that can be folded.
 *     because the top area is not enough to display」
 *
 * 原来它是标题底下一行 `line-clamp-2` 的小字（AssignedPromptLine），全文只在
 * `title` 属性里 —— 鼠标悬停才看得见，手机上根本看不见。老师布置的题目本来就
 * 常常有三五行，而题库那 705 道里中考语文的二选一题面有三四百字：
 * 两行的地方装不下，她写到一半想回去核一下要求，只能滑到顶上把鼠标停在那儿。
 *
 * 题目是她写这一篇**全程都要看的东西**，不是一句一次性的交代。所以给它一栏。
 *
 * # 折起来这件事要记住
 *
 * 屏幕窄的时候这一栏会挤掉正文，所以能折。折没折记在 localStorage 里，
 * 按**这一篇**记 —— 她在这一篇折起来了，不该影响另一篇；而同一篇下次进来
 * 应该还是她上次那个样子。
 *
 * 🚨 localStorage 读写都包在 try 里：无痕窗口、关掉站点数据的浏览器里它会抛，
 * 而一个读不到偏好的侧栏应该默认展开，不是整屏白掉。
 */

function storeKey(writingId: string): string {
  return `mk:prompt-rail:${writingId}`;
}

function readCollapsed(writingId: string): boolean {
  try {
    return localStorage.getItem(storeKey(writingId)) === "1";
  } catch {
    return false;
  }
}

export function PromptSidebar({ writing }: { writing: Writing }) {
  const prompt = assignedPromptOf(writing);
  const [collapsed, setCollapsed] = useState(() => readCollapsed(writing.id));

  // 换了一篇就重读那一篇自己的偏好。
  useEffect(() => {
    setCollapsed(readCollapsed(writing.id));
  }, [writing.id]);

  function toggle() {
    const next = !collapsed;
    setCollapsed(next);
    try {
      localStorage.setItem(storeKey(writing.id), next ? "1" : "0");
    } catch {
      // 存不住就只是这一次有效，不影响她现在看不看得见题目。
    }
  }

  // 她自己开的那一篇没有题目，这一栏整个不出现。
  if (!prompt) return null;

  if (collapsed) {
    return (
      <aside
        aria-label="题目"
        className="flex w-10 shrink-0 flex-col items-center border-r border-mk-border bg-mk-surface py-3"
      >
        <button
          type="button"
          onClick={toggle}
          aria-expanded={false}
          title="展开题目"
          className="flex flex-col items-center gap-2 rounded-mk-sm px-1 py-2 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700"
        >
          <Icon icon={PanelLeftOpen} size={16} />
          {/* 竖排「题目」两个字：折起来之后这一栏还得说得出自己是什么。 */}
          <span className="text-mk-label [writing-mode:vertical-rl]">题目</span>
        </button>
      </aside>
    );
  }

  return (
    <aside
      aria-label="题目"
      // 240 而不是 280：构思那一屏上它要和对话、思维导图并排
      //（2026-09-22 起三栏），而那张图本来就不宽。段落和成稿两页上
      // 少 40px 看不出来，构思那一屏上这 40px 是图的第四张卡。
      className="mk-scroll flex w-[240px] shrink-0 flex-col overflow-y-auto border-r border-mk-border bg-mk-surface"
    >
      <div className="flex items-center justify-between gap-2 border-b border-mk-border px-3 py-2">
        <span className="text-mk-caption text-mk-muted">题目</span>
        <button
          type="button"
          onClick={toggle}
          aria-expanded
          title="折起题目"
          className="rounded-mk-sm p-1 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700"
        >
          <Icon icon={PanelLeftClose} size={16} />
        </button>
      </div>
      <p className="whitespace-pre-wrap px-3 py-3 text-mk-small leading-relaxed text-mk-ink">
        {prompt}
      </p>
      {writing.targetWords != null && (
        <p className="px-3 pb-3 text-mk-label text-mk-muted">
          目标字数 {writing.targetWords}
        </p>
      )}
    </aside>
  );
}
