import type { ReadingWord } from "./readingRoom";
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

/** Everything a heartbeat can be posted for. Wider than `AtomKind`: reports
 *  only exist for reading/writing, but the project room ticks the same clock
 *  (see `heartbeatBase`/`sendHeartbeat` below and apps/api/internal/api/
 *  pbl_heartbeat.go), so `sendHeartbeat` takes this instead of `AtomKind`. */
export type HeartbeatKind = AtomKind | "project";

export type ReportStat = { key: string; label: string; value: number; unit: string };
export type ReportMoment = { quote: string; where: string };
/** `reportKeep` — apps/api/internal/api/atom_report.go. The report's 我的收获
 *  card. `source` says WHO WROTE `text`: `"student"` when it is her own
 *  takeaway verbatim, `"coach"` when 印记 wrote it from her session because
 *  she left none (完成这篇 stopped asking for one, so `"coach"` is now the
 *  normal case). The two get different attribution lines and the line is
 *  never omitted — a generated paragraph must never read as her own words.
 *
 *  A report stored before `source` existed decodes it as absent; `?? "student"`
 *  in `normalizeReport` is the correct default, since back then a keep could
 *  only ever have been her own takeaway. */
export type ReportKeep = { label: string; text: string; source: "student" | "coach" };

/** `reportLensNote` — apps/api/internal/api/atom_report.go. What her 透镜
 *  work produced: the sentence SHE picked out of the article, and the 发现
 *  the room drew from it. Reading-kind only (absent on a writing report). */
export type ReportLensNote = { lens: string; quote: string; finding: string };

/** `reportNote` — apps/api/internal/api/atom_report.go. One margin note:
 *  `quote` is the ARTICLE's sentence she marked, `note` is HERS. The two are
 *  labelled separately wherever they render and must never be merged into one
 *  string — that distinction is R4's whole point. Reading-kind only. */
export type ReportNote = { quote: string; note: string };

/** `reportTurningPoint` — apps/api/internal/api/atom_report.go. 对话里的一处
 *  转折：她说的那一句、印记接的那一句，加上模型写的一句「这里发生了什么」。
 *
 *  🚨 `student` 和 `coach` 是服务端按编号从 atom_message 里逐字取出来的，
 *  模型碰不到它们 —— 它只回了一个编号和 `why`。所以这两句永远是真说过的话。
 *  渲染时两句必须各自标明是谁说的：这是报告上唯一同时印着她的话和印记的话的
 *  地方，混在一起就是把印记的话记在她名下。 */
export type ReportTurningPoint = { turn: number; why: string; student: string; coach: string };

/** `reportArticle` — apps/api/internal/api/atom_report.go. 阅读专属。
 *  `excerpt` 是**一段摘录**，服务端封了 200 字，永远不是全文。 */
export type ReportArticle = { sourceUrl: string; host: string; excerpt: string };

/** 阅读报告上「段落工具」那一节 —— apps/api/internal/api/atom_report_toolkit.go。
 *  全是确定性的：数的是她真的做过的事，不经过模型。 */
export type ReportToolkit = {
  tools: { label: string; blocks: number }[];
  words: ReadingWord[];
  grammar: { sentence: string; points: string[] }[];
  /** `prompt` 是 印记 那一行（工具 + 段号 + 题目），`text` 才是她写的。 */
  writings: { tool: string; prompt: string; text: string }[];
};

/** 阅读报告上「我摆的板」那一节 —— apps/api/internal/api/atom_report_boards.go。
 *  标题按体裁来（议论文是论证图，报道是事实与来源对照……）。板上的句子是
 *  **文章的原话**，摆的位置才是她的判断。 */
export type ReportBoard = {
  kind: "label" | "order";
  title: string;
  groups?: { bin: string; quotes: string[] }[];
  order?: string[];
};

