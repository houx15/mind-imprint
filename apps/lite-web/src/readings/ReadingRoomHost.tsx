import { useCallback, useEffect, useMemo, useState } from "react";
import type { Anchor, MaterialSource } from "@mind-imprint/contracts";
import { Button, Input, Textarea } from "@/ui";
import { ReadingRoom } from "./ReadingRoom";
import { ApiError } from "../api/client";
import {
  getReading,
  getReadingSource,
  getReadingTakeaway,
  reopenReading,
  putReadingSource,
  type Reading,
  type ReadingSource,
  type ReadingFigure,
  type ReadingOutline,
} from "../api/readings";
import { isFinished as isFinishedReading, shortDay } from "./ReadingHistoryPanel";
import {
  createReadingRoomApi,
  getReadingPlan,
  listReadingBlockNotes,
  listReadingBlockTools,
  listReadingAnnotations,
  listReadingCards,
  listReadingMessages,
  toReadingOutcomes,
  type LiteAnnotation,
  type LiteCard,
  type LiteMessage,
  type ReadingBlockNote,
  type ReadingBlockTool,
  type ReadingPlan,
  type ReadingTask,
} from "../api/readingRoom";
import { ReportPanel } from "../reports/ReportPanel";
import { FinishedTabs, type FinishedTab } from "../reports/FinishedTabs";
import { TranscriptView } from "../reports/TranscriptView";
import { ReadingArticle } from "../reports/ReadingArticle";
import { useHeartbeat } from "../shared/useHeartbeat";
import { ReadingQuestions } from "./ReadingQuestions";
import { liteRoutePath, navigate } from "../routing";
import { apiErrorText } from "../api/errorText";

/**
 * ReadingRoomHost — mounts lite's own `ReadingRoom` (`./ReadingRoom`) for one
 * reading. Nothing here re-implements the room: the host's whole job is to
 * load what the room needs, assemble its props, and hand it an `api` object
 * bound to `/api/v1/readings/{id}/…`.
 *
 * Three decisions worth knowing about:
 *
 *  - **The room is lite's own file now.** 证据笔记 and 追踪来源 are gone
 *    because that file never renders them — not because a `capabilities`
 *    object switches them off. The lite edition has neither an evidence map
 *    nor an exploration graph behind those surfaces.
 *
 *  - **A reading with no article is not an error.** `createReading` and
 *    `putReadingSource` are two calls, so a dropped connection between them
 *    leaves a real reading with nothing in it. That reading is still HERS and
 *    still listed in 过往的阅读, so opening it must offer the paste box again
 *    rather than dead-ending on 「加载失败」.
 *
 *  - **The transcript is restored, not replayed.** `coachMessages` seeds the
 *    conversation from `GET /messages` so a reload resumes it instead of
 *    greeting her as if nothing had happened.
 */

type LoadState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "needs-source"; reading: Reading }
  | { phase: "finished"; reading: Reading; takeaway: string }
  | {
      phase: "ready";
      reading: Reading;
      source: ReadingSource;
      annotations: LiteAnnotation[];
      messages: LiteMessage[];
      cards: LiteCard[];
    };

