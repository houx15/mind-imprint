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
  RecipientDTO,
} from "../api/assignments";
import type { LibraryArticle } from "../api/library";
import type { RosterRow } from "../api/teacher";
import { beijingInputToISO, type AssignmentStatus } from "../shared/deadline";
import { safeHttpUrl } from "./format";

export type ReadingSource = "library" | "url" | "text";

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
  };
}

const str = (v: unknown): string => (typeof v === "string" ? v : "");

/** A stored assignment's kind + payload → an editable draft. */
export function settingsFromAssignment(kind: AssignmentKind, payload: Record<string, unknown>): SettingsDraft {
  const d = emptySettings(kind);
  if (kind === "reading") {
    const source = payload.source;
    d.readingSource = source === "url" || source === "text" ? source : "library";
    d.slug = str(payload.slug);
    d.tier = typeof payload.tier === "number" ? payload.tier : null;
    d.url = str(payload.url);
    d.text = str(payload.text);
  } else if (kind === "writing") {
    d.prompt = str(payload.prompt);
    d.targetWords = typeof payload.targetWords === "number" && payload.targetWords > 0 ? String(payload.targetWords) : "";
    d.lang = payload.lang === "en" ? "en" : "zh";
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
    } else {
      const text = d.text.trim();
      if (!text) return "请粘贴文章正文";
      if (runes(text) > 50000) return "文章正文不能超过 50000 字";
    }
    return null;
  }
  if (d.kind === "writing") {
    const prompt = d.prompt.trim();
    if (!prompt) return "请填写写作题目";
    if (runes(prompt) > 2000) return "写作题目不能超过 2000 字";
    if (parseTargetWords(d.targetWords) === null) return "目标字数需在 1 到 100000 之间";
    return null;
  }
  const q = d.drivingQuestion.trim();
  if (!q) return "请填写驱动问题";
  if (runes(q) > 4000) return "驱动问题不能超过 4000 字";
  return null;
}

/** The payload for a draft that passed `validateSettings`. A library reading
 * with no chosen level leaves `tier` out, which the server reads as "use the
 * student's current level". */
export function buildPayload(d: SettingsDraft): AssignmentPayload {
  if (d.kind === "reading") {
    if (d.readingSource === "library") {
      return d.tier === null ? { source: "library", slug: d.slug.trim() } : { source: "library", slug: d.slug.trim(), tier: d.tier };
    }
    if (d.readingSource === "url") return { source: "url", url: d.url.trim() };
    return { source: "text", text: d.text.trim() };
  }
  if (d.kind === "writing") {
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
      payload: buildPayload(d),
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
}

/** The PATCH body for the detail page's edit. `kind`/`payload` are sent only
 * when the settings are editable (no recipient has started); the server
 * compares values, so resending unchanged settings is not a change. */
export function buildPatchInput(e: EditDraft, settingsEditable: boolean): Built<PatchAssignmentInput> {
  const common = validateCommon(e.title, e.dueInput);
  if (common) return { ok: false, error: common };
  const patch: PatchAssignmentInput = {
    title: e.title.trim(),
    instructions: e.instructions.trim(),
    dueAt: beijingInputToISO(e.dueInput) ?? "",
  };
  if (settingsEditable) {
    const settings = validateSettings(e.settings);
    if (settings) return { ok: false, error: settings };
    patch.kind = e.settings.kind;
    patch.payload = buildPayload(e.settings);
  }
  return { ok: true, value: patch };
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
export const STATUS_ORDER: readonly AssignmentStatus[] = ["not_started", "in_progress", "done", "done_late", "overdue"];

const STATUS_HUE: Record<AssignmentStatus, string> = {
  not_started: "var(--mk-muted)",
  in_progress: "var(--mk-accent-500)",
  done: "var(--mk-success)",
  done_late: "var(--mk-warning)",
  overdue: "var(--mk-danger)",
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
  const hue = STATUS_HUE[key];
  return {
    background: `color-mix(in srgb, ${hue} 14%, var(--mk-surface))`,
    color: key === "in_progress" ? "var(--mk-accent-700)" : `color-mix(in srgb, ${hue} 40%, var(--mk-ink))`,
  };
}

const TIER_NAMES = ["入门", "基础", "进阶", "高阶", "原文"];

export function tierLabel(tier: number | null): string {
  if (tier === null) return "按学生当前水平";
  return TIER_NAMES[tier - 1] ?? `第 ${tier} 档`;
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
