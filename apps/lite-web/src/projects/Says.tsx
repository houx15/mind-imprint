import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

/** Older error receipts joined the reason and a Markdown excerpt without a line break. */
export function errorMarkdown(content: string): string {
  return content.replace(/^([^\r\n]*?：)(?=#{1,6}[^\S\r\n])/, "$1\n\n");
}

/** Accept existing Chinese list markers without altering code or decimals. */
export function normalizeSayMarkdown(content: string): string {
  let fence: { marker: string; length: number } | null = null;
  return content.split("\n").map((line) => {
    const delimiter = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(line);
    if (delimiter) {
      const marker = delimiter[1]!;
      if (!fence) fence = { marker: marker[0]!, length: marker.length };
      else if (marker[0] === fence.marker && marker.length >= fence.length && !delimiter[2]!.trim()) fence = null;
      return line;
    }
    if (fence || /^(?: {4}|\t)/.test(line)) return line;
    return line.replace(/^( {0,3})[·•]\s+/, "$1- ").replace(/^( {0,3})(\d+)、\s*/, "$1$2. ")
      // A closing delimiter after punctuation and before Chinese prose is not
      // emphasis in CommonMark. Keep visible punctuation outside the label.
      .replace(/^( {0,3}(?:[-*+] |\d+\. )?)\*\*([^*`\n]{1,200}?)([：。！？；，])\*\*(?=\S)/u, "$1**$2**$3");
  }).join("\n");
}

/** Render the student's and coach's actual formatting; never execute embedded HTML. */
export function Says({ content }: { content: string }) {
  return <div className="min-w-0 space-y-2 break-words [&_p]:whitespace-pre-wrap [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-1 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:font-semibold [&_h3]:font-semibold [&_blockquote]:border-l-2 [&_blockquote]:border-mk-border [&_blockquote]:pl-3">
    <ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml components={{
      a: ({ children, href }) => <a href={href} target="_blank" rel="noreferrer" className="underline">{children}</a>,
      img: ({ alt }) => <span>[图片：{alt || "未提供说明"}]</span>,
      pre: ({ children }) => <pre className="max-w-full overflow-x-auto rounded-mk-md bg-mk-paper p-3 whitespace-pre">{children}</pre>,
      table: ({ children }) => <div className="max-w-full overflow-x-auto"><table className="border-collapse [&_th]:border [&_td]:border [&_th]:border-mk-border [&_td]:border-mk-border [&_th]:p-2 [&_td]:p-2">{children}</table></div>,
    }}>{normalizeSayMarkdown(content)}</ReactMarkdown>
  </div>;
}
