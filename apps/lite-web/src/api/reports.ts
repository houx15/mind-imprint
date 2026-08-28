import { API_BASE, ApiError, apiFetch } from "./client";

/**
 * api/reports.ts — the end-of-session report, from the client's side.
 *
 * Mirrors apps/lite-web/src/api/readingRoom.ts's own conventions: the same
 * `apiFetch` helper, the same `?? null` / `?? []` defaulting so a missing
 * field never crashes a render. `LiteReport` is copied field-for-field off
 * `liteReportDTO` (apps/api/internal/api/atom_report.go) — checked against
 * that Go source, not assumed from the spec, and it matches exactly.
 *
 * One shape mismatch worth naming: `moments` and `gains` are `omitempty` on
 * the wire — a thin session sends the field ABSENT, not `[]` — so every read
 * here defaults it back to `[]` rather than trusting it present.
 */

export type AtomKind = "reading" | "writing";

export type ReportStat = { key: string; label: string; value: number; unit: string };
export type ReportMoment = { quote: string; where: string };
export type ReportKeep = { label: string; text: string };

/** `liteReportDTO` — apps/api/internal/api/atom_report.go. */
export type LiteReport = {
  version: 1;
  kind: AtomKind;
  title: string;
  studentName: string;
  finishedAt: string;
  stats: ReportStat[];
  moments: ReportMoment[];
  keep: ReportKeep | null;
  gains: string[];
};

/** Raw wire shape of `LiteReport`, before the `?? []` defaulting below —
 *  `moments`/`gains` are `omitempty` on the Go side, so they may be absent. */
type RawLiteReport = Omit<LiteReport, "moments" | "gains"> & {
  moments?: ReportMoment[];
  gains?: string[];
};

function normalizeReport(raw: RawLiteReport): LiteReport {
  return { ...raw, moments: raw.moments ?? [], gains: raw.gains ?? [] };
}

const pathSegment: Record<AtomKind, string> = { reading: "readings", writing: "writings" };

/** `kind` → the `readings`/`writings` `{id}` root. Written once so every call
 *  site below stays a one-liner. */
function atomBase(kind: AtomKind, id: string): string {
  return `/api/v1/${pathSegment[kind]}/${encodeURIComponent(id)}`;
}

/**
 * `null` means the atom is not finished yet — a normal, expected state (the
 * server generates the report on first call and stores it), not an error.
 */
export async function getReport(kind: AtomKind, id: string): Promise<LiteReport | null> {
  const raw = await apiFetch<{ report: RawLiteReport | null }>(`${atomBase(kind, id)}/report`);
  return raw.report ? normalizeReport(raw.report) : null;
}

/** Mints (or, on a report already shared, re-returns) the public share link. */
export async function shareReport(kind: AtomKind, id: string): Promise<{ token: string; url: string }> {
  return apiFetch<{ token: string; url: string }>(`${atomBase(kind, id)}/report/share`, { method: "POST" });
}

/** Idempotent: revoking a report that was never shared is still a success. */
export async function unshareReport(kind: AtomKind, id: string): Promise<void> {
  await apiFetch<void>(`${atomBase(kind, id)}/report/share`, { method: "DELETE" });
}

/** Thrown by `getPublicReport` for an unknown or revoked token — the one case
 *  the share page must tell apart from a plain network failure, since each
 *  gets its own message ("这份记录不存在或已被收回" vs. "网络好像断开了"). */
export class PublicReportNotFoundError extends Error {
  constructor() {
    super("report_not_found");
    this.name = "PublicReportNotFoundError";
  }
}

/**
 * `GET /api/v1/public/reports/{token}` — called from a page that runs with
 * NO session (the share link is opened by anyone, no login). Deliberately
 * does NOT go through `apiFetch`: that helper always sends
 * `credentials:"include"`, which would ask the browser to attach a session
 * cookie this page never has. `credentials:"omit"` here is explicit rather
 * than relying on the fetch default, since the API can be cross-origin from
 * this page in production.
 *
 * A 404 (unknown or revoked token) raises `PublicReportNotFoundError`; any
 * other failure (network down, non-2xx, bad body) raises a plain `Error` —
 * the caller tells them apart with `instanceof`, never by string-matching a
 * message.
 */
export async function getPublicReport(token: string): Promise<LiteReport> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}/api/v1/public/reports/${encodeURIComponent(token)}`, {
      credentials: "omit",
    });
  } catch (err) {
    throw err instanceof Error ? err : new Error("网络请求失败");
  }
  if (res.status === 404) {
    throw new PublicReportNotFoundError();
  }
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      message = body?.error?.message ?? message;
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError("internal_error", message, res.status);
  }
  const body = (await res.json()) as { report: RawLiteReport };
  return normalizeReport(body.report);
}

/**
 * Posted every 60s while — and only while — the reading/writing room's tab
 * is visible. `seconds` is the client's own count of that interval; the
 * server clamps it server-side (see atom_heartbeat.go), so no clamping
 * happens here.
 */
export async function sendHeartbeat(kind: AtomKind, id: string, seconds: number): Promise<void> {
  await apiFetch<void>(`${atomBase(kind, id)}/heartbeat`, {
    method: "POST",
    body: JSON.stringify({ seconds }),
  });
}
