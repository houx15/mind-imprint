import { apiFetch } from "./client";

// api/writings.ts — the lite shell's 写作 CRUD client. Shapes read straight
// off the Go handlers (apps/api/internal/api/writings.go), not guessed:
// `writingDTO` (id/title/lang/stage/targetWords/status/createdAt/updatedAt/
// lastActivityAt/finishedAt) and createWriting's `{"id": "..."}` 201 body.
// Mirrors api/readings.ts's shape; the room-specific calls (turn/outline/
// snippets/compose/cards/summon) live in api/writingRoom.ts instead, matching
// the reading split (readings.ts vs readingRoom.ts).

export interface Writing {
  id: string;
  title: string;
  lang: string;
  /** One of 'outline' | 'snippets' | 'draft' | 'finished' — the three-stage
   *  MAP position, 结构 / 段落 / 成稿 (铁律②: a map, never a gate). The old
   *  'ideate' step was retired on 2026-08-27: migration 0100 moved every row
   *  onto 'outline', and the server now 400s on it. */
  stage: string;
  targetWords: number | null;
  /** Which skeleton she picked out of the fixed structure library; "" = none
   *  yet, which is what the 结构 page's empty state keys on. */
  structureKey: string;
  /** When the entry 设定 dialog was completed. null = never, and that is
   *  exactly what opens it — see WritingSetupModal. */
  setupAt: string | null;
  /** 'active' | 'finished' — independent of `stage` (a finished piece may
   *  still show any stage; see writing_stage.go's file comment). */
  status: string;
  createdAt: string;
  /** `writing.updated_at` — rename / stage change / target-words only. NOT
   *  "when she last worked on this". */
  updatedAt: string;
  /** `atom.last_activity_at` — the last time she wrote anything into this
   *  writing (a turn, an outline edit, a snippet, a draft save). This is
   *  what 上次改到 means and what 「你有 N 篇还没写完」 orders by. Mirrors
   *  readings.ts's `Reading.lastActivityAt` exactly. */
  lastActivityAt: string;
  finishedAt: string | null;
  /** Set while she edits a finished writing again. `status` stays
   *  "finished" the whole time (a homework stays 已提交). */
  revisingAt?: string | null;
  /** `"here"` 在这个房间里写的 / `"brought"` 她带进来的成稿（0146）。
   *  界面据此说明结构和段落两步没有发生过。老的行读到 "here"。 */
  origin?: string;
  /** The teacher's prompt for a writing started from an assignment; null (or
   *  missing, from an older server) for a writing she opened herself. It is the
   *  teacher's text: the room shows it labelled 题目, never as her message. */
  assignedPrompt?: string | null;
}

/** The teacher's prompt to show in the room, or null. A blank prompt counts as
 *  none, so the room never renders an empty 题目 line. */
export function assignedPromptOf(w: Pick<Writing, "assignedPrompt">): string | null {
  const prompt = typeof w.assignedPrompt === "string" ? w.assignedPrompt.trim() : "";
  return prompt ? prompt : null;
}

/** Whether the writing was started from a teacher's assignment. Its language
 *  and target are then the teacher's: the setup dialog shows them read-only
 *  and the room's length meter cannot change the target. Same rule as the
 *  server's `writingIsAssigned`: a blank prompt is not an assignment. */
export function isAssignedWriting(w: Pick<Writing, "assignedPrompt">): boolean {
  return assignedPromptOf(w) !== null;
}

/** A writing is finished when the server says so — same both-fields
 *  tolerance as readings.ts's `isFinished`. */
export function isWritingFinished(w: Pick<Writing, "status" | "finishedAt">): boolean {
  return w.status === "finished" || Boolean(w.finishedAt);
}

/** She reopened a finished writing to edit it. */
export function isRevising(w: Pick<Writing, "revisingAt">): boolean {
  return typeof w.revisingAt === "string" && w.revisingAt !== "";
}

/**
 * GET /api/v1/writings — 我的写作.
 *
 * Mirrors listReadings (readings.ts): the server orders by when each writing
 * last MATTERED (finished → when she finished it; still open →
 * `lastActivityAt`) and cuts to `limit`, so this is genuinely the most
 * recent N rather than the first N of everything.
 */
export async function listWritings(limit?: number): Promise<Writing[]> {
  const query = limit ? `?limit=${encodeURIComponent(String(limit))}` : "";
  const raw = await apiFetch<{ writings: Writing[] }>(`/api/v1/writings${query}`);
  return raw.writings;
}

