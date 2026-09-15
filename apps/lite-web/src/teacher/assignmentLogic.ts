// teacher/assignmentLogic.ts — pure logic behind the teacher assignment pages:
// the form draft, its client-side checks, the request bodies built from it,
// and the small derivations the list/detail pages show. No React here, so
// every rule the pages depend on is testable without rendering.
//
// Client-side check messages are copied from the server
// (apps/api/internal/liteassign/payload.go and lite_teacher_assignments.go),
// so a teacher sees the same sentence whether the browser or the server
// catches the problem.

import { ApiError } from "../api/client";
import type {
  AssignmentKind,
  AssignmentPayload,
  CreateAssignmentInput,
  PatchAssignmentInput,
  PersonalPick,
  PreviewRow,
  RecipientDTO,
  RecipientReading,
  ReturnRecipientInput,
} from "../api/assignments";
import type { LibraryArticle, LibraryTag } from "../api/library";
import type { RosterRow } from "../api/teacher";
import { beijingInputToISO, type AssignmentStatus } from "../shared/deadline";
import { safeHttpUrl } from "./format";
import { buildRubric, rubricDraftFromPayload, sameRubricDraft, validateRubricDraft, type RubricDraft } from "./rubricLogic";

export type ReadingSource = "library" | "url" | "text" | "file" | "personalized";

/** One row of the personalized-reading picks table: the preview's
 * recommendation, or a swap the teacher made. */
export interface PickRow {
  userId: string;
  name: string;
  slug: string;
  title: string;
  /** The tier saved with the pick; null = her level at start. */
  tier: number | null;
  suggestedTier: number;
  reason: string;
  /** The teacher chose this article; a new preview keeps it. */
  swapped: boolean;
}

/** A swap, or a saved pick being edited, carried across a fresh preview run
 * (`mergePickRows`'s `kept` argument). */
export interface KeptPick {
  slug: string;
  title: string;
  tier: number | null;
  /** Where this came from: a value already on the server ("saved") or a
   * swap made in this editing session ("swapped"). `mergePickRows` only
   * treats a same-article "saved" pick as unswapped when just its tier
   * differs from the class-wide chip — a session swap is always shown as a
   * swap, regardless of slug. */
  origin: "saved" | "swapped";
}

/** The kind-specific part of an assignment — shared by the create form and
 * the detail page's settings edit. `targetWords` is the raw input string so
 * an empty field stays empty (a missing extracted count must not become 0). */
export interface SettingsDraft {
  kind: AssignmentKind;
  readingSource: ReadingSource;
  slug: string;
  tier: number | null;
  url: string;
  text: string;
  prompt: string;
  targetWords: string;
  lang: "zh" | "en";
  drivingQuestion: string;
  description: string;
  /** Writing only, and only once a homework exists: there is no rubric
   *  editor on the create form (`buildPayload` never sends one), so this is
   *  `null` there. Editing an existing writing homework seeds it from the
   *  server's effective rubric (`rubricDraftFromPayload`); editable after
   *  students start (sent as the PATCH's top-level `rubric`). */
  rubric: RubricDraft | null;
  /** Upload-tab only; set once a document has been extracted. */
  fileName: string;
  /** Personalized reading only: the class-wide discipline filter. */
  disciplines: string[];
  /** Personalized reading only: the class-wide tier override; null = each
   *  student's own level. */
  personalTier: number | null;
  /** Personalized reading only: the picks table, null while the preview has
   *  not loaded (or failed) — distinct from `[]`, an empty class. */
  picks: PickRow[] | null;
  /** Personalized reading only: the picks exactly as the server has them
   *  stored, read back by `settingsFromAssignment` — the fallback source
   *  when `picks` is null (see `buildPayload`). */
  savedPicks: Record<string, PersonalPick>;
  /** Whether the STORED payload (as loaded by `settingsFromAssignment`) was
   *  itself a personalized reading — false for a brand new homework and for
   *  a stored library/url/text homework the teacher is switching to
   *  personalized in this edit. Only a homework that was already
   *  personalized has real `savedPicks` to fall back to while a fresh
   *  preview is loading (see `buildPatchInput`'s wait-for-preview check). */
  storedPersonalized: boolean;
}

