export interface PrintPanel { title: string; body: string }
export interface PrintLayout { format: "a4-accordion-six"; panels: PrintPanel[] }

export function readPrintLayout(value: unknown): PrintLayout | null {
  if (!value || typeof value !== "object") return null;
  const p = value as Partial<PrintLayout>;
  if (p.format !== "a4-accordion-six" || !Array.isArray(p.panels) || p.panels.length !== 6) return null;
  if (!p.panels.every(x => x && typeof x.title === "string" && x.title.trim() && typeof x.body === "string" && x.body.trim())) return null;
  return p as PrintLayout;
}

const escape = (s: string) => s.replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[c]!);

/** Physical dimensions, not six flowing document sections. Text is never clipped. */
export function accordionHTML(layout: PrintLayout, renderMarkdown: (body: string) => string): string {
  return `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>折页打印版</title><style>
  *{box-sizing:border-box}body{margin:0;background:white;color:black;font-family:system-ui,sans-serif}
  .sheet{width:297mm;height:210mm;display:grid;grid-template-columns:repeat(6,1fr)}
  .panel{position:relative;min-width:0;padding:7mm 4mm;border-right:1px dashed #888;height:210mm}
  .panel:last-child{border-right:0}.content{height:190mm;font-size:10pt;line-height:1.5;overflow-wrap:anywhere}
  h1{font-size:14pt;line-height:1.3;margin:0 0 5mm}h2,h3,h4{font-size:11pt;margin:3mm 0}p{margin:2mm 0}ul,ol{padding-left:4mm}li{margin:1mm 0}table{border-collapse:collapse;width:100%}td,th{border:1px solid;padding:1mm}pre{white-space:pre-wrap;font:inherit}
  .number{position:absolute;bottom:4mm;left:4mm;font-size:8pt}.guide{padding:8mm;font-size:11pt;max-width:297mm}
  @page{size:A4 landscape;margin:0}@media print{.guide{display:none}.sheet{break-after:avoid}}
  </style><body><div class="sheet">${layout.panels.map((p,i) => `<section class="panel"><div class="content"><h1>${escape(p.title)}</h1>${renderMarkdown(p.body)}</div><div class="number">${i+1} / 6${i===0 ? " · 封面" : ""}</div></section>`).join("")}</div>
  <p class="guide">A4横向，单面打印，实际大小100%，关闭页眉页脚。背面留白。沿五条虚线交替向前、向后折，第一面作封面；展开后从左向右查找。请先试印一张检查边缘和文字，页面内容超出分面时请缩短内容后重做。</p></body></html>`;
}