/** `liteReportDTO` — apps/api/internal/api/atom_report.go. */
export type LiteReport = {
  version: 1;
  kind: AtomKind;
  title: string;
  studentName: string;
  finishedAt: string;
  /** 她完成这一篇时已经完成的篇数 —— 开场那句「第 8 篇」用的数。
   *
   *  服务端存的是**数字不是句子**（见 Go 侧 liteReportDTO.Ordinal）：报告是
   *  整块存下来的 JSON，一句话写进去就永远改不动，而人称还得按语境变 ——
   *  她自己看是「我」，公开页上访客看到的是她的名字。
   *
   *  0 = 这份报告早于这个字段（服务端 omitempty，这里 `?? 0`）。那一句整句
   *  不渲染 —— 「第 0 篇」比不渲染糟得多。 */
  ordinal: number;
  stats: ReportStat[];
  moments: ReportMoment[];
  keep: ReportKeep | null;
  gains: string[];
  lensNotes: ReportLensNote[];
  notes: ReportNote[];
  /** 对话里的转折。空数组 = 这次没挑出来（或这份报告早于这个字段）。 */
  turningPoints: ReportTurningPoint[];
  /** 我读的这篇。写作报告永远是 null。 */
  article: ReportArticle | null;
  /** 这份报告是第几版。没有 = 第 1 版。「继续阅读」再完成之后重新生成的是 2、3…… */
  revision?: number;
  /** 最近一次重新生成的时间（RFC3339）。 */
  revisedAt?: string;
  /** 段落工具上她做过的事（阅读专属，2026-09-17）。早于这个字段的报告是 null。 */
  toolkit?: ReportToolkit | null;
  /** 她摆过的板（阅读专属，2026-09-18）。早于这个字段的报告没有这一项；
   *  规范化之后是空数组，旧的夹具里整个键缺席。 */
  boards?: ReportBoard[];
  /** The finished piece, in full, HER OWN words — writing-kind only, and
   *  `""` on a writing report generated before the field existed (no
   *  backfill, same as `notes`/`lensNotes`). This is what makes a scanned
   *  share link open the thing she actually wrote, not only the numbers
   *  about it. */
  piece: string;
  /** The deterministic half of this report is stored and complete, and the one
   *  model call (金句 / 这次的收获 / the generated 我的收获) has not run yet.
   *
   *  🚨 This is what stopped 「印记正在把这次读的东西整理成一份报告，稍等一下。」
   *  from being the only thing on screen for up to two and a half minutes. The
   *  prose is an `assess`-class call (`reasoning: "max"`, 180s budget) served
   *  inline; the server now stores and returns everything else first and does
   *  that call on the FOLLOW-UP request. `ReportPanel` renders the report
   *  immediately and re-fetches once to collect the prose.
   *
   *  Absent on every report stored before the split — which is correct, since
   *  all of those already have their prose. Never present on a PUBLIC share
   *  payload (the server strips it: a visitor cannot poll an authenticated
   *  endpoint). */
  prosePending: boolean;
};

/** Raw wire shape of `LiteReport`, before the `?? []` defaulting below —
 *  `moments`/`gains`/`lensNotes`/`notes` are `omitempty` on the Go side, so
 *  they may be absent (a report generated before `notes` existed lacks it). */
type RawLiteReport = Omit<
  LiteReport,
  | "moments"
  | "gains"
  | "lensNotes"
  | "notes"
  | "keep"
  | "piece"
  | "prosePending"
  | "ordinal"
  | "turningPoints"
  | "article"
  | "toolkit"
  | "boards"
> & {
  toolkit?: Partial<ReportToolkit> | null;
  boards?: ReportBoard[];
  prosePending?: boolean;
  ordinal?: number;
  turningPoints?: ReportTurningPoint[];
  article?: ReportArticle | null;
  moments?: ReportMoment[];
  gains?: string[];
  lensNotes?: ReportLensNote[];
  notes?: ReportNote[];
  piece?: string;
  keep?: { label: string; text: string; source?: "student" | "coach" } | null;
};