export interface AssignmentDraft extends SettingsDraft {
  classId: string;
  title: string;
  instructions: string;
  dueInput: string;
  userIds: string[];
}

export function emptySettings(kind: AssignmentKind = "reading"): SettingsDraft {
  return {
    kind,
    readingSource: "library",
    slug: "",
    tier: null,
    url: "",
    text: "",
    prompt: "",
    targetWords: "",
    lang: "zh",
    drivingQuestion: "",
    description: "",
    rubric: null,
    fileName: "",
    disciplines: [],
    personalTier: null,
    picks: null,
    savedPicks: {},
    storedPersonalized: false,
  };
}

const str = (v: unknown): string => (typeof v === "string" ? v : "");
const cutRunes = (s: string, n: number): string => [...s].slice(0, n).join("");

/** A stored personalized payload's `picks` → the wire-shaped record, dropping
 * any malformed entry rather than throwing on an old/hand-edited payload. */
function savedPicksOf(raw: unknown): Record<string, PersonalPick> {
  const out: Record<string, PersonalPick> = {};
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return out;
  for (const [uid, v] of Object.entries(raw as Record<string, unknown>)) {
    const p = v && typeof v === "object" ? (v as Record<string, unknown>) : {};
    if (typeof p.slug !== "string" || !p.slug) continue;
    out[uid] = { slug: p.slug, tier: typeof p.tier === "number" ? p.tier : null };
  }
  return out;
}

const titleOf = (a: LibraryArticle | undefined, slug: string): string => (a ? a.zhTitle || a.title : "") || slug;

/** A stored assignment's kind + payload → an editable draft. */
export function settingsFromAssignment(kind: AssignmentKind, payload: Record<string, unknown>): SettingsDraft {
  const d = emptySettings(kind);
  if (kind === "reading") {
    const source = payload.source;
    d.readingSource = source === "url" || source === "text" || source === "personalized" ? source : "library";
    // Frozen at load time: whether the STORED payload was personalized, not
    // whatever `readingSource` becomes if the teacher switches tabs in the
    // edit UI afterward.
    d.storedPersonalized = source === "personalized";
    d.slug = str(payload.slug);
    d.tier = typeof payload.tier === "number" ? payload.tier : null;
    d.url = str(payload.url);
    d.text = str(payload.text);
    d.fileName = str(payload.fileName);
    // A stored `text` source with a file name came from the upload tab —
    // reopen it there, not on the plain-paste tab.
    if (d.readingSource === "text" && d.fileName) d.readingSource = "file";
    if (d.readingSource === "personalized") {
      d.disciplines = Array.isArray(payload.disciplines) ? payload.disciplines.filter((x): x is string => typeof x === "string") : [];
      d.personalTier = d.tier;
      d.tier = null;
      d.savedPicks = savedPicksOf(payload.picks);
    }
  } else if (kind === "writing") {
    d.prompt = str(payload.prompt);
    d.targetWords = typeof payload.targetWords === "number" && payload.targetWords > 0 ? String(payload.targetWords) : "";
    d.lang = payload.lang === "en" ? "en" : "zh";
    d.rubric = rubricDraftFromPayload(payload);
  } else {
    d.drivingQuestion = str(payload.drivingQuestion);
    d.description = str(payload.description);
  }
  return d;
}

const runes = (s: string): number => [...s].length;

/** The target word count the teacher typed, or `null` when it is not a whole
 * number in the server's range. */
export function parseTargetWords(raw: string): number | null {
  const t = raw.trim();
  if (!/^\d+$/.test(t)) return null;
  const n = Number(t);
  return n >= 1 && n <= 100000 ? n : null;
}

