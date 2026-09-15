// teacher/rubricLogic.ts — the 评分标准 section's draft, checks and request
// body. `validateRubricDraft`'s messages mirror liteassign.ValidateRubric's
// exactly, so a teacher reads the same sentence whether the browser or the
// server catches the problem — that is a deliberate mirror of server *error
// text*, not of server *data*, and is unrelated to the constraint below.
//
// What this file does NOT do: keep a frontend copy of
// `liteassign.DefaultRubric`'s zh/en dimension names and notes. Task 9 hit
// the same line (api/gradings.ts's `FALLBACK_RUBRIC`) — the server always
// bakes the effective rubric into a real writing payload, so a frontend
// default is both dead weight and a second source of truth waiting to drift
// from the Go one.
//
// Controller ruling 2026-09-15 (fix round 1): there is no rubric editor on a
// brand-new homework's create form at all — a new writing homework's create
// payload simply never carries `rubric` (assignmentLogic.ts's `buildPayload`
// leaves it out unconditionally), and the server applies its own default for
// the homework's `lang`. `rubricDefaultNote` is the create form's stand-in
// for a real editor. A rubric only ever exists as an editable draft once a
// homework exists and its detail page is opened — seeded straight from the
// server's effective rubric (`rubricDraftFromPayload`), never guessed.

import { normalizeRubric, type Rubric, type RubricDimension } from "../api/gradings";

export interface RubricDraft {
  scale: "letter" | "points";
  /** Raw input; only read when scale is points. */
  max: string;
  dimensions: RubricDimension[];
  focus: string;
}

export function rubricDraftOf(r: Rubric): RubricDraft {
  return { scale: r.scale, max: r.scale === "points" ? String(r.max ?? "") : "", dimensions: r.dimensions.map((d) => ({ ...d })), focus: r.focus };
}

/** Structural equality, field by field — not `JSON.stringify` comparison,
 *  which would treat two dimensions built with the same `name`/`note` in a
 *  different key order as different drafts. `max` is compared only when
 *  `scale` is "points" (its only meaning), so stale leftover text from a
 *  scale that was since switched away from never reads as a real change. */
export function sameRubricDraft(a: RubricDraft, b: RubricDraft): boolean {
  if (a.scale !== b.scale) return false;
  if (a.scale === "points" && a.max !== b.max) return false;
  if (a.focus !== b.focus) return false;
  if (a.dimensions.length !== b.dimensions.length) return false;
  return a.dimensions.every((d, i) => d.name === b.dimensions[i]!.name && d.note === b.dimensions[i]!.note);
}

/** A stored writing homework's rubric draft — the server always bakes the
 *  effective rubric (its own stored one, or its own default for the
 *  homework's language) into every writing payload it returns, so this reads
 *  straight off `payload.rubric`. `null` only if the payload carries none at
 *  all (should not happen for a real writing homework; there is nothing
 *  honest to show in its place, so callers must handle "no rubric loaded
 *  yet" rather than being handed a guessed placeholder). */
export function rubricDraftFromPayload(payload: Record<string, unknown>): RubricDraft | null {
  const r = normalizeRubric(payload.rubric);
  return r ? rubricDraftOf(r) : null;
}

/** The create form's note in place of a rubric editor — a new writing
 *  homework always uses the server's own default for `lang`, which this
 *  frontend keeps no copy of, so the note names the language, not the
 *  actual dimension text. */
export function rubricDefaultNote(lang: "zh" | "en"): string {
  return lang === "en"
    ? "评分标准：使用英文默认标准，创建后可在作业详情中调整"
    : "评分标准：使用中文默认标准，创建后可在作业详情中调整";
}

const runes = (s: string): number => [...s].length;

export function validateRubricDraft(d: RubricDraft): string | null {
  if (d.scale === "points") {
    const max = /^\d+$/.test(d.max.trim()) ? Number(d.max.trim()) : NaN;
    if (!(max >= 1 && max <= 100)) return "满分需在 1 到 100 之间";
  }
  if (d.dimensions.length < 1 || d.dimensions.length > 6) return "评分维度需有 1 到 6 项";
  const seen = new Set<string>();
  for (const dim of d.dimensions) {
    const name = dim.name.trim();
    if (name === "" || runes(name) > 40) return "维度名称不能为空，不超过 40 字";
    if (seen.has(name)) return "维度名称不能重复";
    seen.add(name);
    if (runes(dim.note.trim()) > 200) return "维度说明不超过 200 字";
  }
  if (runes(d.focus.trim()) > 500) return "批改重点不超过 500 字";
  return null;
}

/** The rubric for a draft that passed validateRubricDraft. */
export function buildRubric(d: RubricDraft): Rubric {
  const dimensions = d.dimensions.map((x) => ({ name: x.name.trim(), note: x.note.trim() }));
  return d.scale === "points"
    ? { scale: "points", max: Number(d.max.trim()), dimensions, focus: d.focus.trim() }
    : { scale: "letter", dimensions, focus: d.focus.trim() };
}
