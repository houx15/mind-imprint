import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { splitByMarks, type ReviewMark } from "../../../api/review";
import { DONE, TODO } from "../../../shared/tone";

interface TextTree {
  type: string;
  value?: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: TextTree[];
}

/** Format first, then annotate text nodes; Markdown syntax never becomes a quote.
 * Quotes spanning formatting nodes remain available in the adjacent MarkRow.
 */
export function ReviewMarkdown({ text, marks, onOpen }: {
  text: string;
  marks: ReviewMark[];
  onOpen: (id: string) => void;
}) {
  const highlight = () => (tree: unknown) => {
    function walk(node: TextTree) {
      if (!node.children || node.tagName === "code" || node.tagName === "pre") return;
      node.children = node.children.flatMap((child) => {
        if (child.type !== "text" || !child.value) {
          walk(child);
          return [child];
        }
        return splitByMarks(child.value, marks).map((segment): TextTree =>
          segment.mark ? {
            type: "element", tagName: "mark",
            properties: { dataReviewMark: segment.mark.id },
            children: [{ type: "text", value: segment.text }],
          } : { type: "text", value: segment.text },
        );
      });
    }
    walk(tree as TextTree);
  };
  return <div className="space-y-3 leading-relaxed text-mk-ink [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:font-semibold [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-1 [&_p]:whitespace-pre-wrap [&_table]:w-full [&_table]:border-collapse [&_th]:border [&_th]:border-mk-border [&_th]:bg-mk-paper [&_th]:p-3 [&_th]:text-left [&_td]:border [&_td]:border-mk-border [&_td]:p-3 [&_td]:min-w-20 [&_td]:h-14 [&_blockquote]:border-l-2 [&_blockquote]:pl-3">
    <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[highlight]} skipHtml components={{
      table: ({ children }) => <div className="overflow-x-auto"><table>{children}</table></div>,
      img: ({ alt }) => <span>[图片：{alt || "未提供说明"}]</span>,
      a: ({ children, href }) => <a href={href} target="_blank" rel="noreferrer" className="underline">{children}</a>,
      mark: ({ node, children }) => {
        const id = String(node?.properties.dataReviewMark ?? "");
        const index = marks.findIndex((m) => m.id === id);
        const mark = marks[index];
        if (!mark) return <>{children}</>;
        const state = mark.answer.trim() ? DONE : TODO;
        return <mark onClick={() => onOpen(id)} className="cursor-pointer rounded-mk-sm px-0.5"
          style={{ background: state.bg, color: "var(--mk-ink)", boxShadow: `inset 0 -2px 0 ${state.solid}` }}>
          {children}<sup className="ml-0.5 rounded-mk-full px-1 text-[10px] font-semibold"
            style={{ background: state.solid, color: "var(--mk-surface)" }}>{index + 1}</sup>
        </mark>;
      },
    }}>{text}</ReactMarkdown>
  </div>;
}
