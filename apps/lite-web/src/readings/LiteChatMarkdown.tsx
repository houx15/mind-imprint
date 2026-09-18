import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkCjkFriendly from "remark-cjk-friendly";
import type { Components } from "react-markdown";

/**
 * LiteChatMarkdown — lite's own chat-bubble markdown renderer.
 *
 * A copy of `@/studio/ai/ChatMarkdown` with exactly ONE difference: `**bold**`
 * lands in the accent colour instead of in ink.
 *
 * Why a copy rather than a prop on the shared one: that component styles the
 * chat bubbles of four pro rooms (Plan / Reading / Writing / Review), and
 * `apps/web/` is off limits to lite work — recolouring bold there would
 * recolour every bubble in the pro product to make a point about lite.
 *
 * Why recolour at all: the complaint that started this was that 印记's replies
 * read as "very text-heavy and students won't have patience to read it
 * thoroughly". Bold that is merely bold, in the same ink as everything around
 * it, is a distinction a middle-schooler skimming a 200-字 reply does not
 * make. Colour is the thing her eye actually lands on. This buys the prompt's
 * new permission (bold ONE key word) an effect worth having.
 *
 * `--mk-accent-600` (#CE3A2B), not `-500` (#EA5140): 500 is the interactive
 * accent — buttons, rails, the pebble — and at 14px body weight it sits around
 * 3.9:1 on paper, under the 4.5:1 floor for text. 600 is the same hue two
 * steps down, ~5.4:1, and reads as emphasis rather than as a link.
 *
 * ⚠️ Applied as an inline style holding the bare CSS variable. `mk-*` tokens
 * are bare custom properties, so Tailwind's alpha syntax (`text-mk-accent/80`)
 * emits no CSS at all for them — an arbitrary-value class here would be a
 * silent no-op.
 *
 * `remark-cjk-friendly`: CommonMark does not close `**` when the bold text
 * ends in CJK punctuation and a CJK character follows (`**这一步做完。**下一步`
 * — the closing run is not "right-flanking"), so 印记's bold came through as
 * literal asterisks (colleague report, 2026-09-18). The plugin relaxes the
 * flanking rule for CJK text; `apps/web`'s card modal uses the same one.
 *
 * AI-role messages ONLY — student turns stay plain `whitespace-pre-wrap` text
 * in `ReadingCoachPanel` (a student typing a literal `*` or `#` must never be
 * silently swallowed as markup).
 */
const components: Components = {
  p: ({ children }) => <p className="mb-2">{children}</p>,
  strong: ({ children }) => (
    <strong className="font-bold" style={{ color: "var(--mk-accent-600)" }}>
      {children}
    </strong>
  ),
  em: ({ children }) => <em className="italic">{children}</em>,
  ul: ({ children }) => <ul className="mb-2 list-disc pl-5">{children}</ul>,
  ol: ({ children }) => <ol className="mb-2 list-decimal pl-5">{children}</ol>,
  li: ({ children }) => <li className="my-0.5">{children}</li>,
  h1: ({ children }) => <p className="mb-2 text-[15.5px] font-bold text-mk-ink">{children}</p>,
  h2: ({ children }) => <p className="mb-2 text-[15px] font-bold text-mk-ink">{children}</p>,
  h3: ({ children }) => <p className="mb-2 text-[14.5px] font-bold text-mk-ink">{children}</p>,
  blockquote: ({ children }) => (
    <blockquote className="mb-2 border-l-[3px] border-mk-border pl-3 text-mk-muted">{children}</blockquote>
  ),
  code: ({ children }) => (
    <code className="rounded bg-mk-paper px-1.5 py-0.5 font-mono text-[13px]">{children}</code>
  ),
  pre: ({ children }) => <pre className="mb-2 overflow-x-auto rounded-mk-sm">{children}</pre>,
  a: ({ children, href }) => (
    <a href={href} target="_blank" rel="noopener noreferrer" className="text-mk-accent underline">
      {children}
    </a>
  ),
};

export function LiteChatMarkdown({ text }: { text: string }) {
  return (
    <div className="-mb-2 text-mk-body text-mk-ink">
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkCjkFriendly]} components={components}>
        {text}
      </ReactMarkdown>
    </div>
  );
}
