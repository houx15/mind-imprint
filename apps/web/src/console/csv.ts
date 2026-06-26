export interface ParsedRow {
  class: string;
  teacher_email?: string;
  student_email?: string;
}

// parseCsv: the first non-blank line is a header mapping the columns `class` /
// `teacher_email` / `student_email` (case-insensitive, any order). Cells are
// trimmed; double-quoted fields may contain commas. Blank lines are skipped.
// Throws on an empty file or a missing `class` column. Limitation: no escaped
// quote ("") handling — out of scope for a roster sheet; a malformed file
// surfaces as a thrown error, never a silent mis-parse.
export function parseCsv(text: string): ParsedRow[] {
  const lines = text.split(/\r?\n/).filter((l) => l.trim() !== "");
  if (lines.length === 0) throw new Error("空文件");
  const header = splitCsvLine(lines[0]!).map((h) => h.trim().toLowerCase());
  const ci = header.indexOf("class");
  if (ci === -1) throw new Error("缺少 class 列");
  const ti = header.indexOf("teacher_email");
  const si = header.indexOf("student_email");
  const rows: ParsedRow[] = [];
  for (let i = 1; i < lines.length; i++) {
    const cells = splitCsvLine(lines[i]!);
    const row: ParsedRow = { class: (cells[ci] ?? "").trim() };
    if (ti !== -1) {
      const v = (cells[ti] ?? "").trim();
      if (v) row.teacher_email = v;
    }
    if (si !== -1) {
      const v = (cells[si] ?? "").trim();
      if (v) row.student_email = v;
    }
    rows.push(row);
  }
  return rows;
}

function splitCsvLine(line: string): string[] {
  const out: string[] = [];
  let cur = "";
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === '"') {
      inQuotes = !inQuotes;
      continue;
    }
    if (ch === "," && !inQuotes) {
      out.push(cur);
      cur = "";
      continue;
    }
    cur += ch;
  }
  out.push(cur);
  return out;
}
