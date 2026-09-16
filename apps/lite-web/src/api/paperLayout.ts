export interface PaperElement {
  kind: "rect" | "ellipse" | "line" | "text";
  x: number; y: number; width: number; height: number;
  text?: string; fontSize?: number; fill?: "none" | "white" | "light" | "dark"; dashed?: boolean;
}
export interface PaperLayout { format: "a4-portrait"; title: string; notice: string; elements: PaperElement[] }

export function readPaperLayout(value: unknown): PaperLayout | null {
  if (!value || typeof value !== "object") return null;
  const p = value as PaperLayout;
  if (p.format !== "a4-portrait" || typeof p.title !== "string" || typeof p.notice !== "string" || !Array.isArray(p.elements) || !p.elements.length || p.elements.length > 120) return null;
  if (!p.elements.every(e => e && (e.fill === undefined || ["none","white","light","dark"].includes(e.fill)) && (e.dashed === undefined || typeof e.dashed === "boolean") && ["rect", "ellipse", "line", "text"].includes(e.kind) && [e.x,e.y,e.width,e.height].every(Number.isFinite) && e.x >= 12 && e.y >= 35 && e.width >= 0 && e.height >= 0 && e.x+e.width <= 198 && e.y+e.height <= 277 && (e.kind !== "text" || (typeof e.text === "string" && Number.isFinite(e.fontSize) && e.fontSize! >= 3 && e.fontSize! <= 8)))) return null;
  return p;
}

const escape = (s: string) => s.replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[c]!);
export function paperSVG(layout: PaperLayout): string {
  if (!readPaperLayout(layout)) throw new Error("纸面布局无效");
  // Text is always foreground; model array order does not control legibility.
  const ordered = [...layout.elements.filter(e => e.kind !== "text"), ...layout.elements.filter(e => e.kind === "text")];
  const shapes = ordered.map(e => {
    const fill = {none:"none",white:"#fff",light:"#f0f0ec",dark:"#273735"}[e.fill ?? "none"] ?? "none";
    const style = `fill="${fill}" stroke="#273735" stroke-width="0.4"${e.dashed ? ' stroke-dasharray="2 1.5"' : ""}`;
    switch (e.kind) {
      case "rect": return `<rect x="${e.x}" y="${e.y}" width="${e.width}" height="${e.height}" ${style}/>`;
      case "ellipse": return `<ellipse cx="${e.x+e.width/2}" cy="${e.y+e.height/2}" rx="${e.width/2}" ry="${e.height/2}" ${style}/>`;
      case "line": return `<line x1="${e.x}" y1="${e.y}" x2="${e.x+e.width}" y2="${e.y+e.height}" stroke="#273735" stroke-width="0.4"${e.dashed ? ' stroke-dasharray="2 1.5"' : ""}/>`;
      case "text": return `<text x="${e.x}" y="${e.y+e.fontSize!}" font-size="${e.fontSize}" fill="#273735" stroke="white" stroke-width="0.6" paint-order="stroke fill">${escape(e.text!)}</text>`;
    }
  }).join("");
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 210 297" role="img" aria-label="${escape(layout.title)}"><rect width="210" height="297" fill="white"/><g font-family="Noto Sans CJK SC,Microsoft YaHei,sans-serif"><text x="12" y="19" font-size="6" font-weight="700" fill="#273735">${escape(layout.title)}</text><text x="12" y="28" font-size="3" fill="#555">${escape(layout.notice)}</text><g id="paper-elements">${shapes}</g><line x1="12" y1="283" x2="198" y2="283" stroke="#aaa" stroke-width="0.2"/><text x="12" y="289" font-size="2.8" fill="#555">A4 · 图形为纸面示意，非实物尺寸</text></g></svg>`;
}

export function paperHTML(layout: PaperLayout): string {
  return `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>${escape(layout.title)}</title><style>*{box-sizing:border-box}html,body{margin:0;background:#fff}.sheet{width:210mm;height:297mm}.sheet svg{display:block;width:100%;height:100%;overflow:visible}@page{size:A4 portrait;margin:0}@media print{.sheet{break-inside:avoid;break-after:avoid}}</style><body><main class="sheet">${paperSVG(layout)}</main></body></html>`;
}

export function downloadPaper(layout: PaperLayout) {
  const url = URL.createObjectURL(new Blob([paperHTML(layout)], {type:"text/html;charset=utf-8"}));
  const a = document.createElement("a"); a.href=url; a.download="纸面原型-A4.html"; a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
