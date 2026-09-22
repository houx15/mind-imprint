import "./writing-studio.css";
import { useEffect, useState } from "react";
import { PanelLeftClose, PanelLeftOpen, BookOpenText } from "lucide-react";
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
      <aside aria-label="题目" className="writing-prompt-rail is-collapsed">
        <button type="button" onClick={toggle} aria-expanded={false}
          aria-label="展开题目" title="展开题目" className="writing-prompt-tab">
          <Icon icon={PanelLeftOpen} size={16} />
          <span>题目</span>
        </button>
      </aside>
    );
  }

  return (
    <aside aria-label="题目" className="writing-prompt-rail">
      <section className="writing-prompt-card" aria-label="写作题目">
        <header className="writing-prompt-card__header">
          <span className="writing-prompt-card__mark" aria-hidden="true">
            <Icon icon={BookOpenText} size={19} />
          </span>
          <h2>写作题目</h2>
          <button type="button" onClick={toggle} aria-expanded
            aria-label="折起题目" title="折起题目" className="writing-prompt-card__fold">
            <Icon icon={PanelLeftClose} size={16} />
          </button>
        </header>
        <div className="writing-prompt-card__body mk-scroll" tabIndex={0} role="region" aria-label="题目全文">
          <p className="writing-prompt-rail__text">{prompt}</p>
        </div>
        {writing.targetWords != null && (
          <footer className="writing-prompt-card__footer">
            <span>目标字数 </span>
            <strong>{writing.targetWords}</strong>
          </footer>
        )}
      </section>
    </aside>
  );
}