function normalizeReport(raw: RawLiteReport): LiteReport {
  const { toolkit: rawToolkit, ...rest } = raw;
  return {
    ...rest,
    moments: raw.moments ?? [],
    gains: raw.gains ?? [],
    lensNotes: raw.lensNotes ?? [],
    notes: raw.notes ?? [],
    piece: raw.piece ?? "",
    ordinal: raw.ordinal ?? 0,
    turningPoints: raw.turningPoints ?? [],
    // 同 toolkit：服务端没给这个键的时候，规范化之后也不该多出一个键 ——
    // 早于这个字段的每一份旧报告（和每一份旧夹具）的形状必须原样不动。
    ...(raw.boards ? { boards: raw.boards } : {}),
    article: raw.article ?? null,
    // 只在服务端给了的时候才带这个键：早于它的报告（以及每一个旧测试夹具）
    // 规范化之后的形状一个键都不多。
    ...(rawToolkit
      ? {
          toolkit: {
            tools: rawToolkit.tools ?? [],
            words: rawToolkit.words ?? [],
            grammar: (rawToolkit.grammar ?? []).map((g) => ({ ...g, points: g.points ?? [] })),
            writings: rawToolkit.writings ?? [],
          },
        }
      : {}),
    prosePending: raw.prosePending ?? false,
    // See `ReportKeep`: a keep with no source predates the field and can only
    // have been her own takeaway.
    keep: raw.keep ? { ...raw.keep, source: raw.keep.source ?? "student" } : null,
  };
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
  includeTranscript?: boolean;
  includeToolkit?: boolean;
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
  /** 她上次有没有勾「公开我和印记的对话」。和 `shareToken` 同一个道理：不从
   *  服务端读回来，这个勾选框每次重开都从「没勾」开始，于是一个当前为真的
   *  状态在屏幕上显示成假。 */
  includeTranscript: boolean;
  /** 她上次有没有勾「公开段落工具」（阅读报告那一节，有她自己写的仿写）。 */
  includeToolkit: boolean;
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
    includeTranscript: raw.includeTranscript ?? false,
    includeToolkit: raw.includeToolkit ?? false,
    rating: raw.rating ?? null,
  };
}

/** Mints (or, on a report already shared, re-returns) the public share link.
 *
 *  `includeTranscript` 是她自己勾的那一位：对话要不要跟着这条链接一起公开。
 *  默认 false。对一条**已经存在**的链接再调一次只更新这一位 —— token 不变，
 *  因为重新发一个会悄悄弄坏她已经发出去的那条。 */
export async function shareReport(
  kind: AtomKind,
  id: string,
  opts: { includeTranscript?: boolean; includeToolkit?: boolean } = {},
): Promise<{ token: string; url: string; includeTranscript: boolean }> {
  // 🚨 两位**每次都一起发**：服务端按请求体整份覆盖，只发一位等于把另一位
  // 悄悄改回 false。
  return apiFetch<{ token: string; url: string; includeTranscript: boolean }>(
    `${atomBase(kind, id)}/report/share`,
    {
      method: "POST",
      body: JSON.stringify({
        includeTranscript: opts.includeTranscript ?? false,
        includeToolkit: opts.includeToolkit ?? false,
      }),
    },
  );
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
export type PublicTranscriptLine = { who: "student" | "coach"; text: string };

/** 公开页拿到的东西：报告，外加她**勾选过**才会有的那份对话。
 *  没勾的时候服务端连 `transcript` 这个键都不发（不是空数组），所以这里
 *  `?? []` 之后的空数组就是「她没公开对话」。 */
export type PublicReport = { report: LiteReport; transcript: PublicTranscriptLine[] };

export async function getPublicReport(token: string): Promise<PublicReport> {
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
  const body = (await res.json()) as { report: RawLiteReport; transcript?: PublicTranscriptLine[] };
  return { report: normalizeReport(body.report), transcript: body.transcript ?? [] };
}

/** `kind` → the `{readings,writings,pbl/projects}` `{id}/heartbeat` root. */
const heartbeatBase: Record<HeartbeatKind, string> = {
  reading: "/api/v1/readings",
  writing: "/api/v1/writings",
  project: "/api/v1/pbl/projects",
};

/**
 * Posted every 60s while — and only while — the reading/writing/project
 * room's tab is visible. `seconds` is the client's own count of that
 * interval; the server clamps it server-side (see atom_heartbeat.go /
 * pbl_heartbeat.go), so no clamping happens here.
 */
export async function sendHeartbeat(kind: HeartbeatKind, id: string, seconds: number): Promise<void> {
  await apiFetch<void>(`${heartbeatBase[kind]}/${encodeURIComponent(id)}/heartbeat`, {
    method: "POST",
    body: JSON.stringify({ seconds }),
  });
}
