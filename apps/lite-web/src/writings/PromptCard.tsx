import { useState } from "react";
import { Button } from "@/ui";
import type { WritingPrompt } from "../api/writingPrompts";

/**
 * PromptCard —— 题库里的一张题卡。
 *
 * # 卡上摆什么
 *
 * 一张卡要在两秒内回答三个问题：**这是哪儿的题、要写什么、写多少**。
 * 所以顶上一行标签（考试 / 年份 / 难度），中间是题面，底下是字数与时间。
 * 话题标签排在最后 —— 它是用来筛的，不是用来读的。
 *
 * # 🚨 题面要能展开
 *
 * 中考语文那种二选一的题面有三四百字，全摆出来一张卡就吃掉一屏；
 * 只摆三行又常常正好切在「题目二」前面，她根本不知道还有第二个选择。
 * 所以默认收起、给一个「展开」——**不是**靠 CSS 截断后让她自己猜下面还有没有。
 */

export function PromptCard({
  prompt,
  busy,
  onStart,
}: {
  prompt: WritingPrompt;
  busy: boolean;
  onStart: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  // 收起时给的行数。题面按自然段切，超过这个长度才需要「展开」。
  const long = prompt.text.length > 160;
  const shown = open || !long ? prompt.text : prompt.text.slice(0, 160) + "…";

  return (
    <article className="flex flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-paper p-4 shadow-mk-sm">
      <div className="flex flex-wrap items-center gap-1.5">
        <Chip text={prompt.category} tone="accent" />
        <Chip text={String(prompt.year)} />
        <Chip text={prompt.diffLabel} />
        {prompt.type && <Chip text={prompt.type} />}
      </div>

      <p className="whitespace-pre-wrap text-mk-small leading-relaxed text-mk-ink">
        {shown}
      </p>
      {long && (
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="self-start text-mk-label text-mk-accent-700 underline-offset-2 hover:underline"
        >
          {open ? "收起" : "展开全文"}
        </button>
      )}

      <p className="text-mk-label text-mk-muted">
        {prompt.taskType}
        {prompt.wordLimit ? ` · ${prompt.wordLimit}` : ""}
        {prompt.minutes ? ` · ${prompt.minutes} 分钟` : ""}
        {prompt.fullScore ? ` · ${prompt.fullScore} 分` : ""}
      </p>

      {prompt.topics.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {prompt.topics.map((t) => (
            <Chip key={t} text={t} tone="soft" />
          ))}
        </div>
      )}

      <div className="mt-auto flex items-center justify-between gap-2 pt-1">
        <span className="min-w-0 flex-1 truncate text-mk-label text-mk-muted" title={prompt.source}>
          {prompt.source}
        </span>
        <Button size="sm" loading={busy} onClick={() => onStart(prompt.id)}>
          用这道题写
        </Button>
      </div>
    </article>
  );
}

function Chip({ text, tone }: { text: string; tone?: "accent" | "soft" }) {
  // mk 令牌是裸 CSS 变量，Tailwind 的 alpha 语法在它上面一行 CSS 都不出
  //（[[tailwind-mk-token-alpha-trap]]），所以底色走 color-mix。
  const style =
    tone === "accent"
      ? {
          borderColor: "var(--mk-accent-300)",
          background: "color-mix(in srgb, var(--mk-accent-500) 12%, transparent)",
          color: "var(--mk-accent-700)",
        }
      : tone === "soft"
        ? {
            borderColor: "var(--mk-border)",
            background: "var(--mk-surface)",
            color: "var(--mk-secondary)",
          }
        : { borderColor: "var(--mk-border)", color: "var(--mk-secondary)" };
  return (
    <span
      className="rounded-mk-full border px-2 py-0.5 text-mk-label"
      style={style}
    >
      {text}
    </span>
  );
}
