/**
 * 正文里的 markdown 表格。
 *
 * 🚨 分级阅读库里那些论文常常带表格（产品负责人 2026-09-12：「some paper would
 * have a table with markdown format but we didn't load that」）。正文原来是按
 * 纯文本渲染的，于是她看到的是一屏竖线和横杠。
 *
 * 表格**不走 `<p data-block-id>` 那条路**：标注的锚点是这一段文本里的字节偏移，
 * 把一段文字拆成一堆单元格，偏移就对不上了。所以表格块单独认出来、单独画，
 * 其余的段落一个字都不变 —— 现有那一大批 `p[data-block-id]` 选择器照常工作。
 */

export type MarkdownTable = { header: string[]; rows: string[][] };

/** 一行 `| a | b |` 拆成单元格。 */
function cells(line: string): string[] {
  let s = line.trim();
  if (s.startsWith("|")) s = s.slice(1);
  if (s.endsWith("|")) s = s.slice(0, -1);
  return s.split("|").map((c) => c.trim());
}

/** `|---|:--:|` 这种分隔行。 */
function isDivider(line: string): boolean {
  const c = cells(line);
  return c.length > 0 && c.every((x) => /^:?-{2,}:?$/.test(x));
}

/**
 * 这一块是不是一张 markdown 表格。不是就返回 null，调用方照常按段落渲染。
 *
 * 判据要紧一点：至少三行（表头、分隔、一行数据），每行都以竖线起手。松一点的
 * 判据会把正文里偶然出现的一条竖线认成表格。
 */
export function parseMarkdownTable(text: string): MarkdownTable | null {
  const lines = text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l.length > 0);
  if (lines.length < 3) return null;
  if (!lines.every((l) => l.startsWith("|"))) return null;
  if (!isDivider(lines[1]!)) return null;

  const header = cells(lines[0]!);
  const rows: string[][] = [];
  for (const line of lines.slice(2)) {
    if (isDivider(line)) continue;
    rows.push(cells(line));
  }
  if (rows.length === 0) return null;
  return { header, rows };
}
