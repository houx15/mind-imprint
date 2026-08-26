import { useCallback, useEffect, useMemo, useState } from "react";
import type { Anchor, MaterialSource, PhaseTag } from "@mind-imprint/contracts";
import { Button, Input, Textarea } from "@/ui";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";
import type { ChatMessage } from "@/studio/reading/readingLoop";
import { LITE_READING_CAPABILITIES } from "@/rooms/capabilities";
import { ApiError } from "../api/client";
import { getReading, getReadingSource, putReadingSource, type Reading, type ReadingSource } from "../api/readings";
import {
  createReadingRoomApi,
  getReadingBrief,
  listReadingAnnotations,
  listReadingMessages,
  type LiteAnnotation,
  type LiteBrief,
  type LiteMessage,
} from "../api/readingRoom";
import { liteRoutePath, navigate } from "../routing";

/**
 * ReadingRoomHost — mounts the REAL `ReadingRoom` (apps/web) for one lite
 * reading. Nothing here re-implements the room: the host's whole job is to
 * load what the room needs, assemble its props, and hand it an `api` object
 * bound to `/api/v1/readings/{id}/…`.
 *
 * Three decisions worth knowing about:
 *
 *  - **`capabilities={LITE_READING_CAPABILITIES}`** is what removes 证据笔记
 *    and 追踪来源 — not a fork of the component. The lite edition has neither
 *    an evidence map nor an exploration graph behind those surfaces.
 *
 *  - **A reading with no article is not an error.** `createReading` and
 *    `putReadingSource` are two calls, so a dropped connection between them
 *    leaves a real reading with nothing in it. That reading is still HERS and
 *    still listed in 过往的阅读, so opening it must offer the paste box again
 *    rather than dead-ending on 「加载失败」.
 *
 *  - **The transcript is restored, not replayed.** `initialMessages` seeds the
 *    room's chat log from `GET /messages` so a reload resumes the conversation
 *    instead of greeting her as if nothing had happened. `demoMode` stays off —
 *    every write path is live.
 */

type LoadState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "needs-source"; reading: Reading }
  | { phase: "ready"; reading: Reading; source: ReadingSource; brief: LiteBrief; annotations: LiteAnnotation[]; messages: LiteMessage[] };

export function ReadingRoomHost({ readingId }: { readingId: string }) {
  const [state, setState] = useState<LoadState>({ phase: "loading" });
  const [aiError, setAiError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setState({ phase: "loading" });
    void (async () => {
      try {
        const reading = await getReading(readingId);
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
        const [brief, annotations, messages] = await Promise.all([
          getReadingBrief(readingId).catch(() => EMPTY_BRIEF),
          listReadingAnnotations(readingId).catch(() => [] as LiteAnnotation[]),
          listReadingMessages(readingId).catch(() => [] as LiteMessage[]),
        ]);
        if (!cancelled) setState({ phase: "ready", reading, source, brief, annotations, messages });
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

  const onAiError = useCallback((message: string) => setAiError(message), []);
  const api = useMemo(() => createReadingRoomApi(readingId, { onAiError }), [readingId, onAiError]);

  const source: MaterialSource | null = useMemo(() => {
    if (state.phase !== "ready") return null;
    return toMaterialSource(readingId, state.reading, state.source, state.annotations);
  }, [state, readingId]);

  const initialMessages = useMemo(
    () => (state.phase === "ready" ? toChatMessages(state.messages) : []),
    [state],
  );

  if (state.phase === "loading") {
    return <Centered>正在打开这次阅读…</Centered>;
  }
  if (state.phase === "error") {
    return <Centered>{state.message}</Centered>;
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
    <div className="relative h-full">
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
      <ReadingRoom
        // A lite reading has no project and no reference row — the atom id is
        // the only addressing unit, and the api object ignores both anyway.
        projectId={readingId}
        referenceId={readingId}
        source={source!}
        phaseTag={(state.brief.phaseTag as PhaseTag | null) ?? null}
        readingReason={state.brief.readingReason}
        readingFocus={state.brief.readingFocus}
        bib={state.source.sourceUrl ? { url: state.source.sourceUrl } : null}
        initialMessages={initialMessages.length > 0 ? initialMessages : undefined}
        api={api}
        onBack={() => navigate(liteRoutePath({ tab: "readings" }))}
        capabilities={LITE_READING_CAPABILITIES}
      />
    </div>
  );
}

const EMPTY_BRIEF: LiteBrief = { phaseTag: null, readingReason: "", readingFocus: "" };

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <p className="text-mk-body text-mk-muted">{children}</p>
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

/** The persisted transcript → the room's chat log, oldest first. */
export function toChatMessages(messages: LiteMessage[]): ChatMessage[] {
  return messages.map((m) =>
    m.role === "student"
      ? { id: `m${m.seq}`, role: "student", kind: "text", body: m.content }
      : { id: `m${m.seq}`, role: "assistant", kind: "text", body: m.content },
  );
}
