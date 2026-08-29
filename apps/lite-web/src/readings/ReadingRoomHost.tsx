import { useCallback, useEffect, useMemo, useState } from "react";
import type { Anchor, MaterialSource } from "@mind-imprint/contracts";
import { Button, Input, Textarea } from "@/ui";
import { ReadingRoom } from "./ReadingRoom";
import { ApiError } from "../api/client";
import {
  getReading,
  getReadingSource,
  getReadingTakeaway,
  putReadingSource,
  type Reading,
  type ReadingSource,
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
import { useHeartbeat } from "../shared/useHeartbeat";
import { ReadingPlanRail } from "./ReadingPlanRail";
import { ReadingQuestions } from "./ReadingQuestions";
import { liteRoutePath, navigate } from "../routing";

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
        // trip on every open whose answer nothing read. The current-step
        // indicator that lands in that slot is built from the PLAN, not from
        // the brief.
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
          message: err instanceof ApiError ? err.message : "这次阅读暂时打不开，刷新一下再试试。",
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

      {/* 带读进度. STATUS ONLY — the conversation that used to live here moved
          into the room's own coach column, because two AI chat boxes on one
          screen read as two AIs. Nothing in this rail is clickable: 印记 moves
          her between steps. */}
      <aside className="hidden w-[262px] shrink-0 flex-col overflow-y-auto border-r border-mk-border bg-mk-paper p-4 lg:flex">
        <ReadingPlanRail tasks={plan?.tasks ?? []} />
      </aside>

      <div className="min-w-0 flex-1">

      <ReadingRoom
        readingId={readingId}
        source={source!}
        api={api}
        onBack={() => navigate(liteRoutePath({ tab: "readings" }))}
        // 带读. The plan is loaded here because the rail beside the article
        // reads the same list; the conversation that advances it lives inside
        // the room.
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
 * FinishedReadingPanel — what a finished reading opens into.
 *
 * READ-ONLY BY CONSTRUCTION. There is no room here, so there is no way to
 * summon another lens, write another note, or re-finish it: the state is
 * terminal and the surface has nothing that could change it. What she gets
 * instead is the thing the reading produced — 我的收获, in her own words —
 * and, below it via `ReportPanel`, the end-of-session report.
 */
function FinishedReadingPanel({
  reading,
  takeaway,
  onBack,
}: {
  reading: Reading;
  takeaway: string;
  onBack: () => void;
}) {
  const day = shortDay(reading.finishedAt ?? reading.updatedAt);
  return (
    <div className="mx-auto flex w-full max-w-[680px] flex-col gap-6 px-6 py-14">
      <div className="flex flex-col gap-3">
        <span
          className="w-fit rounded-mk-full px-2.5 py-1 text-mk-label text-mk-success"
          style={{ background: "var(--mk-success-bg)" }}
        >
          已完成
        </span>
        <h1 className="text-mk-display text-mk-ink">{reading.title}</h1>
        {day && <p className="text-mk-small text-mk-muted">完成于 {day}</p>}
      </div>

      <div className="rounded-mk-md border border-mk-border bg-mk-surface p-6 shadow-mk-sm">
        <h2 className="text-mk-label text-mk-faint">我的收获</h2>
        <p className="mt-3 whitespace-pre-wrap text-mk-body-lg text-mk-ink">
          {takeaway.trim() || "这次阅读没有留下收获记录。"}
        </p>
      </div>

      <ReadingQuestions readingId={reading.id} />

      <ReportPanel kind="reading" atomId={reading.id} />

      <div>
        <Button variant="secondary" onClick={onBack}>
          回到阅读
        </Button>
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
      setError(err instanceof ApiError ? err.message : "保存失败，请重试。");
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
    origin: "",
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
