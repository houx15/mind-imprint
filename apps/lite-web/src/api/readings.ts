import { apiFetch } from "./client";

// api/readings.ts — the lite shell's 阅读 client. Shapes read straight off
// the Go handlers (apps/api/internal/api/readings.go, reading_source.go),
// not guessed: `readingDTO` (id/title/lang/status/hasSource/createdAt/
// updatedAt/finishedAt) and `sourceDTO` (title/sourceUrl/blocks), and
// createReading's `{"id": "..."}` 201 body.

export interface Reading {
  id: string;
  title: string;
  lang: string;
  status: string;
  hasSource: boolean;
  createdAt: string;
  /** `reading.updated_at` — TITLE/status metadata only: rename and finish are
   *  the only two things that write it. NOT "when she last read this". */
  updatedAt: string;
  /** `atom.last_activity_at` — the last time she wrote anything into this
   *  reading (a turn, a card, the article, a margin note). This is what
   *  上次读到 means and what 「你有 N 篇还没读完」 orders by. Before the
   *  server grew this field both were answered by `updatedAt`, so reading an
   *  article for an hour moved neither. */
  lastActivityAt: string;
  finishedAt: string | null;
}

export interface ReadingBlock {
  id: string;
  text: string;
}

/** 正文里的一张图。分级阅读库开来的那些才有（见 apps/api/internal/library）。
 *  `url` 是已经签好的 CDN 链接，有有效期；`after` 是它跟在哪一段之后，空串
 *  表示题图，站在第一段之前。 */
export interface ReadingFigure {
  after: string;
  url: string;
  width: number;
  height: number;
  caption: string;
  /** 连标记词一起（"Photo: …" / "Graphic: …"）—— 那个词告诉读者这是照片还是
   *  编辑部画的图。 */
  credit: string;
}

/**
 * 导读：这篇在问什么、它怎么组织、哪几段承重。
 *
 * 排读法那一次调用算出来的（`apps/api/internal/api/reading_outline.go`），
 * 排读法之前整个字段不存在。
 *
 * `core` 已经是筛好的段 id 列表 —— 服务端不把三种承重的全表发过来，因为界面上
 * 只有核心段会显示东西，而在客户端再筛一遍就等于把规则写了第二份。
 */
export interface ReadingOutline {
  oneLine: string;
  shape: string;
  core: string[];
  blocks: number;
}

export interface ReadingSource {
  title: string;
  sourceUrl: string;
  blocks: ReadingBlock[];
  /** 版式。粘贴进来的阅读没有这两项，服务端整个省掉这两个字段。 */
  figures?: ReadingFigure[];
  /** 要渲染成小标题的段 id。它们仍然是段（服务端按空行切，不认识 Markdown）。 */
  headings?: string[];
  /** 导读。排读法之前没有。 */
  outline?: ReadingOutline;
}

/**
 * GET /api/v1/readings — 我的阅读.
 *
 * The server orders by when each reading last MATTERED (finished → when she
 * finished it; still open → `lastActivityAt`) and cuts to `limit`, so this is
 * genuinely the most recent N rather than the first N of everything. Asking
 * for more than the server's ceiling gets the ceiling.
 */
export async function listReadings(limit?: number): Promise<Reading[]> {
  const query = limit ? `?limit=${encodeURIComponent(String(limit))}` : "";
  const raw = await apiFetch<{ readings: Reading[] }>(`/api/v1/readings${query}`);
  return raw.readings;
}

/** POST /api/v1/readings — creates an empty reading atom, returns its id.
 * `title` may be blank (the server falls back to 未命名阅读). */
export async function createReading(input: { title?: string; lang?: string }): Promise<{ id: string }> {
  return apiFetch<{ id: string }>("/api/v1/readings", {
    method: "POST",
    body: JSON.stringify({ title: input.title ?? "", lang: input.lang ?? "zh" }),
  });
}

/** GET /api/v1/readings/{id} */
export async function getReading(id: string): Promise<Reading> {
  return apiFetch<Reading>(`/api/v1/readings/${encodeURIComponent(id)}`);
}

/** GET /api/v1/readings/{id}/source — 404 when no article has been pasted
 * yet, which is a RECOVERABLE state (the room offers the paste box for this
 * same reading), not a failure. Callers must tell the two apart by
 * `ApiError.status`. */
export async function getReadingSource(id: string): Promise<ReadingSource> {
  return apiFetch<ReadingSource>(`/api/v1/readings/${encodeURIComponent(id)}/source`);
}

/** PUT /api/v1/readings/{id}/source — full-replace; either `text` (pasted
 * body) or `url` (server-side fetch) must be supplied. */
export async function putReadingSource(
  id: string,
  input: { title?: string; text?: string; url?: string },
): Promise<ReadingSource> {
  return apiFetch<ReadingSource>(`/api/v1/readings/${encodeURIComponent(id)}/source`, {
    method: "PUT",
    body: JSON.stringify({ title: input.title ?? "", text: input.text ?? "", url: input.url ?? "" }),
  });
}

/** POST /api/v1/readings/{id}/source/file — multipart upload of a DOCX or
 * PDF (field name "file", ≤30MB). The server extracts the text and lands it
 * through the SAME storage call as paste/URL, so the response is the same
 * `sourceDTO` and nothing downstream can tell how the article arrived.
 *
 * The extracted DOCUMENT's own title wins server-side — an uploaded file
 * carries a real title and the student typed hers before she knew what the
 * file contained. Callers that want hers to survive must `renameReading`
 * afterwards. */
export async function uploadReadingSourceFile(id: string, file: File): Promise<ReadingSource> {
  const form = new FormData();
  form.append("file", file);
  return apiFetch<ReadingSource>(`/api/v1/readings/${encodeURIComponent(id)}/source/file`, {
    method: "POST",
    body: form,
  });
}

/** PATCH /api/v1/readings/{id} — renames the reading. Title must be
 * non-empty (the server 400s on blank). */
export async function renameReading(id: string, title: string): Promise<Reading> {
  return apiFetch<Reading>(`/api/v1/readings/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ title }),
  });
}

/** GET /api/v1/readings/{id}/takeaway — 「我的收获」. Never 404s: a reading
 * with nothing written yet answers 200 with an empty `text`. */
export async function getReadingTakeaway(id: string): Promise<{ text: string; updatedAt: string }> {
  return apiFetch<{ text: string; updatedAt: string }>(
    `/api/v1/readings/${encodeURIComponent(id)}/takeaway`,
  );
}
