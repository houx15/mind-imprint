import { marked } from "marked";

export interface ArtifactSlide { title: string; markdown: string }

// Use Markdown block tokens so headings inside code, lists, or quotations do
// not accidentally become slides. Source slices retain formatting verbatim.
export function artifactSlides(body: string): ArtifactSlide[] {
  body = body.replace(/\r\n?/g, "\n");
  const tokens = marked.lexer(body);
  const depth = tokens.filter(t => t.type === "heading" && t.depth === 2).length >= 2 ? 2 : 1;
  const breaks: { start: number; end: number; title: string }[] = [];
  let cursor = 0;
  for (const token of tokens) {
    const at = body.indexOf(token.raw, cursor);
    if (at < 0) continue;
    cursor = at + token.raw.length;
    if (token.type === "heading" && token.depth === depth) breaks.push({ start: at, end: cursor, title: token.text });
  }
  if (breaks.length < 2) return [];
  const references = Object.entries(tokens.links).map(([label, link]) =>
    `[${label}]: <${link.href}>${link.title ? ` "${link.title.replace(/"/g, '\\"')}"` : ""}`
  ).join("\n");
  const slides: ArtifactSlide[] = [];
  const intro = body.slice(0, breaks[0]!.start).trim();
  if (intro) slides.push({ title: "项目介绍", markdown: intro });
  breaks.forEach((item, index) => slides.push({
    title: item.title,
    markdown: body.slice(item.start, breaks[index + 1]?.start ?? body.length).trim(),
  }));
  return slides.map(slide => ({ ...slide, markdown: `${slide.markdown}\n\n${references}`.trim() }));
}
