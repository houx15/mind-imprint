import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Components } from "react-markdown";

// Inline-styled renderers so AI replies keep the chat bubble's typography
// (the coach prompt actively asks the model to use **bold**, lists and `>`
// quotes, so these must render rather than show as raw markup).
const components: Components = {
  p: ({ children }) => <p style={{ margin: "0 0 8px" }}>{children}</p>,
  strong: ({ children }) => <strong style={{ fontWeight: 700, color: "#1C2333" }}>{children}</strong>,
  em: ({ children }) => <em>{children}</em>,
  ul: ({ children }) => (
    <ul style={{ margin: "0 0 8px", paddingLeft: "20px", listStyle: "disc" }}>{children}</ul>
  ),
  ol: ({ children }) => (
    <ol style={{ margin: "0 0 8px", paddingLeft: "20px", listStyle: "decimal" }}>{children}</ol>
  ),
  li: ({ children }) => <li style={{ margin: "2px 0" }}>{children}</li>,
  h1: ({ children }) => <div style={{ fontWeight: 700, fontSize: "15.5px", margin: "0 0 6px", color: "#1C2333" }}>{children}</div>,
  h2: ({ children }) => <div style={{ fontWeight: 700, fontSize: "15px", margin: "0 0 6px", color: "#1C2333" }}>{children}</div>,
  h3: ({ children }) => <div style={{ fontWeight: 700, fontSize: "14.5px", margin: "0 0 6px", color: "#1C2333" }}>{children}</div>,
  blockquote: ({ children }) => (
    <blockquote
      style={{
        margin: "0 0 8px",
        paddingLeft: "12px",
        borderLeft: "3px solid #D6DBEA",
        color: "#5B6373",
      }}
    >
      {children}
    </blockquote>
  ),
  code: ({ children }) => (
    <code
      style={{
        background: "#F0F1F5",
        borderRadius: "4px",
        padding: "1px 5px",
        fontSize: "13px",
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
      }}
    >
      {children}
    </code>
  ),
  a: ({ children, href }) => (
    <a href={href} target="_blank" rel="noreferrer" style={{ color: "#2A3B7A", textDecoration: "underline" }}>
      {children}
    </a>
  ),
};

export function Markdown({ text }: { text: string }) {
  return (
    // Strip the trailing margin from the last block so the bubble padding stays even.
    <div style={{ marginBottom: "-8px" }}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {text}
      </ReactMarkdown>
    </div>
  );
}

// Same markdown, but for single-line contexts (a heading, a modal title) that
// already sit inside their own block element (a `<div>`/`<h2>`) — dropping the
// `<p>` wrapper avoids nesting a block inside an inline title and keeps the
// caller's own typography/margins intact. No list/quote handling here; the
// callers this is meant for are short one-line strings.
const inlineComponents: Components = { ...components, p: ({ children }) => <>{children}</> };

export function MarkdownInline({ text }: { text: string }) {
  return (
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={inlineComponents}>
      {text}
    </ReactMarkdown>
  );
}
