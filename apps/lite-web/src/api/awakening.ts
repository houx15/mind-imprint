import { apiFetch } from "./client";

/**
 * api/awakening —— 觉醒协议。
 *
 * 形状逐字段抄自 `apps/api/internal/api/awakening.go`，对着那份 Go 源码核过。
 * 每个读取都带 `?? []` / `?? ""`：少一个字段绝不让一次渲染崩掉。
 *
 * 调用对应的六件事：
 *
 *   fetchAwakeningStatus  树上那条入口显示什么（做过没有、有没有一趟没走完）
 *   startAwakening        开一趟，或者接上没走完的那一趟
 *   saveAwakening         存一次进度 —— **每换一屏调一次**，所以一次中途
 *                         退出也留下痕迹，下次能接着走
 *   postTurn              终端里的一轮
 *   finishAwakening       走完。**这一个会慢**：服务端同步跑两次调用
 *                         （选词 + 报告），界面必须为此显示「正在生成」
 *   fetchReport           读一份已经生成的报告
 */

export type AwakeningStage =
  | "boot"
  | "world"
  | "warning"
  | "archive"
  | "deck"
  | "rejoin"
  | "observer"
  | "energy"
  | "navigator"
  | "terminal"
  | "lens"
  | "challenge"
  | "talent"
  | "report";

export type AwakeningRoute = "" | "joined" | "observer";

export interface AwakeningTurn {
  seq: number;
  nodeIndex: number;
  studentText: string;
  reply: string;
}

/** 能量卡牌那一屏存下来的东西。形状由前端定，服务端只读 `focus` / `domains`。 */
export interface EnergyProfile {
  focus?: string;
  domains?: { id: string; name: string; short: string; score: number }[];
}

/** 天赋卡牌那一屏存下来的东西。 */
export interface TalentState {
  selected?: string[];
  lanes?: { energy?: string[]; learned?: string[]; latent?: string[] };
}

export interface AwakeningRun {
  id: string;
  attemptNo: number;
  stage: AwakeningStage;
  route: AwakeningRoute;
  navigator: string;
  energyProfile: EnergyProfile;
  talent: TalentState;
  lensChoice: string;
  challengeChoice: string;
  archiveAttempts: number;
  observerQuestion: string;
  finishedAt: string;
  turns: AwakeningTurn[];
  /** 下一轮问第几个节点（从 0 起）。等于 8 表示八问问完了。 */
  nextNode: number;
  /** 终端第一屏显示的那个问题。服务端生成，第二趟起会提到她树上已有的词。 */
  openingAsk: string;
}

export interface AwakeningStatus {
  /** 只在她**走完过**至少一次时为真。中途退出不算。 */
  taken: boolean;
  /** 她还没走完的那一趟。 */
  open: AwakeningRun | null;
  /** 最近那份报告对应的 run id，用于从树上直接打开。 */
  latestReportRunId: string;
  finishedCount: number;
}

export interface TurnResult {
  /** 印记回的那一句。**空串表示这一轮没回上来**，界面照实说。 */
  reply: string;
  nodeIndex: number;
  nextNode: number;
  /** 下一轮在同一个节点换问法。界面据此不推进步骤条。 */
  retry: boolean;
  done: boolean;
  ask: string;
  failed: boolean;
}

export type Verdict = "confirm" | "grow";

export interface PlantedWord {
  interestId: string;
  zh: string;
  en: string;
  field: string;
  note: string;
  evidence: string;
  verdict: Verdict;
  strength: number;
}

export interface Driver {
  label: string;
  evidence: string;
  confidence: number;
}

export interface ReadingPick {
  slug: string;
  title: string;
  zhTitle: string;
  field: string;
  tier: number;
  why: string[];
}

export interface TalentPile {
  key: string;
  label: string;
  cards: string[];
}

export interface ReportDiff {
  stronger: string[];
  new: string[];
  previousQuestion: string;
  daysBetween: number;
}

export interface AwakeningReport {
  version: number;
  attemptNo: number;
  navigator: string;
  /** 这次动了的词。**可能是空的** —— 界面必须照实说，不补。 */
  pursuing: PlantedWord[];
  drivers: Driver[];
  /** 她自己写的那个问题，原样。 */
  question: string;
  /** 她自己写的作品设想，原样。 */
  workConcept: string;
  talent: TalentPile[];
  /** 从真实库里挑的文章。空表示库里没有对得上的。 */
  readings: ReadingPick[];
  openFields: string[];
  /** 模型写的那一句。空表示这一次没回上来，界面少显示一块。 */
  summary: string;
  diff: ReportDiff | null;
  answers: string[];
}

const EMPTY_RUN: AwakeningRun = {
  id: "",
  attemptNo: 1,
  stage: "boot",
  route: "",
  navigator: "",
  energyProfile: {},
  talent: {},
  lensChoice: "",
  challengeChoice: "",
  archiveAttempts: 0,
  observerQuestion: "",
  finishedAt: "",
  turns: [],
  nextNode: 0,
  openingAsk: "",
};