export function ReadingRoomHost({ readingId }: { readingId: string }) {
  const [state, setState] = useState<LoadState>({ phase: "loading" });
  const [aiError, setAiError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  // 任务清单 + 段落工具 (0101). Kept OUT of LoadState on purpose: all three
  // degrade to nothing, and a failure to load any of them must not keep her
  // out of the room — the article is the point, these are the scaffolding.
  const [plan, setPlan] = useState<ReadingPlan | null>(null);
  const [blockTools, setBlockTools] = useState<ReadingBlockTool[]>([]);
  const [blockNotes, setBlockNotes] = useState<ReadingBlockNote[]>([]);

  useEffect(() => {
    let cancelled = false;
    setState({ phase: "loading" });
    void (async () => {
      try {
        const reading = await getReading(readingId);
        // A FINISHED reading never mounts the room. Before this branch
        // existed, `/readings/:id` opened the live room whatever the reading's
        // status was — so a student could reopen a reading she had already
        // finished, summon fresh lenses on it, and finish it a second time,
        // with no 已完成 state anywhere to tell her the work was already done.
        // Finished is a terminal state, and the surface has to say so.
        if (isFinishedReading(reading)) {
          const takeaway = await getReadingTakeaway(readingId)
            .then((t) => t.text)
            .catch(() => "");
          if (!cancelled) setState({ phase: "finished", reading, takeaway });
          return;
        }
        // The source is fetched on its own so a 404 can be told apart from a
        // real failure: no article yet is a recoverable state with its own
        // surface, everything else is an error.
        let source: ReadingSource;
        try {
          source = await getReadingSource(readingId);
        } catch (err) {
          if (err instanceof ApiError && err.status === 404) {
            if (!cancelled) setState({ phase: "needs-source", reading });
            return;
          }
          throw err;
        }
        // The rest is decoration around the article — a failure on any of it
        // must not keep her out of the room, so each degrades to its empty
        // value rather than failing the load.
        // No brief fetch here: the 「你读这篇是为了」 bar it fed was write-only
        // in lite and did not survive the fork, so `GET /brief` was a round
        // trip on every open whose answer nothing read. The step surface that
        // answers 「我现在在第几步」 (the floating dial) is built from the
        // PLAN, not from the brief.
        const [annotations, messages, cards, loadedPlan, tools, notes] = await Promise.all([
          listReadingAnnotations(readingId).catch(() => [] as LiteAnnotation[]),
          listReadingMessages(readingId).catch(() => [] as LiteMessage[]),
          listReadingCards(readingId).catch(() => [] as LiteCard[]),
          getReadingPlan(readingId).catch(() => null),
          listReadingBlockTools(readingId).catch(() => ({ lang: "zh", tools: [] as ReadingBlockTool[] })),
          listReadingBlockNotes(readingId).catch(() => [] as ReadingBlockNote[]),
        ]);
        if (!cancelled) {
          setPlan(loadedPlan);
          setBlockTools(tools.tools);
          setBlockNotes(notes);
          setState({ phase: "ready", reading, source, annotations, messages, cards });
        }
      } catch (err) {
        if (cancelled) return;
        setState({
          phase: "error",
          message: apiErrorText(err),
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [readingId, reloadKey]);

  // The report's minute count. `state.phase === "ready"` is exactly "loaded
  // and not finished" — a finished reading takes the "finished" phase above,
  // which never reaches this line — so no extra state is needed for `enabled`.
  useHeartbeat("reading", readingId, state.phase === "ready");

  const onAiError = useCallback((message: string) => setAiError(message), []);
  const api = useMemo(() => createReadingRoomApi(readingId, { onAiError }), [readingId, onAiError]);

  const source: MaterialSource | null = useMemo(() => {
    if (state.phase !== "ready") return null;
    return toMaterialSource(readingId, state.reading, state.source, state.annotations);
  }, [state, readingId]);

  /**
   * 🚨 导读是**排读法那一次**写进 reading_source 的，而这个房间在她按「开始」
   * **之前**就把 source 取回来了 —— 所以取回来的那一份里没有导读，而且不会
   * 自己变。走查里就是这么发现的：线上排读法成功了，`.mk-reading-outline`
   * 却永远等不到。
   *
   * 修法是重取一次 source，而不是让排读法那个接口也发一份导读：导读是文章的
   * 属性，让两个接口都能发它，就等于把「谁说了算」变成两个人。多的这一次
   * GET 每篇文章只发生一次（拿到导读之后条件就不成立了）。
   */
  useEffect(() => {
    if (state.phase !== "ready") return;
    if (state.source.outline) return;
    if (!plan || plan.tasks.length === 0) return;
    let cancelled = false;
    getReadingSource(readingId)
      .then((source) => {
        // 没有导读就别动 state —— 排读法可能整份丢掉了（核心段全标满那种），
        // 那时候重复 setState 只会让房间白白重渲染。
        if (cancelled || !source.outline) return;
        setState((prev) => (prev.phase === "ready" ? { ...prev, source } : prev));
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [state, plan, readingId]);

  // 版式（图 + 小标题）+ 导读。图和小标题只有分级阅读库开来的阅读有；导读是
  // 排读法那一次算出来的，所以排读法之前也是空的。
  const layout = useMemo(() => {
    if (state.phase !== "ready") {
      return {
        figures: [] as ReadingFigure[],
        headings: [] as string[],
        outline: undefined as ReadingOutline | undefined,
        excerptOnly: false,
      };
    }
    return {
      figures: state.source.figures ?? [],
      headings: state.source.headings ?? [],
      outline: state.source.outline,
      excerptOnly: state.source.excerptOnly ?? false,
    };
  }, [state]);

  // Her confirmed findings, rebuilt from the card rows. Without this a reload
  // resets 阅读成果 to 0 and drops the highlights off the article — the work is
  // still in the database, it just stops being visible, which is worse than
  // losing it loudly.
  const initialOutcomes = useMemo(
    () => (state.phase === "ready" ? toReadingOutcomes(state.cards) : []),
    [state],
  );

  if (state.phase === "loading") {
    return <Centered>正在打开这次阅读…</Centered>;
  }
  if (state.phase === "error") {
    return <Centered>{state.message}</Centered>;
  }
  if (state.phase === "finished") {
    return (
      <FinishedReadingPanel
        reading={state.reading}
        takeaway={state.takeaway}
        onBack={() => navigate(liteRoutePath({ tab: "readings" }))}
        // 重新打开之后走的是**同一条加载路径**：reloadKey 变了，上面那个 effect
        // 再跑一遍，这一次 getReading 答的是 active，于是挂的是真的阅读室。
        // 没有第二份「打开房间」的代码，也就不会有第二种打开方式跟它漂移。
        onReopen={async () => {
          await reopenReading(state.reading.id);
          setReloadKey((k) => k + 1);
        }}
      />
    );
  }
  if (state.phase === "needs-source") {
    return (
      <PasteSourcePanel
        reading={state.reading}
        onSaved={() => setReloadKey((k) => k + 1)}
        onBack={() => navigate(liteRoutePath({ tab: "readings" }))}
      />
    );
  }

  return (
    <div className="relative flex h-full">
      {aiError && (
        <div
          role="alert"
          className="absolute inset-x-0 top-0 z-50 px-4 py-2 text-center text-mk-small text-mk-danger"
          style={{ background: "color-mix(in srgb, var(--mk-danger) 12%, var(--mk-surface))" }}
        >
          {aiError}
          <button type="button" className="ml-3 underline" onClick={() => setAiError(null)}>
            知道了
          </button>
        </div>
      )}

      {/* No step column any more. 带读进度 folded into `ReadingPlanDial`, which
          the room floats over its own bottom-left corner — so the 262px this
          aside used to hold went to the article and to 印记, and the step list
          stopped being `lg:`-only (it was absent on a phone entirely). */}
      <div className="min-w-0 flex-1">

      <ReadingRoom
        readingId={readingId}
        source={source!}
        // 版式走自己的两个 prop，不塞进 `MaterialSource` —— 那个类型是 pro 的
        // 契约，而图和小标题只有轻量版的分级阅读库有。
        figures={layout.figures}
        headingBlockIds={layout.headings}
        outline={layout.outline}
        excerptOnly={layout.excerptOnly}
        api={api}
        onBack={() => navigate(liteRoutePath({ tab: "readings" }))}
        // 完成这篇 lands her on the report. Re-running the load is what does
        // it: `getReading` now answers `finished`, and the branch above swaps
        // the room for `FinishedReadingPanel` — so 「a finished reading shows
        // its report」 has ONE implementation, whether she just finished it or
        // opened it a week later.
        onFinished={() => setReloadKey((k) => k + 1)}
        // 带读. The plan is loaded here because the floating dial and the
        // conversation both read the same list; the conversation that advances
        // it lives inside the room.
        tasks={plan?.tasks ?? []}
        onTasks={(tasks: ReadingTask[]) =>
          setPlan((prev) => ({
            routineKey: prev?.routineKey ?? "",
            routineName: prev?.routineName ?? "",
            tasks,
          }))
        }
        coachMessages={state.messages}
        initialOutcomes={initialOutcomes.length > 0 ? initialOutcomes : undefined}
        // 段落工具 (0101). Loaded here (one fetch per reading), rendered under
        // whichever paragraph is open inside the room.
        blockTools={blockTools}
        blockNotes={blockNotes}
        onBlockNote={(note) =>
          setBlockNotes((prev) => [
            ...prev.filter((n) => !(n.blockId === note.blockId && n.tool === note.tool)),
            note,
          ])
        }
      />
      </div>
    </div>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <p className="text-mk-body text-mk-muted">{children}</p>
    </div>
  );
}

/**
 * ReopenButton —— 「继续阅读」。
 *
 * 自己拿着 busy 和失败那句话，因为它是这一页上唯一一个**会写**的动作，而这一页
 * 其余部分是只读的。失败按 AGENTS.md §界面文案怎么写 规矩 8：动词+失败，
 * 后面接后台原话。
 */
function ReopenButton({ onReopen }: { onReopen: () => Promise<void> }) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  return (
    <>
      <Button
        variant="secondary"
        disabled={busy}
        onClick={() => {
          setErr(null);
          setBusy(true);
          void onReopen()
            .catch((e) => {
              setErr(apiErrorText(e));
              setBusy(false);
            });
        }}
      >
        {busy ? "正在打开…" : "继续阅读"}
      </Button>
      {err && <span className="text-mk-small text-mk-danger">继续阅读失败：{err}</span>}
      {/* 🚨 报告是存下来的。继续阅读之后再完成，它会按新的记录重新生成 —— 按之前
          就要说清楚（产品负责人 2026-09-17：「we need to let students know」）。 */}
      <span className="basis-full text-mk-small text-mk-muted">
        继续阅读后，再次完成时报告会按新的阅读记录重新生成，内容可能变化；已分享的链接也会显示新的内容。
      </span>
    </>
  );
}

/**
 * FinishedReadingPanel — what a finished reading opens into.
 *
 * READ-ONLY BY CONSTRUCTION. There is no room here, so there is no way to
 * summon another lens, write another note, or re-finish it: the state is
 * terminal and the surface has nothing that could change it.
 *
 * 2026-09-16：这一页从「只有报告」变成三格 —— **报告 · 对话 · 原文**。
 * 只读这一条一个字没变，三格都守着它：`TranscriptView` 里没有输入框也没有
 * 重试发送，`ReadingArticle` 在这里不标注不划句。产品负责人的原话是
 * 「they cannot go back to view their chat history. but we want that student
 * can view all history.」—— 缺的从来不是数据（`GET /readings/{id}/messages`
 * 一直都在），是没有任何界面去读它。
 *
 * 报告那一格仍然领着这一页并带着自己的宽 hero（标题、她的名字、日期），
 * 所以上面那条横条仍然只有：返回、已完成、日期、页签。
 *
 * Two blocks that used to live here are gone on purpose, and both were
 * duplicates of the report rather than losses:
 *   - the big `<h1>` title — the report's hero states it at display size, and
 *     two titles stacked 40px apart read as a rendering bug.
 *   - the 我的收获 card — `ReportView`'s own 我的收获 renders the SAME
 *     `reading_takeaway.text` verbatim (server-side it is `keep`, copied, not
 *     summarised), as the largest card on the page.
 * `takeaway` therefore no longer renders here; it stays a prop because the
 * loader already has it and the tests pin that a reading without one shows no
 * 我的收获 anywhere.
 *
 * The report is deliberately NOT wrapped in this panel's own max-width:
 * `ReportView` is self-contained (its own 1180px, its own gutters) because
 * `PublicReportPage` mounts it with no chrome at all, and a second wrapper
 * here would double the padding and cap the width twice.
 */
function FinishedReadingPanel({
  reading,
  takeaway,
  onBack,
  onReopen,
}: {
  reading: Reading;
  takeaway: string;
  onBack: () => void;
  /** 继续阅读：把这一篇重新打开，回到原来那个阅读室。 */
  onReopen: () => Promise<void>;
}) {
  const day = shortDay(reading.finishedAt ?? reading.updatedAt);
  // 默认停在报告：她刚完成时想看的是结果。回看是她第二次来才要的东西。
  const [tab, setTab] = useState<FinishedTab>("report");
  return (
    <div className="flex w-full flex-col pb-14">
      <div className="mk-rp-measure flex flex-wrap items-center gap-3 pt-8">
        <Button variant="secondary" onClick={onBack}>
          回到阅读
        </Button>
        <span
          className="rounded-mk-full px-2.5 py-1 text-mk-label text-mk-success"
          style={{ background: "var(--mk-success-bg)" }}
        >
          已完成
        </span>
        {day && <span className="text-mk-small text-mk-muted">完成于 {day}</span>}
        <ReopenButton onReopen={onReopen} />
        {/* A breadcrumb, not a heading: the report's hero states the title at
            display size, so this is the small line that keeps the page named
            when the report is still loading or failed to load. */}
        <span className="min-w-0 truncate text-mk-small text-mk-muted">{reading.title}</span>
      </div>

      <div className="mk-rp-measure pt-5">
        <FinishedTabs
          tabs={[
            { id: "report", label: "报告" },
            { id: "transcript", label: "对话" },
            { id: "source", label: "原文" },
          ]}
          active={tab}
          onPick={setTab}
        />
      </div>

      {/* 🚨 报告那一格用 `hidden` 藏，不用条件渲染拆掉。`ReportPanel` 在
          `prosePending` 时会自己补发一次请求，而那一次请求**真的在等一个旗舰
          调用**；每切一次页签就卸载重挂，等于每切一次就再买一次那通调用。
          另外两格是纯读，拆掉重挂只是多一次 GET，所以照常条件渲染。 */}
      {tab === "transcript" && <TranscriptView kind="reading" atomId={reading.id} />}
      {tab === "source" && (
        <div className="mk-rp-measure py-8">
          <ReadingArticle atomId={reading.id} defaultOpen />
        </div>
      )}

      <div hidden={tab !== "report"}>
      <ReportPanel
        kind="reading"
        atomId={reading.id}
        // Only when there is no report: the report's own 我的收获 renders this
        // exact text (server-side `keep` is the takeaway, copied verbatim), so
        // rendering both would print her 收获 twice on every finished reading.
        fallback={
          takeaway.trim() ? (
            <div className="mk-rp-measure py-8">
              <div className="rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-sm">
                <h2 className="text-mk-label text-mk-faint">我的收获</h2>
                <p className="mt-3 whitespace-pre-wrap text-mk-body-lg text-mk-ink">{takeaway}</p>
              </div>
            </div>
          ) : null
        }
      />

      <div className="mk-rp-measure">
        <ReadingQuestions readingId={reading.id} />
      </div>
      </div>
    </div>
  );
}

/**
 * PasteSourcePanel — the recovery surface for a reading whose article never
 * landed. It PUTs onto the EXISTING reading, so the orphan is repaired rather
 * than abandoned next to a freshly minted duplicate.
 */
function PasteSourcePanel({
  reading,
  onSaved,
  onBack,
}: {
  reading: Reading;
  onSaved: () => void;
  onBack: () => void;
}) {
  const [title, setTitle] = useState(reading.title === "未命名阅读" ? "" : reading.title);
  const [body, setBody] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    const text = body.trim();
    if (!text || saving) return;
    setSaving(true);
    setError(null);
    try {
      await putReadingSource(reading.id, { title: title.trim(), text });
      onSaved();
    } catch (err) {
      setError(apiErrorText(err));
      setSaving(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-5 p-8">
      <div className="flex flex-col gap-1">
        <h1 className="text-mk-h1 text-mk-ink">这次阅读还没有正文</h1>
        <p className="text-mk-body text-mk-muted">把文章粘进来，就能接着这一次读下去——不用重新开一次。</p>
      </div>
      <Input value={title} onChange={setTitle} placeholder="给这次阅读起个名字（可留空）" disabled={saving} />
      <Textarea
        value={body}
        onChange={setBody}
        placeholder="把文章正文粘贴到这里…"
        disabled={saving}
        className="min-h-[220px]"
      />
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}
      <div className="flex justify-between">
        <Button variant="ghost" onClick={onBack}>
          返回
        </Button>
        <Button onClick={save} disabled={!body.trim()} loading={saving}>
          开始阅读
        </Button>
      </div>
    </div>
  );
}

/**
 * toMaterialSource — the lite `{title, sourceUrl, blocks}` widened to the
 * `MaterialSource` the room reads. Every pro-only field is set to its honest
 * empty value: a lite reading has no dossier, no tier, no lateral-read graph,
 * and inventing one would put claims on screen with nothing behind them.
 *
 * `origin` is the exception that stopped being empty: a library article knows
 * who wrote it, so it fills the 「来源 · …」 line the room already had.
 */
export function toMaterialSource(
  readingId: string,
  reading: Reading,
  src: ReadingSource,
  annotations: LiteAnnotation[],
): MaterialSource {
  return {
    id: readingId,
    title: src.title || reading.title,
    sourceUrl: src.sourceUrl,
    kind: "article",
    // The room renders this as 「来源 · …」. Empty for a pasted article — we do
    // not know who wrote it, and the room drops the whole line rather than
    // printing 「来源 · 」 with nothing after it.
    origin: src.byline ?? "",
    blocks: src.blocks,
    locked: false,
    role: "",
    tier: "",
    takeaway: "",
    anchors: annotations.map(toAnchor).filter((a): a is Anchor => a !== null),
    timeSpentS: 0,
    lateralRead: false,
    isLateralInstrument: false,
    siftSkipped: false,
    lateralRelation: "",
    lateralJudgment: "",
  };
}

/** One stored margin note → a renderable highlight. A note whose span has no
 *  real range is DROPPED rather than shown as a zero-width mark. */
function toAnchor(a: LiteAnnotation): Anchor | null {
  const start = a.span?.start ?? 0;
  const end = a.span?.end ?? 0;
  if (end <= start) return null;
  return {
    id: a.id,
    material_id: "",
    block_id: a.blockId,
    start,
    end,
    quote: a.quote,
    dimension: "批注",
    author: "student",
    question: "",
    answer: a.note,
  };
}
