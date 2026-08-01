// #5-follow-on · a section-by-section VIEW over the draft. The draft is still a
// single Markdown string on the wire (export / 体检 / preview all consume it
// unchanged) — this only parses it into editable sections and serializes back,
// so the student can write under outline-driven headings without the draft
// becoming a structured document (铁律②: we organize her structure, we don't
// author or lay out her deliverable). It's a toggle; free-form text is always
// available.

export type DraftSection = { id: string; level: number; heading: string; body: string };

let sSeq = 0;
const sid = () => `sec-${sSeq++}`;

const HEADING = /^(#{1,3})\s+(.*)$/;

// parseSections splits Markdown into sections by #/##/### headings. Text before
// the first heading becomes an untitled intro section (level 0). Returns [] for
// empty input.
export function parseSections(md: string): DraftSection[] {
  const src = md ?? "";
  if (src.trim() === "") return [];
  const sections: DraftSection[] = [];
  let cur: DraftSection | null = null;
  let bodyLines: string[] = [];
  const flush = () => {
    if (cur) {
      cur.body = bodyLines.join("\n").trim();
      sections.push(cur);
    }
    bodyLines = [];
  };
  for (const line of src.split("\n")) {
    const m = HEADING.exec(line);
    if (m) {
      flush();
      cur = { id: sid(), level: m[1]!.length, heading: m[2]!.trim(), body: "" };
    } else {
      if (!cur) cur = { id: sid(), level: 0, heading: "", body: "" };
      bodyLines.push(line);
    }
  }
  flush();
  return sections;
}

// serializeSections renders sections back to a Markdown string: a heading line
// (clamped 1–3 #) then a blank line then the body; a level-0 intro is just its
// body. Empty sections contribute nothing (a lone empty intro yields "").
export function serializeSections(sections: DraftSection[]): string {
  const chunks: string[] = [];
  for (const s of sections) {
    const body = s.body.trim();
    if (s.level === 0) {
      if (body) chunks.push(body);
      continue;
    }
    const hashes = "#".repeat(Math.max(1, Math.min(s.level, 3)));
    const heading = s.heading.trim();
    if (!heading && !body) continue; // a wholly empty section drops out
    chunks.push(body ? `${hashes} ${heading}\n\n${body}` : `${hashes} ${heading}`);
  }
  return chunks.join("\n\n");
}

// sectionsFromOutline builds level-1 sections from top-level outline headings,
// skipping any whose heading a current section already carries (case/space-
// insensitive) so "generate from outline" is additive, never duplicating.
export function sectionsFromOutline(headings: string[], existing: DraftSection[]): DraftSection[] {
  const have = new Set(existing.map((s) => s.heading.trim().toLowerCase()).filter(Boolean));
  const out: DraftSection[] = [];
  for (const h of headings) {
    const t = h.trim();
    if (!t || have.has(t.toLowerCase())) continue;
    have.add(t.toLowerCase());
    out.push({ id: sid(), level: 1, heading: t, body: "" });
  }
  return out;
}

export const newSection = (level = 1): DraftSection => ({ id: sid(), level, heading: "", body: "" });