/** Whether a URL passes the server's rule: http/https with a host. */
function isHttpUrlWithHost(raw: string): boolean {
  const safe = safeHttpUrl(raw.trim());
  if (!safe) return false;
  try {
    return new URL(safe).host !== "";
  } catch {
    return false;
  }
}

export function validateSettings(d: SettingsDraft): string | null {
  if (d.kind === "reading") {
    if (d.readingSource === "library") {
      if (!d.slug.trim()) return "请选择一篇文章";
      if (d.tier !== null && (d.tier < 1 || d.tier > 5)) return "难度档位需在 1 到 5 之间";
    } else if (d.readingSource === "url") {
      if (!isHttpUrlWithHost(d.url)) return "请输入以 http 或 https 开头的链接";
    } else if (d.readingSource === "personalized") {
      if (d.picks === null) return "请等待推荐列表加载完成";
    } else {
      // Ruling: the upload tab needs an actual extracted file, not just any
      // text — a stray body left over from the paste tab must not publish
      // silently as "uploaded" content.
      if (d.readingSource === "file" && !d.fileName.trim()) return "请上传文件";
      const text = d.text.trim();
      if (!text) return d.readingSource === "file" ? "请上传文件" : "请粘贴文章正文";
      if (runes(text) > 50000) return "文章正文不能超过 50000 字";
    }
    return null;
  }
  if (d.kind === "writing") {
    const prompt = d.prompt.trim();
    if (!prompt) return "请填写写作题目";
    if (runes(prompt) > 2000) return "写作题目不能超过 2000 字";
    if (parseTargetWords(d.targetWords) === null) return "目标字数需在 1 到 100000 之间";
    if (d.rubric) {
      const rubric = validateRubricDraft(d.rubric);
      if (rubric) return rubric;
    }
    return null;
  }
  const q = d.drivingQuestion.trim();
  if (!q) return "请填写驱动问题";
  if (runes(q) > 4000) return "驱动问题不能超过 4000 字";
  return null;
}

/** The payload for a draft that passed `validateSettings`. A library reading
 * with no chosen level leaves `tier` out, which the server reads as "use the
 * student's current level".
 *
 * `recipientIds` is a required parameter (the type system enforces it — fix
 * round 2: an earlier version made it optional with a runtime throw, which
 * the one production caller of `buildPatchInput` did not know to satisfy and
 * would have thrown uncaught on every save of a settings-editable
 * personalized homework). Only the `personalized` reading branch below
 * actually reads it; every other kind/source ignores it. For a
 * `personalized` reading, only those ids' picks are sent — a PATCH's
 * `payload` field, when present, replaces the stored payload wholesale, so
 * leaving a removed recipient's id out of `picks` here is what *deletes* her
 * stale pick from storage, not something the server quietly preserves on its
 * own; naming the current recipients exactly also keeps a stale or new pick
 * for a non-recipient from reaching the server and 400ing
 * (`pick_not_recipient`). */
