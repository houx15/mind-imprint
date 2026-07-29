import { useEffect, useState } from "react";

// The Write room's draft preview. `marked` is **dynamically imported** (mirrors
// how workspace/export dynamic-imports xlsx/docx) so the markdown renderer lands
// in its own lazy chunk — never the entry bundle. marked emits raw HTML, so the
// output is run through a small DOM-based sanitizer before it's injected.

// Strip anything script-like out of marked's HTML. The draft is the student's
// own text (self-XSS at worst), but we still drop active content: script/style/
// frame/embed elements, on* handlers, and javascript: URLs. DOMParser exists in
// both the browser and jsdom; if it's somehow missing we fall back to escaping.
function sanitizeHtml(html: string): string {
  if (typeof DOMParser === "undefined") {
    return html.replace(/[<>]/g, (c) => (c === "<" ? "&lt;" : "&gt;"));
  }
  const doc = new DOMParser().parseFromString(html, "text/html");
  doc.querySelectorAll("script, style, iframe, object, embed, link, meta, form").forEach((el) => el.remove());
  doc.querySelectorAll("*").forEach((el) => {
    for (const attr of Array.from(el.attributes)) {
      const name = attr.name.toLowerCase();
      const value = attr.value.trim().toLowerCase();
      if (name.startsWith("on")) el.removeAttribute(attr.name);
      else if ((name === "href" || name === "src") && value.startsWith("javascript:")) el.removeAttribute(attr.name);
    }
  });
  return doc.body.innerHTML;
}

// Scoped typography for the rendered draft — kept local to this component (no
// global CSS touched) and namespaced under `.mi-md-preview` so it can't leak.
const PREVIEW_CSS = `
.mi-md-preview { color: #1C2333; font-size: 14.5px; line-height: 1.75; }
.mi-md-preview > :first-child { margin-top: 0; }
.mi-md-preview h1 { font-size: 22px; font-weight: 700; margin: 1.2em 0 0.5em; }
.mi-md-preview h2 { font-size: 18px; font-weight: 700; margin: 1.1em 0 0.5em; }
.mi-md-preview h3 { font-size: 15.5px; font-weight: 700; margin: 1em 0 0.4em; }
.mi-md-preview p { margin: 0 0 0.85em; }
.mi-md-preview ul { margin: 0 0 0.85em; padding-left: 1.4em; list-style: disc; }
.mi-md-preview ol { margin: 0 0 0.85em; padding-left: 1.4em; list-style: decimal; }
.mi-md-preview li { margin: 0.2em 0; }
.mi-md-preview a { color: #2A3B7A; text-decoration: underline; }
.mi-md-preview strong { font-weight: 700; }
.mi-md-preview em { font-style: italic; }
.mi-md-preview blockquote { margin: 0 0 0.85em; padding: 0.1em 0 0.1em 0.9em; border-left: 3px solid #D6DBEA; color: #5B6373; }
.mi-md-preview code { background: #F0F1F5; border-radius: 4px; padding: 1px 5px; font-size: 13px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.mi-md-preview pre { background: #F0F1F5; border-radius: 8px; padding: 12px 14px; overflow-x: auto; margin: 0 0 0.85em; }
.mi-md-preview pre code { background: none; padding: 0; }
.mi-md-preview hr { border: none; border-top: 1px solid #E4E7F0; margin: 1.4em 0; }
.mi-md-preview table { border-collapse: collapse; margin: 0 0 0.85em; font-size: 13.5px; }
.mi-md-preview th, .mi-md-preview td { border: 1px solid #E4E7F0; padding: 5px 9px; text-align: left; }
.mi-md-preview th { background: #F6F7FA; font-weight: 700; }
.mi-md-preview img { max-width: 100%; }
`;

export function MarkdownPreview({ text }: { text: string }) {
  const [html, setHtml] = useState<string>("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const { marked } = await import("marked");
      const raw = await marked.parse(text, { gfm: true, breaks: true });
      if (!cancelled) setHtml(sanitizeHtml(raw));
    })();
    return () => {
      cancelled = true;
    };
  }, [text]);

  if (!text.trim()) {
    return (
      <div className="min-h-0 flex-1 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-5">
        <p className="text-[13.5px] text-mk-muted-2">还没有内容——切回「写」开始你的草稿，这里会实时渲染。</p>
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-5">
      <style>{PREVIEW_CSS}</style>
      <div className="mi-md-preview" dangerouslySetInnerHTML={{ __html: html }} />
    </div>
  );
}
