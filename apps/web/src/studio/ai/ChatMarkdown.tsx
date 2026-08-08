import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Components } from "react-markdown";

/**
 * ChatMarkdown — the ONE shared markdown renderer for 印记's chat bubbles.
 *
 * Replaces four duplicated hand-rolled `renderRich` copies (StudioCoachChat /
 * PlanBlock / WritingBlock / ReviewBlock) that only ever handled `**bold**` —
 * the coach routinely returns lists, links, inline code and headings too,
 * none of which rendered before (they showed as raw `- `/`[text](url)`
 * markup). `apps/web/src/cards/Markdown.tsx` already does this for card
 * bodies; this is the chat-bubble-scale sibling — tight block spacing (the
 * wrapper's `-mb-2` cancels the last block's own `mb-2` so bubble padding
 * stays the only whitespace), `text-mk-body` (14px, satisfies the ≥14px
 * floor), links open in a new tab.
 *
 * AI-role messages ONLY — student turns stay plain `whitespace-pre-wrap`
 * text in every caller (a student typing a literal `*` or `#` must never be
 * silently swallowed as markup).
 */
const components: Components = {
  p: ({ children }) => <p className="mb-2">{children}</p>,
  strong: ({ children }) => <strong className="font-bold text-mk-ink">{children}</strong>,
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

export function ChatMarkdown({ text }: { text: string }) {
  return (
    <div className="-mb-2 text-mk-body text-mk-ink">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {text}
      </ReactMarkdown>
    </div>
  );
}