export function buildPayload(d: SettingsDraft, recipientIds: string[]): AssignmentPayload {
  if (d.kind === "reading") {
    if (d.readingSource === "library") {
      return d.tier === null ? { source: "library", slug: d.slug.trim() } : { source: "library", slug: d.slug.trim(), tier: d.tier };
    }
    if (d.readingSource === "url") return { source: "url", url: d.url.trim() };
    if (d.readingSource === "personalized") {
      const keep = new Set(recipientIds);
      const picks: Record<string, PersonalPick> = {};
      // `d.picks` is null when the preview has not loaded (or failed) while
      // editing — controller ruling 2: fall back to what is already stored
      // rather than losing it, so a title-only save on an unstarted
      // personalized homework still works without the preview.
      if (d.picks) {
        for (const row of d.picks) {
          if (keep.has(row.userId)) picks[row.userId] = { slug: row.slug, tier: row.tier };
        }
      } else {
        for (const [uid, p] of Object.entries(d.savedPicks)) {
          if (keep.has(uid)) picks[uid] = p;
        }
      }
      return {
        source: "personalized",
        ...(d.disciplines.length ? { disciplines: d.disciplines } : {}),
        ...(d.personalTier !== null ? { tier: d.personalTier } : {}),
        picks,
      };
    }
    const fileName = d.fileName.trim();
    return d.readingSource === "file" && fileName
      ? { source: "text", text: d.text.trim(), fileName }
      : { source: "text", text: d.text.trim() };
  }
  if (d.kind === "writing") {
    // No `rubric` here, ever — there is no rubric editor on the create form
    // (controller ruling), and a settings-editable PATCH's nested payload
    // rubric is a no-op anyway: the server's `CarryRubric` step always
    // carries the stored rubric over a resent payload. The only path that
    // changes a rubric is `buildPatchInput`'s top-level `patch.rubric`.
    return { prompt: d.prompt.trim(), targetWords: parseTargetWords(d.targetWords) ?? 0, lang: d.lang };
  }
  const description = d.description.trim();
  return description
    ? { drivingQuestion: d.drivingQuestion.trim(), description }
    : { drivingQuestion: d.drivingQuestion.trim() };
}

function validateCommon(title: string, dueInput: string): string | null {
  const t = title.trim();
  if (!t || runes(t) > 200) return "请填写作业标题，不超过 200 字";
  if (!dueInput.trim()) return "请填写截止时间";
  if (beijingInputToISO(dueInput) === null) return "截止时间格式错误";
  return null;
}

export type Built<T> = { ok: true; value: T } | { ok: false; error: string };

export function buildCreateInput(d: AssignmentDraft): Built<CreateAssignmentInput> {
  if (!d.classId) return { ok: false, error: "请选择班级" };
  const common = validateCommon(d.title, d.dueInput);
  if (common) return { ok: false, error: common };
  const settings = validateSettings(d);
  if (settings) return { ok: false, error: settings };
  if (d.userIds.length === 0) return { ok: false, error: "请至少选择一名学生" };
  const instructions = d.instructions.trim();
  return {
    ok: true,
    value: {
      kind: d.kind,
      title: d.title.trim(),
      ...(instructions ? { instructions } : {}),
      payload: buildPayload(d, d.userIds),
      dueAt: beijingInputToISO(d.dueInput) ?? "",
      userIds: d.userIds,
    },
  };
}

export interface EditDraft {
  title: string;
  instructions: string;
  dueInput: string;
  settings: SettingsDraft;
  /** The rubric exactly as the server currently has it stored — the same
   *  value `settings.rubric` was seeded from when edit mode was opened, kept
   *  here as an unchanging snapshot so a later edit to `settings.rubric` can
   *  be told apart from "still what the server has". Required (not
   *  optional) so a caller cannot forget it: an omitted snapshot previously
   *  meant "always send", which turned every title-only edit on an existing
   *  writing homework into a rubric overwrite (fix round 1). `null` for a
   *  non-writing kind, or a writing homework whose payload carried no
   *  rubric (see `rubricDraftFromPayload`). */
  originalRubric: RubricDraft | null;
}

/** The PATCH body for the detail page's edit. `kind`/`payload` are sent only
 * when the settings are editable (no recipient has started); the server
 * compares values, so resending unchanged settings is not a change. The
 * rubric is a separate, always-editable top-level field (`patch.rubric`):
 * the server carries the stored rubric over any resent kind/payload
 * regardless (`CarryRubric`), so it only ever changes through this field —
 * and only when it actually changed, so a title-only edit on an older
 * homework never overwrites a custom rubric with whatever the draft
 * happened to be initialized as. `recipientIds` is a required parameter,
 * forwarded to `buildPayload` — only its personalized-reading branch reads
 * it, but every caller must name the current recipients (see `buildPayload`'s
 * doc comment for why an optional-with-throw version was rejected). */