function normalizeRun(raw: Partial<AwakeningRun> | null | undefined): AwakeningRun {
  return {
    ...EMPTY_RUN,
    ...(raw ?? {}),
    energyProfile: raw?.energyProfile ?? {},
    talent: raw?.talent ?? {},
    turns: (raw?.turns ?? []).map((t) => ({
      seq: t?.seq ?? 0,
      nodeIndex: t?.nodeIndex ?? 0,
      studentText: t?.studentText ?? "",
      reply: t?.reply ?? "",
    })),
    nextNode: raw?.nextNode ?? 0,
    openingAsk: raw?.openingAsk ?? "",
  };
}

/**
 * 把一份 payload 收拾成界面可以直接渲染的样子。
 *
 * 🚨 **每一层的数组都要兜底，不只是顶层。** Go 把 nil 切片 marshal 成 `null`，
 * 而 `null.join(...)` 会让整个报告页白掉。2026-09-19 线上走查里，一趟没有长出
 * 词的重做正好产出了 `diff.stronger = null` —— 服务端那一侧已经改成空切片，
 * 但**库里已经写下的那些行改不回去**，所以这里必须挡住。
 */
function normalizeReport(raw: Partial<AwakeningReport> | null | undefined): AwakeningReport {
  const d = raw?.diff;
  return {
    version: raw?.version ?? 1,
    attemptNo: raw?.attemptNo ?? 1,
    navigator: raw?.navigator ?? "",
    pursuing: raw?.pursuing ?? [],
    drivers: raw?.drivers ?? [],
    question: raw?.question ?? "",
    workConcept: raw?.workConcept ?? "",
    talent: (raw?.talent ?? []).map((p) => ({
      key: p?.key ?? "",
      label: p?.label ?? "",
      cards: p?.cards ?? [],
    })),
    readings: (raw?.readings ?? []).map((a) => ({
      slug: a?.slug ?? "",
      title: a?.title ?? "",
      zhTitle: a?.zhTitle ?? "",
      field: a?.field ?? "",
      tier: a?.tier ?? 2,
      why: a?.why ?? [],
    })),
    openFields: raw?.openFields ?? [],
    summary: raw?.summary ?? "",
    diff: d
      ? {
          stronger: d.stronger ?? [],
          new: d.new ?? [],
          previousQuestion: d.previousQuestion ?? "",
          daysBetween: d.daysBetween ?? 0,
        }
      : null,
    answers: raw?.answers ?? [],
  };
}

export async function fetchAwakeningStatus(): Promise<AwakeningStatus> {
  const raw = await apiFetch<Partial<AwakeningStatus>>("/api/v1/awakening");
  return {
    taken: raw.taken ?? false,
    open: raw.open ? normalizeRun(raw.open) : null,
    latestReportRunId: raw.latestReportRunId ?? "",
    finishedCount: raw.finishedCount ?? 0,
  };
}

export async function startAwakening(): Promise<AwakeningRun> {
  return normalizeRun(
    await apiFetch<Partial<AwakeningRun>>("/api/v1/awakening", { method: "POST" }),
  );
}

/** 存一次进度。**整份状态都要带上** —— 服务端整行覆盖，少带一个就是清空它。 */
export async function saveAwakening(
  id: string,
  state: {
    stage: AwakeningStage;
    route: AwakeningRoute;
    navigator: string;
    energyProfile: EnergyProfile;
    talent: TalentState;
    lensChoice: string;
    challengeChoice: string;
    archiveAttempts: number;
    observerQuestion: string;
  },
): Promise<AwakeningRun> {
  return normalizeRun(
    await apiFetch<Partial<AwakeningRun>>(`/api/v1/awakening/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(state),
    }),
  );
}

export async function postTurn(id: string, text: string): Promise<TurnResult> {
  const raw = await apiFetch<Partial<TurnResult>>(
    `/api/v1/awakening/${encodeURIComponent(id)}/turn`,
    { method: "POST", body: JSON.stringify({ text }) },
  );
  return {
    reply: raw.reply ?? "",
    nodeIndex: raw.nodeIndex ?? 0,
    nextNode: raw.nextNode ?? 0,
    retry: raw.retry ?? false,
    done: raw.done ?? false,
    ask: raw.ask ?? "",
    failed: raw.failed ?? false,
  };
}

/** ⏳ 会慢：服务端同步跑选词和报告两次调用。调用处必须显示「正在生成」。 */
export async function finishAwakening(id: string): Promise<AwakeningReport> {
  const raw = await apiFetch<{ report?: Partial<AwakeningReport> }>(
    `/api/v1/awakening/${encodeURIComponent(id)}/finish`,
    { method: "POST" },
  );
  return normalizeReport(raw.report);
}

export async function fetchReport(runId: string): Promise<AwakeningReport> {
  const raw = await apiFetch<{ report?: Partial<AwakeningReport> }>(
    `/api/v1/awakening/${encodeURIComponent(runId)}/report`,
  );
  return normalizeReport(raw.report);
}

export async function shareReport(
  runId: string,
  isPublic: boolean,
): Promise<{ public: boolean; token: string }> {
  const raw = await apiFetch<{ public?: boolean; token?: string }>(
    `/api/v1/awakening/${encodeURIComponent(runId)}/share`,
    { method: "POST", body: JSON.stringify({ public: isPublic }) },
  );
  return { public: raw.public ?? false, token: raw.token ?? "" };
}
