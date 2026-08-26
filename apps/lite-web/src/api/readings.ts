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
  updatedAt: string;
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

/** GET /api/v1/readings — 过往的阅读, newest-ordered by the server. */
export async function listReadings(): Promise<Reading[]> {
  const raw = await apiFetch<{ readings: Reading[] }>("/api/v1/readings");
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