export function buildPatchInput(e: EditDraft, settingsEditable: boolean, recipientIds: string[]): Built<PatchAssignmentInput> {
  const common = validateCommon(e.title, e.dueInput);
  if (common) return { ok: false, error: common };
  const patch: PatchAssignmentInput = {
    title: e.title.trim(),
    instructions: e.instructions.trim(),
    dueAt: beijingInputToISO(e.dueInput) ?? "",
  };
  if (settingsEditable) {
    // Controller ruling 2: editing a homework that was ALREADY personalized
    // has stored picks to fall back to, so its preview not having loaded (or
    // having failed) is not "invalid" the way a brand new one would be —
    // `buildPayload` falls back to `savedPicks`, so the wait-for-preview
    // check is skipped here. `storedPersonalized` is required, not just
    // "readingSource is personalized now": switching a stored library/url/
    // text homework to personalized in this same edit has no saved picks at
    // all, so it must still wait like a brand new one (fix round 1 — the
    // earlier, broader skip let that case save `{picks: {}}`, an unseen
    // automatic recommendation for every student).
    const waitingOnPreview =
      e.settings.kind === "reading" &&
      e.settings.readingSource === "personalized" &&
      e.settings.picks === null &&
      e.settings.storedPersonalized;
    if (!waitingOnPreview) {
      const settings = validateSettings(e.settings);
      if (settings) return { ok: false, error: settings };
    }
    patch.kind = e.settings.kind;
    patch.payload = buildPayload(e.settings, recipientIds);
  }
  if (e.settings.kind === "writing" && e.settings.rubric) {
    const rubric = validateRubricDraft(e.settings.rubric);
    if (rubric) return { ok: false, error: rubric };
    if (!e.originalRubric || !sameRubricDraft(e.settings.rubric, e.originalRubric)) {
      patch.rubric = buildRubric(e.settings.rubric);
    }
  }
  return { ok: true, value: patch };
}

const MAX_RETURN_NOTE = 500;

/** 退回修改 request body. Messages match lite_teacher_return.go. */
export function buildReturnInput(dueInput: string, note: string, nowMs: number): Built<ReturnRecipientInput> {
  const dueAt = beijingInputToISO(dueInput);
  if (!dueAt) return { ok: false, error: "请填写新的截止时间" };
  if (Date.parse(dueAt) <= nowMs) return { ok: false, error: "新的截止时间需要晚于现在" };
  const trimmed = note.trim();
  if (Array.from(trimmed).length > MAX_RETURN_NOTE) return { ok: false, error: "退回说明不超过 500 字" };
  return { ok: true, value: trimmed ? { dueAt, note: trimmed } : { dueAt } };
}

/** Kind and payload stay editable while no recipient has started — the same
 * rule the server enforces (an item linked = started). Status alone is not
 * enough: an unstarted recipient past the deadline reads 已逾期, not 未开始. */
export function canEditSettings(recipients: RecipientDTO[]): boolean {
  return recipients.every((r) => r.atomId === null && r.startedAt === null);
}

/** Class students who are not yet recipients, in roster order. */
export function unassignedStudents(roster: RosterRow[], recipients: RecipientDTO[]): RosterRow[] {
  const assigned = new Set(recipients.map((r) => r.userId));
  return roster.filter((s) => !assigned.has(s.id));
}

export function toggleId(ids: string[], id: string): string[] {
  return ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id];
}

/** Status columns on the assignment list, in the order a class moves through them. */
export const STATUS_ORDER: readonly AssignmentStatus[] = [
  "not_started",
  "in_progress",
  "done",
  "done_late",
  "overdue",
  "returned",
  "resubmitted",
];

const STATUS_HUE: Record<AssignmentStatus, string> = {
  not_started: "var(--mk-muted)",
  in_progress: "var(--mk-accent-500)",
  done: "var(--mk-success)",
  done_late: "var(--mk-warning)",
  overdue: "var(--mk-danger)",
  returned: "var(--mk-warning)",
  resubmitted: "var(--mk-success)",
};

