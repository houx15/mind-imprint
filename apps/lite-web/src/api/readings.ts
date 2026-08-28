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

export interface ReadingSource {
  title: string;
  sourceUrl: string;
  blocks: ReadingBlock[];
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
