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
// from the Go one. `UNSET_RUBRIC_DRAFT` below is this file's equivalent: a
// generic, non-localized placeholder, never the real default text.

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

/**
 * Stands in for "no rubric yet" — the draft a brand-new writing homework's
 * form starts from, before any real rubric exists on the server, and the
 * fallback for a payload that carries none (should not happen for a real
 * writing homework; the server always bakes one in). A single generic
 * dimension, not `liteassign.DefaultRubric`'s actual zh/en text — see this
 * file's header. `buildPayload`/`buildPatchInput` (assignmentLogic.ts) read
 * a draft that still equals this as "the teacher hasn't customized it" and
 * leave `rubric` out of the request, so the server applies its own default
 * for the homework's language instead of this placeholder ever being sent.
 */
export const UNSET_RUBRIC_DRAFT: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "总评", note: "" }], focus: "" };

/** Structural equality — a `RubricDraft` is plain data, so this is enough to
 *  tell "the teacher touched it" from "still what it was loaded/initialized
 *  as", without caring which fields changed. */
export function sameRubricDraft(a: RubricDraft, b: RubricDraft): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/** A stored writing homework's rubric draft. The server always bakes the
 *  effective rubric (its own stored one, or its own default for the
 *  homework's language) into every writing payload it returns, so this reads
 *  straight off `payload.rubric` — `UNSET_RUBRIC_DRAFT` only if the payload
 *  carries none at all. */
export function rubricDraftFromPayload(payload: Record<string, unknown>): RubricDraft {
  const r = normalizeRubric(payload.rubric);
  return r ? rubricDraftOf(r) : UNSET_RUBRIC_DRAFT;
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

/**
 * Controller ruling 2026-09-15: a language switch never rewrites the rubric
 * text. This frontend keeps no copy of the Go default's per-language
 * dimension names (see this file's header), so it cannot honestly guess
 * what the other language's default would say — inventing text the server
 * never produced would be worse than leaving the draft alone. The draft —
 * whether it is still `UNSET_RUBRIC_DRAFT`, a rubric loaded from the server,
 * or something the teacher wrote — passes through unchanged either way.
 */
export function rubricAfterLangChange(d: RubricDraft, from: "zh" | "en", to: "zh" | "en"): RubricDraft {
  return from === to ? d : d;
}