/** Chip colours for a status. `color-mix` because Tailwind alpha modifiers on
 * `mk-*` tokens emit no CSS. An unknown status gets the muted tint.
 *
 * Text contrast is at least 4.5:1 on the 14% tint in the lite student theme
 * and the pro theme, light and dark, for every accent preset (measured
 * 2026-09-15). 进行中 uses `--mk-accent-700`, the step lite dark mode
 * brightens; `--mk-accent-500` stayed at the preset in dark mode and read
 * about 3.1–3.5:1. The other four have no stronger token, so their text is
 * the hue mixed 40% into ink: at 78% 逾期完成 read 2.8:1 in light mode and
 * 已逾期 4.2:1 in lite dark mode. */
export function statusChipStyle(status: string): { background: string; color: string } {
  const key: AssignmentStatus = Object.prototype.hasOwnProperty.call(STATUS_HUE, status)
    ? (status as AssignmentStatus)
    : "not_started";
  const tint = tintedChipStyle(STATUS_HUE[key]);
  return key === "in_progress" ? { ...tint, color: "var(--mk-accent-700)" } : tint;
}

/** A chip tinted with `hue`: 14% of the hue over the surface, text the hue
 * mixed 40% into ink, which keeps success, warning and danger text at or above
 * 4.5:1 in both lite themes. Shared by the status chips and the weekly card
 * tags. */
export function tintedChipStyle(hue: string): { background: string; color: string } {
  return {
    background: `color-mix(in srgb, ${hue} 14%, var(--mk-surface))`,
    color: `color-mix(in srgb, ${hue} 40%, var(--mk-ink))`,
  };
}

const TIER_NAMES = ["入门", "基础", "进阶", "高阶", "原文"];

export function tierLabel(tier: number | null): string {
  if (tier === null) return "按学生当前水平";
  return TIER_NAMES[tier - 1] ?? `第 ${tier} 档`;
}

/** The tier label for one student's personalized-reading pick or recipient
 * row: null means her own level, shown as 「按学生水平」. Distinct from
 * `tierLabel(null)`'s 「按学生当前水平」, which describes a class-wide
 * setting (the personalized homework's own tier filter, a library reading's
 * tier) rather than one student's resolved article. */
export function personalTierLabel(tier: number | null): string {
  return tier === null ? "按学生水平" : tierLabel(tier);
}

export type ExtractOutcome = { ok: true; text: string; fileName: string; title: string } | { ok: false; error: string };

/** A `/documents/extract` result → the text source. The 50000-character cap
 * is the server's; checking it here names the problem before 发布. */
export function readExtractResult(result: { title: string; text: string }, fileName: string): ExtractOutcome {
  const text = result.text.trim();
  if (!text) return { ok: false, error: "提取失败：文件中没有读到文字" };
  if (runes(text) > 50000) return { ok: false, error: "提取失败：正文超过 50000 字" };
  const name = cutRunes(fileName.trim(), 200);
  const stem = name.replace(/\.[^.]+$/, "");
  return { ok: true, text, fileName: name, title: cutRunes(result.title.trim() || stem, 200) };
}

export function extractedCountText(text: string): string {
  return `已提取 ${runes(text.trim())} 字`;
}

export function fillTitleIfEmpty(current: string, candidate: string): string {
  return current.trim() ? current : candidate;
}

/** Preview rows → pick rows. A kept pick from an earlier saved payload
 * (`origin: "saved"`) that still names the same article as the fresh
 * preview is NOT shown as a swap even when its tier differs from the
 * class-wide chip — only the article choice makes it a swap; her stored
 * tier is kept as-is rather than silently replaced by the chip (fix round
 * 1: a saved pick with `tier: null`, previewed again after raising the
 * chip, used to flip every row to 已更换 on a tier difference alone). A kept
 * pick made by swapping in THIS session (`origin: "swapped"`) always shows
 * as a swap, regardless of slug — that is a real teacher choice. Students
 * not in the preview (no longer enrolled) are dropped. */
