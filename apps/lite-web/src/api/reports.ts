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

/** `reportLensNote` — apps/api/internal/api/atom_report.go. What her 透镜
 *  work produced: the sentence SHE picked out of the article, and the 发现
 *  the room drew from it. Reading-kind only (absent on a writing report). */
export type ReportLensNote = { lens: string; quote: string; finding: string };

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
  lensNotes: ReportLensNote[];
};

/** Raw wire shape of `LiteReport`, before the `?? []` defaulting below —
 *  `moments`/`gains`/`lensNotes` are `omitempty` on the Go side, so they may
 *  be absent (a report generated before `lensNotes` existed always lacks it). */
type RawLiteReport = Omit<LiteReport, "moments" | "gains" | "lensNotes"> & {
  moments?: ReportMoment[];
  gains?: string[];
  lensNotes?: ReportLensNote[];
};

function normalizeReport(raw: RawLiteReport): LiteReport {
  return { ...raw, moments: raw.moments ?? [], gains: raw.gains ?? [], lensNotes: raw.lensNotes ?? [] };
}

const pathSegment: Record<AtomKind, string> = { reading: "readings", writing: "writings" };

/** `kind` → the `readings`/`writings` `{id}` root. Written once so every call
 *  site below stays a one-liner. */
function atomBase(kind: AtomKind, id: string): string {
  return `/api/v1/${pathSegment[kind]}/${encodeURIComponent(id)}`;
}

/** Wire shape of the AUTHENTICATED GET .../report response — `report` plus
 *  F2's share state (`shared`/`shareToken`), so a returning student sees a
 *  share she already made instead of the panel always starting closed. The
 *  PUBLIC payload (`getPublicReport`) never carries these two fields — see
 *  atom_report_share.go's file comment and TestPublicPayloadCarriesNothingExtra. */
type RawReportEnvelope = {
  report: RawLiteReport | null;
  shared?: boolean;
  shareToken?: string | null;
  /** 她给这次体验打的星（1–5），没打过就是 null。绝不会是 0——「没说」和
   *  「给了最低分」必须分得开。 */
  rating?: number | null;
};

/** F2: `report` plus whether this report is already shared and, if so, with
 *  which token — read by `ReportPanel` to hand `SharePanel` its starting
 *  state on mount, rather than always starting at {phase:"off"}. */
export type ReportEnvelope = {
  report: LiteReport | null;
  shareToken: string | null;
  /** The star she already gave, so the scorer at the foot of the report opens
   *  filled in instead of asking her again every time. */
  rating: number | null;
};

/**
 * `null` means the atom is not finished yet — a normal, expected state (the
 * server generates the report on first call and stores it), not an error.
 */
export async function getReport(kind: AtomKind, id: string): Promise<LiteReport | null> {
  const env = await getReportEnvelope(kind, id);
  return env.report;
}

/** Same fetch as `getReport`, but also surfaces F2's share state — see
 *  `ReportEnvelope`. */
export async function getReportEnvelope(kind: AtomKind, id: string): Promise<ReportEnvelope> {
  const raw = await apiFetch<RawReportEnvelope>(`${atomBase(kind, id)}/report`);
  return {
    report: raw.report ? normalizeReport(raw.report) : null,
    shareToken: raw.shareToken ?? null,
    rating: raw.rating ?? null,
  };
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
