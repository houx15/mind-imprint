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
import { tierOrNull } from "../api/assignments";
import type {
  AssignmentKind,
  AssignmentPayload,
  CreateAssignmentInput,
  PatchAssignmentInput,
  PersonalPick,
  PreviewRow,
  ReadingAssignmentPayload,
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
   * swap made in this editing session ("swapped"). `mergePickRows` treats a
   * same-article "saved" pick as unswapped, keeping its own tier; a session
   * swap is always shown as a swap, regardless of slug. */
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

/** The fields the workspace turn endpoint's `liteWorkspaceArtifact` actually
 * reads (apps/api/internal/api/lite_teacher_workspace.go's `artifact`
 * struct) — never the whole draft. Sending the whole draft put a
 * 50000-rune pasted-text material back on the wire on every later turn,
 * just because it lives on the same draft object:
 * the server ignores JSON fields it does not declare, but the bytes still
 * cross the network before it gets the chance to. Built as an explicit
 * object, not a spread of the draft, so a future large field on the draft
 * cannot leak onto this wire the same way. */
export interface WorkspaceArtifactPayload {
  kind: AssignmentKind;
  title: string;
  instructions: string;
  dueInput: string;
  readingSource: ReadingSource;
  slug: string;
  tier: number | null;
  userIds: string[];
}

export function workspaceArtifactPayload(d: AssignmentDraft): WorkspaceArtifactPayload {
  return {
    kind: d.kind,
    title: d.title,
    instructions: d.instructions,
    dueInput: d.dueInput,
    readingSource: d.readingSource,
    slug: d.slug,
    tier: d.tier,
    userIds: d.userIds,
  };
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
    out[uid] = { slug: p.slug, tier: tierOrNull(p.tier) };
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
      // The upload tab needs an extracted file: text left over from the paste
      // tab must not publish as uploaded content.
      if (d.readingSource === "file" && !d.fileName.trim()) return "请上传文件";
      const text = d.text.trim();
      // A file name is already required above, so reaching here on the
      // upload tab means she cleared the extracted text, not that she never
      // uploaded — the message must tell those two apart.
      if (!text) return d.readingSource === "file" ? "请填写文章正文" : "请粘贴文章正文";
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
 * `recipientIds` names the assignment's current recipients; only the
 * `personalized` reading branch reads it, sending picks for those ids only.
 * A PATCH's `payload` field replaces the stored payload wholesale, so an id
 * left out of `picks` here deletes that student's pick, and naming exactly
 * the current recipients keeps a stale or new pick for a non-recipient from
 * reaching the server and 400ing (`pick_not_recipient`). */
export function buildPayload(d: SettingsDraft, recipientIds: string[]): AssignmentPayload {
  if (d.kind === "reading") {
    if (d.readingSource === "library") {
      return d.tier === null ? { source: "library", slug: d.slug.trim() } : { source: "library", slug: d.slug.trim(), tier: d.tier };
    }
    if (d.readingSource === "url") return { source: "url", url: d.url.trim() };
    if (d.readingSource === "personalized") {
      const keep = new Set(recipientIds);
      const picks: Record<string, PersonalPick> = {};
      // `d.picks` is null while the preview has not loaded (or failed): fall
      // back to the picks already stored, so a title-only save on an
      // unstarted personalized homework still works without the preview.
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
    // No `rubric` here, ever: there is no rubric editor on the create form,
    // and a settings-editable PATCH's nested payload rubric is a no-op
    // anyway — the server's `CarryRubric` step always carries the stored
    // rubric over a resent payload. Only `buildPatchInput`'s top-level
    // `patch.rubric` changes a rubric.
    return { prompt: d.prompt.trim(), targetWords: parseTargetWords(d.targetWords) ?? 0, lang: d.lang };
  }
  const description = d.description.trim();
  return description
    ? { drivingQuestion: d.drivingQuestion.trim(), description }
    : { drivingQuestion: d.drivingQuestion.trim() };
}

/** The create form's 班级 selector changed to `classId`: any personalized-
 * reading `picks` on the draft belong to the OLD class's students, so they
 * must go back to `null`. Otherwise `validateSettings` passes on the stale
 * rows while class B's preview is loading or has failed, and 发布 filters
 * every stale row out against the new roster and sends `{picks: {}}`.
 * `disciplines` and `personalTier` are a class-independent filter, not tied
 * to one class's students, so they are kept as-is. */
export function draftOnClassChange(d: AssignmentDraft, classId: string): AssignmentDraft {
  return { ...d, classId, picks: null };
}

/** 类型 changed to `kind`. Only a reading homework has a material row, so
 * leaving 阅读 takes the chosen article with it — the same three fields the
 * server's `set_fields` clears on the same transition
 * (apps/api/internal/api/lite_teacher_workspace.go).
 *
 * The AI mode needs this because the card it holds is sent back to the model as
 * card state on the next turn: a draft that says 「种类：writing」 beside
 * 「文章 slug：…」 shows the model a card that cannot exist, which is the state
 * the server guard exists to prevent. Arriving through her own 类型 control
 * rather than through a tool does not make it a different state. */
export function draftOnKindChange<T extends SettingsDraft>(d: T, kind: AssignmentKind): T {
  if (kind === "reading") return { ...d, kind };
  return { ...d, kind, readingSource: "library", slug: "", tier: null, text: "" };
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
   *  be told apart from "still what the server has". Required, not
   *  optional: a missing snapshot must never be read as "always send",
   *  which would turn a title-only edit on an existing writing homework
   *  into a rubric overwrite. `null` for a non-writing kind, or a writing
   *  homework whose payload carried no rubric (see
   *  `rubricDraftFromPayload`). */
  originalRubric: RubricDraft | null;
}

/** How much of the kind/payload the detail page may change: "all" before any
 * recipient starts; "picks" once someone has started on a homework that was
 * already personalized (only unstarted students' picks); "none" otherwise.
 * The server applies the same rule (`checkSettingsChangeAllowed`). */
export type SettingsAccess = "all" | "picks" | "none";

export function settingsAccess(recipients: RecipientDTO[], stored: SettingsDraft): SettingsAccess {
  if (canEditSettings(recipients)) return "all";
  return stored.kind === "reading" && stored.storedPersonalized ? "picks" : "none";
}

/** The PATCH body for the detail page's edit. `kind`/`payload` are sent in
 * full only when `access` is "all"; the server compares values, so resending
 * unchanged settings is not a change. With "picks", only `payload` is sent,
 * and every started student's pick is put back exactly as stored
 * (`startedIds`), so the only differences the server sees are picks for
 * students who have not started. With "none", neither is sent. The
 * rubric is a separate, always-editable top-level field (`patch.rubric`):
 * the server carries the stored rubric over any resent kind/payload
 * regardless (`CarryRubric`), so it only ever changes through this field —
 * and only when it actually changed, so a title-only edit on an older
 * homework never overwrites a custom rubric with whatever the draft
 * happened to be initialized as. `recipientIds` is forwarded to
 * `buildPayload`, which every caller must give the current recipients (see
 * `buildPayload`'s doc comment). */
export function buildPatchInput(
  e: EditDraft,
  access: SettingsAccess,
  recipientIds: string[],
  startedIds: string[] = [],
): Built<PatchAssignmentInput> {
  const common = validateCommon(e.title, e.dueInput);
  if (common) return { ok: false, error: common };
  const patch: PatchAssignmentInput = {
    title: e.title.trim(),
    instructions: e.instructions.trim(),
    dueAt: beijingInputToISO(e.dueInput) ?? "",
  };
  if (access === "picks") {
    const d = e.settings;
    // No preview loaded means no pick was swapped: nothing to send.
    if (d.kind === "reading" && d.readingSource === "personalized" && d.storedPersonalized && d.picks !== null) {
      const payload = buildPayload(d, recipientIds) as ReadingAssignmentPayload;
      const picks = { ...(payload.picks ?? {}) };
      for (const uid of startedIds) {
        const saved = d.savedPicks[uid];
        if (saved) picks[uid] = saved;
        else delete picks[uid];
      }
      patch.payload = { ...payload, picks };
    }
  }
  if (access === "all") {
    // A homework that was ALREADY personalized has stored picks to fall
    // back to, so its preview not having loaded (or having failed) is not
    // "invalid" the way a brand new one would be — `buildPayload` falls back
    // to `savedPicks`, so the wait-for-preview check is skipped here.
    // `storedPersonalized` must be checked, not just "readingSource is
    // personalized now": switching a stored library/url/text homework to
    // personalized in this same edit has no saved picks at all, so it must
    // still wait like a brand new one — otherwise the save would silently
    // send `{picks: {}}`, an unseen automatic recommendation for every
    // student.
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

/** Whether this recipient has started (an item linked or a start time set).
 * Status alone is not enough: an unstarted recipient past the deadline reads
 * 已逾期, not 未开始. */
export function recipientStarted(r: RecipientDTO): boolean {
  return r.atomId !== null || r.startedAt !== null;
}

/** Kind and payload stay editable while no recipient has started — the same
 * rule the server enforces. */
export function canEditSettings(recipients: RecipientDTO[]): boolean {
  return !recipients.some(recipientStarted);
}

/** A personalized pick can change until that student starts. A user who is
 * not a recipient (the create form, or a student not yet added) has not
 * started. */
export function canSwapPick(recipients: RecipientDTO[], userId: string): boolean {
  const r = recipients.find((x) => x.userId === userId);
  return !r || !recipientStarted(r);
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

/** A null tier always means her own level, on a library reading's fixed
 * default and on a personalized pick alike. */
export function tierLabel(tier: number | null): string {
  if (tier === null) return "按学生水平";
  return TIER_NAMES[tier - 1] ?? `第 ${tier} 档`;
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

/** Preview rows → pick rows. Every row starts with `tier: null`, so an
 * unswapped row always follows the class-wide 难度 chip when it is shown or
 * sent (`pickTierText`, the server's `pickedTier`) instead of freezing
 * whatever the chip read the moment the preview ran. A kept pick from an
 * earlier saved payload (`origin: "saved"`) that still names the same
 * article as the fresh preview keeps its own stored tier instead — only the
 * article choice makes it a swap. A kept pick made by swapping in THIS
 * session (`origin: "swapped"`) always shows as a swap, regardless of slug —
 * that is a real teacher choice. Students not in the preview (no longer
 * enrolled) are dropped. */
export function mergePickRows(preview: PreviewRow[], personalTier: number | null, kept: Record<string, KeptPick>): PickRow[] {
  return preview.map((r) => {
    const base: PickRow = {
      userId: r.userId,
      name: r.name,
      slug: r.slug,
      title: r.title,
      tier: null,
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

/** The tier text for one picked row — and, once picked, the tier she
 * actually starts at: her own pick's tier if set, else the class-wide 难度
 * chip (`personalTier`), the same order the server's `pickedTier` resolves a
 * null pick tier in. With neither set, this names today's estimate rather
 * than stating it as fact — 「按学生水平（进阶）」 — since her real tier is
 * only resolved when she starts. */
export function pickTierText(row: PickRow, personalTier: number | null): string {
  if (row.tier !== null) return tierLabel(row.tier);
  if (personalTier !== null) return tierLabel(personalTier);
  if (row.suggestedTier) return `按学生水平（${tierLabel(row.suggestedTier)}）`;
  return tierLabel(null);
}

/** Shown in place of an article title the page does not have. A slug is a
 *  system value and never goes on a teacher screen (spec §12.1). */
export const MISSING_ARTICLE_TEXT = "文章信息缺失";

export interface PickRowView {
  title: string;
  tierText: string;
  /** Empty for a started row: the preview's reason is about the article the
   *  preview would pick now, not the one she is reading. */
  reason: string;
  started: boolean;
}

/**
 * One row of the personalized picker as the teacher sees it.
 *
 * A started student's row shows the article she is actually reading (her
 * recipient row), since a fresh preview may pick a different one. If that
 * row does not say what she is reading, the title is MISSING_ARTICLE_TEXT
 * rather than the preview's pick. An unstarted row shows the preview or the
 * teacher's pick. In both cases a title that is empty or is only the slug
 * (the shelf failed to load) shows as MISSING_ARTICLE_TEXT.
 */
export function pickRowView(row: PickRow, recipients: RecipientDTO[], personalTier: number | null): PickRowView {
  const readable = (title: string, slug: string) => (title && title !== slug ? title : MISSING_ARTICLE_TEXT);
  if (canSwapPick(recipients, row.userId)) {
    return { title: readable(row.title, row.slug), tierText: pickTierText(row, personalTier), reason: row.reason, started: false };
  }
  const reading = recipients.find((x) => x.userId === row.userId)?.reading;
  if (reading && reading.state === "started") {
    return { title: readable(reading.title, reading.slug), tierText: tierLabel(reading.tier), reason: "", started: true };
  }
  return { title: MISSING_ARTICLE_TEXT, tierText: "—", reason: "", started: true };
}

/** The selected 学科筛选 as read-only text (picks mode, where the filter can no
 *  longer change). Each id shows as its Chinese label from the shelf; an id
 *  the shelf does not carry is counted, never shown raw. */
export function selectedDisciplinesText(ids: string[], options: LibraryTag[]): string {
  if (ids.length === 0) return "不限学科";
  const byId = new Map(options.map((t) => [t.id, t.zh]));
  const labels: string[] = [];
  let missing = 0;
  for (const id of ids) {
    const zh = byId.get(id);
    if (zh) labels.push(zh);
    else missing += 1;
  }
  if (missing > 0) labels.push(`另有 ${missing} 个学科（名称加载失败）`);
  return labels.join("、");
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

/**
 * 学习进度 on a homework's student row: how far she got, not only whether she
 * pressed 完成. Reading: steps done of the plan; writing: the latest
 * submitted version and its length; every kind: minutes spent. "—" before
 * she has started.
 */
export function recipientProgressText(
  kind: AssignmentKind,
  r: Pick<RecipientDTO, "atomId" | "activeMinutes" | "stepsDone" | "stepsTotal" | "versionCount" | "latestWordCount">,
  targetWords: number | null = null,
): string {
  if (!r.atomId) return "—";
  const parts: string[] = [];
  if (kind === "reading" && r.stepsTotal > 0) parts.push(`${r.stepsDone}/${r.stepsTotal} 步`);
  if (kind === "writing" && r.versionCount > 0) {
    parts.push(`v${r.versionCount} · ${r.latestWordCount}${targetWords ? ` / ${targetWords}` : ""} 字`);
  }
  if (kind === "writing" && r.versionCount === 0) parts.push("未提交");
  parts.push(r.activeMinutes > 0 ? `${r.activeMinutes} 分钟` : "不到 1 分钟");
  return parts.join(" · ");
}

/** The weekday of a datetime-local value (2026-09-18T21:00 → 周五), or ""
 *  when it cannot be read. The input shows no weekday, and a deadline that
 *  landed on Sunday instead of Friday looked right (real-user walk,
 *  2026-09-17). The value is Beijing wall-clock text, so the date part alone
 *  decides the weekday — no time zone is involved. */
export function dueWeekday(dueInput: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})T\d{2}:\d{2}$/.exec(dueInput.trim());
  if (!m) return "";
  const day = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))).getUTCDay();
  return ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][day] ?? "";
}

export function recipientReadingText(reading: RecipientReading | null): string {
  if (!reading) return "—";
  if (reading.state === "pending") return "待推荐";
  return `${reading.title || reading.slug} · ${tierLabel(reading.tier)}`;
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

/** Articles carrying at least one of `ids` as a discipline tag; no ids means
 *  no filter. */
export function filterByDisciplines(articles: LibraryArticle[], ids: string[]): LibraryArticle[] {
  if (ids.length === 0) return articles;
  const want = new Set(ids);
  return articles.filter((a) => (Array.isArray(a.tags) ? a.tags : []).some((t) => want.has(t.id)));
}

/**
 * The picker's next value after a click on the card for `slug`. Selection is
 * by slug alone, so the same article clicked in the recommended row or in the
 * full grid gives the same result, and both show it as selected. Clicking the
 * selected card keeps it (and its tier); another article starts from a null
 * tier, since it may not have the chosen level.
 */
export function pickArticle(
  current: { slug: string; tier: number | null },
  slug: string,
): { slug: string; tier: number | null } {
  return slug === current.slug ? current : { slug, tier: null };
}

/**
 * The part of the full grid shown before 显示全部. A selected article past
 * the cut is moved to the front (and the page stays `limit` long), so the
 * chosen card is the first thing she sees. One already inside the page keeps
 * its place, so a click does not make the card jump.
 */
export function pageArticles(list: LibraryArticle[], limit: number, selectedSlug: string): LibraryArticle[] {
  if (list.length <= limit) return list;
  const head = list.slice(0, limit);
  if (!selectedSlug || head.some((a) => a.slug === selectedSlug)) return head;
  const selected = list.find((a) => a.slug === selectedSlug);
  return selected ? [selected, ...head.slice(0, limit - 1)] : head;
}

/**
 * The discipline filter chips: tags ordered by how many articles carry them
 * (ties keep library order). Collapsed, only the first `limit` show, plus any
 * chosen tag past the cut so an active filter is never hidden; `hidden`
 * counts the rest for the 更多 chip.
 */
export function disciplineChips(
  articles: LibraryArticle[],
  opts: { limit: number; expanded: boolean; chosen: string[] },
): { shown: LibraryTag[]; hidden: number } {
  const counts = new Map<string, number>();
  for (const a of articles) {
    for (const t of Array.isArray(a.tags) ? a.tags : []) counts.set(t.id, (counts.get(t.id) ?? 0) + 1);
  }
  const ordered = disciplineOptions(articles)
    .map((t, i) => ({ t, i }))
    .sort((x, y) => (counts.get(y.t.id) ?? 0) - (counts.get(x.t.id) ?? 0) || x.i - y.i)
    .map((x) => x.t);
  if (opts.expanded || ordered.length <= opts.limit) return { shown: ordered, hidden: 0 };
  const chosen = new Set(opts.chosen);
  const shown = ordered.filter((t, i) => i < opts.limit || chosen.has(t.id));
  return { shown, hidden: ordered.length - shown.length };
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
      const parts = ["个性化阅读", tierLabel(d.personalTier)];
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
 * already-archived assignment is treated as success, not a failure. */
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

export type AssignmentMode = "traditional" | "ai";

const ASSIGNMENT_MODE_KEY = "lite.teacher.assignmentMode";

/** The create page's last chosen mode; a teacher with no record yet gets
 * "ai", not the plain form. */
export function readAssignmentMode(): AssignmentMode {
  try {
    return window.localStorage.getItem(ASSIGNMENT_MODE_KEY) === "traditional" ? "traditional" : "ai";
  } catch {
    return "ai";
  }
}

export function writeAssignmentMode(mode: AssignmentMode): void {
  try {
    window.localStorage.setItem(ASSIGNMENT_MODE_KEY, mode);
  } catch {
    /* storage blocked: the page still works, it just forgets */
  }
}