export function mergePickRows(preview: PreviewRow[], personalTier: number | null, kept: Record<string, KeptPick>): PickRow[] {
  return preview.map((r) => {
    const base: PickRow = {
      userId: r.userId,
      name: r.name,
      slug: r.slug,
      title: r.title,
      tier: personalTier,
      suggestedTier: r.suggestedTier,
      reason: r.reason,
      swapped: false,
    };
    const k = kept[r.userId];
    if (!k) return base;
    if (k.origin === "saved" && k.slug === base.slug) {
      return k.tier === base.tier ? base : { ...base, tier: k.tier };
    }
    return { ...base, slug: k.slug, title: k.title, tier: k.tier, reason: "已更换", swapped: true };
  });
}

export function keptFromRows(rows: PickRow[] | null): Record<string, KeptPick> {
  const out: Record<string, KeptPick> = {};
  for (const r of rows ?? []) if (r.swapped) out[r.userId] = { slug: r.slug, title: r.title, tier: r.tier, origin: "swapped" };
  return out;
}

export function keptFromSaved(saved: Record<string, PersonalPick>, articles: LibraryArticle[]): Record<string, KeptPick> {
  const out: Record<string, KeptPick> = {};
  for (const [uid, p] of Object.entries(saved)) {
    out[uid] = { slug: p.slug, title: titleOf(articles.find((a) => a.slug === p.slug), p.slug), tier: p.tier, origin: "saved" };
  }
  return out;
}

export function swapPick(rows: PickRow[], userId: string, next: { slug: string; tier: number | null }, articles: LibraryArticle[]): PickRow[] {
  return rows.map((r) =>
    r.userId === userId
      ? { ...r, slug: next.slug, title: titleOf(articles.find((a) => a.slug === next.slug), next.slug), tier: next.tier, reason: "已更换", swapped: true }
      : r,
  );
}

export function visiblePickRows(rows: PickRow[], recipientIds: string[]): PickRow[] {
  const keep = new Set(recipientIds);
  return rows.filter((r) => keep.has(r.userId));
}

/** The tier a row would actually be sent at (and — once picked — the tier
 * she starts at): her own pick's tier, else the class-wide 难度 chip
 * (`personalTier`), else her suggested tier — the same order the server's
 * `pickedTier` resolves a null pick tier in. A bare `row.tier ?? row.
 * suggestedTier` skipped the middle step and showed her suggested level even
 * with a class-wide tier set (fix round 1). */
export function pickTierText(row: PickRow, personalTier: number | null): string {
  return tierLabel(row.tier ?? personalTier ?? row.suggestedTier);
}

/** Discipline tags that appear on library articles, each once, in library order. */
export function disciplineOptions(articles: LibraryArticle[]): LibraryTag[] {
  const seen = new Set<string>();
  const out: LibraryTag[] = [];
  for (const a of articles) {
    for (const t of Array.isArray(a.tags) ? a.tags : []) {
      if (!seen.has(t.id)) {
        seen.add(t.id);
        out.push(t);
      }
    }
  }
  return out;
}

export function recipientReadingText(reading: RecipientReading | null): string {
  if (!reading) return "—";
  if (reading.state === "pending") return "待推荐";
  return `${reading.title || reading.slug} · ${personalTierLabel(reading.tier)}`;
}

export function assignmentFileName(a: { kind: AssignmentKind; payload: Record<string, unknown> }): string | null {
  if (a.kind !== "reading" || a.payload.source !== "text") return null;
  const name = str(a.payload.fileName).trim();
  return name || null;
}

