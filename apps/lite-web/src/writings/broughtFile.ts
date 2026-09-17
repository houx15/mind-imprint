/**
 * An uploaded file → the two boxes of 带一篇写好的进来.
 *
 * 🚨 2026-09-18 写作入口走查：文件的第一行是题目，它同时出现在「题目」框和
 * 正文的第一段。通篇审阅于是把题目当成第一段来读，成稿和老师看到的正文开头
 * 也多一行题目。题目认出来了，就从正文里拿掉那一行。
 *
 * No title from the file → the file name, with separators turned into spaces
 * (「en-toefl-groups」 reads as a slug, not a name).
 */
export function splitBroughtFile(
  extracted: { title: string; text: string },
  fileName: string,
): { title: string; body: string } {
  const text = extracted.text.replace(/^\s+/, "");
  const title = extracted.title.trim();
  if (title) {
    const nl = text.indexOf("\n");
    const first = (nl < 0 ? text : text.slice(0, nl)).replace(/^#+/, "").trim();
    const rest = nl < 0 ? "" : text.slice(nl + 1).trim();
    if (first === title && rest) return { title, body: rest };
    return { title, body: text.trim() };
  }
  const stem = fileName.replace(/\.[^.]+$/, "").replace(/[-_]+/g, " ").trim();
  return { title: stem, body: text.trim() };
}