/**
 * POST /api/v1/writings — "type one sentence into a box and you're started."
 * `idea` must be non-empty (the server 400s on blank: unlike a reading, a
 * writing has nothing to talk about without one).
 *
 * `lang` here is PROVISIONAL. It used to be final, and the landing box
 * hardcoded "zh" for anything freely typed — so a student who typed her own
 * English essay idea silently lost every English-only affordance in the room
 * and had no way to discover why. The entry 设定 dialog now asks her outright
 * and overwrites this via `setWritingSetup` before she does anything, so a
 * wrong guess at creation time costs nothing.
 *
 * `body` 是她**已经写完**带进来的那一篇（2026-09-11）。给了 body，服务端会把
 * 这一篇直接落在 draft、来源记成 brought —— 她要的是意见，不是从零开始。
 * 正文只进 writing_draft，不进转录（理由见 writings.go）。
 */
export async function createWriting(input: {
  idea: string;
  lang?: "zh" | "en";
  body?: string;
}): Promise<{ id: string }> {
  // No lang → the server guesses from the idea and body (guessWritingLang).
  // Only a caller that KNOWS the language (a topic tile) sends one.
  const payload: Record<string, string> = { idea: input.idea };
  if (input.lang) payload.lang = input.lang;
  if (input.body) payload.body = input.body;
  return apiFetch<{ id: string }>("/api/v1/writings", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/** GET /api/v1/writings/{id} */
export async function getWriting(id: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}`);
}

/** PATCH /api/v1/writings/{id} — renames the writing. Title must be
 *  non-empty (the server 400s on blank). */
export async function renameWriting(id: string, title: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ title }),
  });
}

/**
 * POST /api/v1/documents/extract —— 把一个上传的文件变成纯文本，**不落任何库**。
 *
 * 阅读那一侧的上传会直接把文本落成那一篇阅读材料（和一个 reading id 绑死）；
 * 写作要的是另一件事：把文字取出来放进她正在写的那个框，落不落库由她按
 * 「请印记看看」的时候决定。共用的是取文字那一段，不是落库那一段。
 *
 * 收哪几种格式由服务端的 docextract.Supported 说了算。
 */
export async function extractDocument(file: File): Promise<{ title: string; text: string }> {
  const form = new FormData();
  form.append("file", file);
  return apiFetch<{ title: string; text: string }>("/api/v1/documents/extract", {
    method: "POST",
    body: form,
  });
}

export interface WritingVersionSummary {
  number: number;
  title: string;
  wordCount: number;
  submittedAt: string;
}

export interface WritingVersion extends WritingVersionSummary {
  body: string;
}

export interface WritingVersionList {
  /** Newest first. */
  versions: WritingVersionSummary[];
  /** A submitted homework past its deadline: 修改 is refused (403 writing_locked). */
  locked: boolean;
  lockReason: string | null;
}

export function normalizeVersionSummary(raw: unknown): WritingVersionSummary {
  const r = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  return {
    number: typeof r.number === "number" ? r.number : 0,
    title: typeof r.title === "string" ? r.title : "",
    wordCount: typeof r.wordCount === "number" ? r.wordCount : 0,
    submittedAt: typeof r.submittedAt === "string" ? r.submittedAt : "",
  };
}

export function normalizeWritingVersionList(raw: unknown): WritingVersionList {
  const r = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  return {
    versions: (Array.isArray(r.versions) ? r.versions : []).map(normalizeVersionSummary).filter((v) => v.number > 0),
    locked: r.locked === true,
    lockReason: typeof r.lockReason === "string" ? r.lockReason : null,
  };
}

/** GET /api/v1/writings/{id}/versions */
export async function listWritingVersions(id: string): Promise<WritingVersionList> {
  return normalizeWritingVersionList(await apiFetch<unknown>(`/api/v1/writings/${encodeURIComponent(id)}/versions`));
}

/** GET /api/v1/writings/{id}/versions/{n} */
export async function getWritingVersion(id: string, n: number): Promise<WritingVersion> {
  const raw = await apiFetch<Record<string, unknown>>(`/api/v1/writings/${encodeURIComponent(id)}/versions/${n}`);
  return { ...normalizeVersionSummary(raw), body: typeof raw.body === "string" ? raw.body : "" };
}

/** POST /api/v1/writings/{id}/revise — 修改. 403 writing_locked when locked. */
export async function reviseWriting(id: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}/revise`, { method: "POST" });
}

/** POST /api/v1/writings/{id}/revise/discard — 放弃修改. */
export async function discardWritingRevision(id: string): Promise<Writing> {
  return apiFetch<Writing>(`/api/v1/writings/${encodeURIComponent(id)}/revise/discard`, { method: "POST" });
}