/** Title-substring search over the shelf (English or Chinese title). */
export function filterArticles(articles: LibraryArticle[], query: string): LibraryArticle[] {
  const q = query.trim().toLowerCase();
  if (!q) return articles;
  return articles.filter((a) => a.title.toLowerCase().includes(q) || (a.zhTitle ?? "").toLowerCase().includes(q));
}

/** One-line settings summary for the detail header, e.g. `目标字数 800 · 中文`.
 * A link source is rendered by the page itself (through `safeHttpUrl`), so
 * only its label is here. Long text (题目 / 驱动问题) is shown as its own block. */
export function settingsSummary(kind: AssignmentKind, payload: Record<string, unknown>, articleTitle?: string | null): string {
  const d = settingsFromAssignment(kind, payload);
  if (kind === "reading") {
    if (d.readingSource === "library") return ["分级阅读库", articleTitle || d.slug || "—", tierLabel(d.tier)].join(" · ");
    if (d.readingSource === "url") return "链接";
    if (d.readingSource === "file") return `上传文件 · ${d.fileName} · ${runes(d.text)} 字`;
    if (d.readingSource === "personalized") {
      // personalTierLabel, not tierLabel: null here means each student's
      // own level, shown as 按学生水平 — the library source's null (a fixed
      // class-wide default) keeps tierLabel's 按学生当前水平 above.
      const parts = ["个性化阅读", personalTierLabel(d.personalTier)];
      if (d.disciplines.length) parts.push(`学科筛选 ${d.disciplines.length} 项`);
      return parts.join(" · ");
    }
    return `正文 · ${runes(d.text)} 字`;
  }
  if (kind === "writing") {
    return [`目标字数 ${d.targetWords || "—"}`, d.lang === "en" ? "英文" : "中文"].join(" · ");
  }
  return "项目";
}

export function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

/** `{verb}失败：{message}`, unless the server's message already starts with
 * that prefix — then it is shown as-is rather than doubled. */
export function failText(verb: string, e: unknown): string {
  const msg = errorText(e);
  return msg.startsWith(`${verb}失败`) ? msg : `${verb}失败：${msg}`;
}

/** After a confirmed archive, 404 also means archived: a repeat DELETE on an
 * archived assignment is not found (controller Ruling P2-4). */
export function isArchiveSuccess(e: unknown): boolean {
  return e instanceof ApiError && e.status === 404;
}

/**
 * `AssignmentDetailPage`'s 学生/批改 tab after `assignmentId` potentially
 * changes. The page's reset effect runs on every mount too — not only after
 * a genuine switch to a different assignment — so it must tell the two
 * apart: the FIRST run (mount) leaves `currentTab` untouched (that is what
 * lets `?tab=grading`'s `initialTab` actually land her on 批改, instead of
 * being overwritten one tick after the initial render); a real switch
 * (`prevId !== nextId`) always resets to 学生, since a fresh assignment has
 * no reason to inherit whatever tab the previous one happened to be on.
 */
export function tabAfterAssignmentChange(
  prevId: string,
  nextId: string,
  currentTab: "students" | "grading",
): "students" | "grading" {
  return prevId === nextId ? currentTab : "students";
}

/** Which class a page opens on: an explicit choice, else the remembered one,
 * else the first — each only if it is still one of her classes. */
export function pickClassId(classIds: string[], preferred?: string | null, remembered?: string | null): string {
  if (preferred && classIds.includes(preferred)) return preferred;
  if (remembered && classIds.includes(remembered)) return remembered;
  return classIds[0] ?? "";
}

const LAST_CLASS_KEY = "lite-teacher:assignments:lastClassId";

export function readLastClassId(): string | null {
  try {
    return window.localStorage.getItem(LAST_CLASS_KEY);
  } catch {
    return null;
  }
}

export function writeLastClassId(classId: string): void {
  try {
    window.localStorage.setItem(LAST_CLASS_KEY, classId);
  } catch {
    /* storage blocked: the page still works, it just forgets */
  }
}
