import { useState } from "react";

/**
 * ThinkingFold — 印记这一轮的思考过程，默认折起来。
 *
 * 为什么放出来：这个产品教学生「不被俘获」。一个只给结论、不给过程的
 * AI，正是它教学生要警惕的那种东西——所以让她能看见这句话是怎么问出来的，
 * 是产品立场，不是调试功能。
 *
 * 为什么默认折起来：她来这儿是读文章、写东西的，不是来读模型的草稿的。
 * 摊开会把对话挤没了，而且思考过程往往比回话长一个数量级。想看的人点一下
 * 就有；不想看的人不该被它挡路。
 *
 * 🚨 内容为空时整块不渲染。一个空的折叠区读起来是「它没有想」，
 * 而实际情况是「这一档没有思考预算，没有可读的东西」——两件完全不同的事。
 * 什么时候有内容，取决于这次调用落在哪个能力档上
 * （见 docs/superpowers/specs/2026-09-02-llm-routing-taxonomy-design.md）：
 * dialogue 档关思考，所以这里通常是空的；compose / review / assess 档有。
 *
 * 它不被保存：刷新之后就没有了。模型的草稿不是她的记录，过程树才是。
 */
export function ThinkingFold({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  const body = text.trim();
  if (!body) return null;

  return (
    <div className="mt-2">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="inline-flex items-center gap-1 text-[11px] text-[color:var(--mk-ink-3)] hover:text-[color:var(--mk-ink-2)] transition-colors"
      >
        <span
          aria-hidden
          className="inline-block transition-transform duration-150"
          style={{ transform: open ? "rotate(90deg)" : "none" }}
        >
          ›
        </span>
        {open ? "收起思考过程" : "查看思考过程"}
      </button>
      {open && (
        <div className="mt-1.5 whitespace-pre-wrap rounded-lg border border-[color:var(--mk-line)] bg-[color:var(--mk-surface-2)] px-3 py-2 text-[12px] leading-relaxed text-[color:var(--mk-ink-3)]">
          {body}
        </div>
      )}
    </div>
  );
}
