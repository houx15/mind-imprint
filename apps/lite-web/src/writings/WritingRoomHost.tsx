import { useCallback, useEffect, useMemo, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { Button } from "@/ui";
import { countWords } from "@/workspace/blocks/wordcount";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { ApiError } from "../api/client";
import { getWriting, isWritingFinished, type Writing } from "../api/writings";
import {
  listWritingMessages,
  getWritingOutline,
  getWritingSnippets,
  getWritingDraft,
  postWritingTurn,
  postWritingOpening,
  setWritingStage,
  setWritingTargetWords,
  type WritingOutlineItem,
  type WritingSnippet,
  type WritingDraft,
} from "../api/writingRoom";
import type { LiteMessage } from "../api/readingRoom";
import { liteRoutePath, navigate } from "../routing";
import { StageMap, type WritingStageKey } from "./StageMap";
import { WritingSetupModal } from "./WritingSetupModal";
import { StructureStage } from "./StructureStage";
import { SnippetsStage } from "./SnippetsStage";
import { ComposeStage } from "./ComposeStage";

/**
 * WritingRoomHost — the 写作 room.
 *
 * Reshaped on 2026-08-27 after walking it on production. Three things were
 * wrong, and all three were about guidance rather than features:
 *
 *   1. **The room was silent.** Her opening sentence sat alone in the rail and
 *      the coach said nothing at all until she spoke first. Fixed by
 *      `postWritingOpening` — 印记 speaks first, right after setup.
 *   2. **目标字数 looked broken.** The PUT was always a 200, but nothing
 *      confirmed it and no screen ever showed the number again, so from her
 *      side it did nothing. It now lives in the header as a live counter
 *      (`已写 320 / 800`), which is both the confirmation and the point.
 *   3. **The tool cards were parked on screen permanently**, four of them,
 *      pushed at her whether or not anything needed them. They are gone
 *      entirely — pro's own writing surface barely used them, and a student
 *      stuck on a paragraph wants a question, not a form. What she gets
 *      instead is the guiding box, on the block she is actually stuck on.
 *
 * Layout: stage map + length counter up top, a main panel on the left that
 * changes with `writing.stage`, and a coach rail on the right that is THE
 * SAME regardless of stage — "talk first" is not something a stage switch
 * should ever hide.
 */

type LoadState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "finished"; writing: Writing; draft: WritingDraft }
  | {
      phase: "ready";
      writing: Writing;
      messages: LiteMessage[];
      outline: WritingOutlineItem[];
      snippets: WritingSnippet[];
      draft: WritingDraft;
    };

const EMPTY_DRAFT: WritingDraft = { body: "", updatedAt: null };

export function WritingRoomHost({ writingId }: { writingId: string }) {
  const [state, setState] = useState<LoadState>({ phase: "loading" });
  const [roomError, setRoomError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [draftText, setDraftText] = useState("");
  /**
   * Tracked separately from `sending` on purpose. Both put the thinking
   * indicator in the transcript, but only `sending` locks the composer.
   * Reusing `sending` for the opening would disable her input for the whole
   * first second of the room — locking her out at the exact moment the room
   * is supposed to feel welcoming. She can always type; 印记 catching up is
   * 印记's problem.
   */
  const [opening, setOpening] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setState({ phase: "loading" });
    void (async () => {
      try {
        const writing = await getWriting(writingId);
        if (isWritingFinished(writing)) {
          const draft = await getWritingDraft(writingId).catch(() => EMPTY_DRAFT);
          if (!cancelled) setState({ phase: "finished", writing, draft });
          return;
        }
        const [messages, outline, snippets, draft] = await Promise.all([
          listWritingMessages(writingId).catch(() => [] as LiteMessage[]),
          getWritingOutline(writingId).catch(() => [] as WritingOutlineItem[]),
          getWritingSnippets(writingId).catch(() => [] as WritingSnippet[]),
          getWritingDraft(writingId).catch(() => EMPTY_DRAFT),
        ]);
        if (!cancelled) setState({ phase: "ready", writing, messages, outline, snippets, draft });
      } catch (err) {
        if (cancelled) return;
        setState({
          phase: "error",
          message: err instanceof ApiError ? err.message : "这次写作暂时打不开，刷新一下再试试。",
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [writingId]);

  /**
   * The coach's opening line. Fired once setup is done and the transcript
   * holds nothing from 印记 yet.
   *
   * Safe to call more than once: the endpoint replays an existing opening
   * rather than generating a second one, so a double-invoked effect or a
   * refresh mid-flight cannot produce two greetings or two charges. The local
   * guard below is about not showing a spinner twice, not about correctness.
   */
  const openingNeeded =
    state.phase === "ready" && state.writing.setupAt !== null && !state.messages.some((m) => m.role === "ai");

  useEffect(() => {
    if (!openingNeeded) return;
    let cancelled = false;
    setOpening(true);
    void postWritingOpening(writingId)
      .then((res) => {
        if (cancelled) return;
        const reply = res.reply.trim();
        if (!reply) return;
        setState((s) =>
          s.phase === "ready" ? { ...s, messages: [...s.messages, { seq: -10, role: "ai", content: reply, createdAt: "" }] } : s,
        );
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        // USER RULE: an AI failure is surfaced, never masked by a canned
        // greeting. She can still type — the room is usable, just not greeted.
        setRoomError(err instanceof ApiError ? err.message : "印记这次没接上，你可以直接开始说。");
      })
      .finally(() => {
        if (!cancelled) setOpening(false);
      });
    return () => {
      cancelled = true;
    };
  }, [openingNeeded, writingId]);

  const chatMessages: ChatMessage[] = useMemo(() => {
    if (state.phase !== "ready") return [];
    return state.messages
      .filter((m) => m.role === "student" || m.role === "ai")
      .map((m) => ({
        id: `m${m.seq}`,
        role: m.role === "ai" ? "assistant" : "student",
        node: m.role === "ai" ? <ChatMarkdown text={m.content} /> : m.content,
      }));
  }, [state]);

  const jumpStage = useCallback(
    async (stage: WritingStageKey) => {
      if (state.phase !== "ready") return;
      try {
        const next = await setWritingStage(writingId, stage);
        setState((s) => (s.phase === "ready" ? { ...s, writing: next } : s));
      } catch (err) {
        setRoomError(err instanceof ApiError ? err.message : "切换阶段失败，请重试。");
      }
    },
    [state.phase, writingId],
  );

  async function send() {
    const text = draftText.trim();
    if (!text || sending || state.phase !== "ready") return;
    setDraftText("");
    setSending(true);
    setRoomError(null);
    const optimistic: LiteMessage = { seq: -1, role: "student", content: text, createdAt: "" };
    setState((s) => (s.phase === "ready" ? { ...s, messages: [...s.messages, optimistic] } : s));
    try {
      const turn = await postWritingTurn(writingId, text);
      const reply = turn.reply.trim();
      if (reply) {
        setState((s) =>
          s.phase === "ready" ? { ...s, messages: [...s.messages, { seq: -2, role: "ai", content: reply, createdAt: "" }] } : s,
        );
      }
    } catch (err) {
      setRoomError(err instanceof ApiError ? err.message : "印记暂时没接上，请重试。");
      setState((s) => (s.phase === "ready" ? { ...s, messages: s.messages.filter((m) => m !== optimistic) } : s));
      setDraftText(text);
    } finally {
      setSending(false);
    }
  }

  async function changeTargetWords(next: number) {
    if (state.phase !== "ready") return;
    try {
      const wr = await setWritingTargetWords(writingId, next);
      setState((s) => (s.phase === "ready" ? { ...s, writing: wr } : s));
    } catch (err) {
      setRoomError(err instanceof ApiError ? err.message : "改目标字数失败，请重试。");
    }
  }

  if (state.phase === "loading") return <Centered>正在打开这次写作…</Centered>;
  if (state.phase === "error") return <Centered>{state.message}</Centered>;
  if (state.phase === "finished") {
    return <FinishedWritingPanel writing={state.writing} draft={state.draft} onBack={() => navigate(liteRoutePath({ tab: "writings" }))} />;
  }

  const { writing } = state;

  // The setup dialog gates the room on first open only — `setupAt` is stamped
  // once and never cleared, and migration 0100 back-filled it for every
  // writing that predates the dialog, so nobody mid-piece gets ambushed.
  if (writing.setupAt === null) {
    return (
      <>
        <div className="h-full w-full bg-mk-paper" />
        <WritingSetupModal
          writing={writing}
          onDone={(next) => setState((s) => (s.phase === "ready" ? { ...s, writing: next } : s))}
        />
      </>
    );
  }

  return (
    <div className="mx-auto flex h-full w-full max-w-[1180px] flex-col gap-4 p-4 sm:p-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <button
            type="button"
            onClick={() => navigate(liteRoutePath({ tab: "writings" }))}
            className="text-mk-small text-mk-muted hover:text-mk-accent-700"
          >
            ← 我的写作
          </button>
          <h1 className="truncate text-mk-h2 text-mk-ink">{writing.title || "还没起名字的写作"}</h1>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <LengthMeter writing={writing} state={state} onChange={(n) => void changeTargetWords(n)} />
          <StageMap stage={writing.stage} onJump={(s) => void jumpStage(s)} />
        </div>
      </header>

      {roomError && (
        <div role="alert" className="rounded-mk-sm px-3 py-2 text-mk-small text-mk-danger" style={{ background: "var(--mk-danger-bg)" }}>
          {roomError}
          <button type="button" className="ml-3 underline" onClick={() => setRoomError(null)}>
            知道了
          </button>
        </div>
      )}

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[1fr_380px]">
        <div className="mk-scroll min-h-0 overflow-y-auto rounded-mk-md border border-mk-border bg-mk-surface p-5">
          <StagePanel state={state} writingId={writingId} setState={setState} onGoToStructure={() => void jumpStage("outline")} />
        </div>

        <div className="flex min-h-0 flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-3">
          <ChatLog messages={chatMessages} thinking={sending || opening} className="min-h-0 flex-1" />
          <Composer
            value={draftText}
            onChange={setDraftText}
            onSend={() => void send()}
            state={sending ? "replying" : undefined}
            placeholder="想到什么，跟印记说说"
          />
        </div>
      </div>
    </div>
  );
}

/**
 * LengthMeter — 目标字数, finally visible.
 *
 * Counts the same material the student is actually producing: her composed
 * draft once there is one, otherwise the paragraphs she has written so far.
 * `countWords` is imported from pro's own counter rather than reimplemented —
 * an independent copy is how the two silently drift, and the last time this
 * count was hand-rolled it counted CHARACTERS and read ~5× high on English.
 */
function LengthMeter({
  writing,
  state,
  onChange,
}: {
  writing: Writing;
  state: Extract<LoadState, { phase: "ready" }>;
  onChange: (next: number) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(writing.targetWords != null ? String(writing.targetWords) : "");

  const written = useMemo(() => {
    const body = state.draft.body.trim();
    if (body) return countWords(body);
    return state.snippets.reduce((n, s) => n + countWords(s.text), 0);
  }, [state.draft.body, state.snippets]);

  function commit() {
    const n = Number(value.trim());
    setEditing(false);
    if (Number.isFinite(n) && n > 0 && n !== writing.targetWords) onChange(Math.round(n));
  }

  if (editing) {
    return (
      <span className="flex items-center gap-1.5">
        <input
          type="number"
          min={1}
          autoFocus
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === "Enter") commit();
            if (e.key === "Escape") setEditing(false);
          }}
          aria-label="目标字数"
          className="w-24 rounded-mk-xs border border-mk-input-border bg-mk-paper px-2 py-1 text-mk-small text-mk-ink outline-none focus-visible:border-mk-accent"
        />
      </span>
    );
  }

  return (
    <button
      type="button"
      onClick={() => {
        setValue(writing.targetWords != null ? String(writing.targetWords) : "");
        setEditing(true);
      }}
      title="点一下改目标字数"
      className="rounded-mk-full border border-mk-border px-2.5 py-1 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      {writing.targetWords != null ? (
        <>
          已写 <span className="font-semibold text-mk-ink">{written}</span> / {writing.targetWords}
        </>
      ) : (
        <>已写 {written} · 定个目标</>
      )}
    </button>
  );
}

function StagePanel({
  state,
  writingId,
  setState,
  onGoToStructure,
}: {
  state: Extract<LoadState, { phase: "ready" }>;
  writingId: string;
  setState: Dispatch<SetStateAction<LoadState>>;
  onGoToStructure: () => void;
}) {
  const { writing, outline, snippets, draft } = state;
  switch (writing.stage) {
    case "snippets":
      return (
        <SnippetsStage
          writingId={writingId}
          lang={writing.lang}
          outline={outline}
          snippets={snippets}
          onSnippetsChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, snippets: next } : s))}
          onGoToStructure={onGoToStructure}
        />
      );
    case "draft":
    case "finished":
      return (
        <ComposeStage
          writingId={writingId}
          draft={draft}
          onDraftChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, draft: next } : s))}
          onFinished={(w) => setState({ phase: "finished", writing: w, draft: state.draft })}
        />
      );
    // 'outline' is 结构, and it is also the fallback: the retired 'ideate'
    // lands here rather than on a blank panel.
    case "outline":
    default:
      return (
        <StructureStage
          writingId={writingId}
          lang={writing.lang}
          structureKey={writing.structureKey}
          outline={outline}
          onOutlineChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, outline: next } : s))}
          onStructureChange={(key) =>
            setState((s) => (s.phase === "ready" ? { ...s, writing: { ...s.writing, structureKey: key } } : s))
          }
        />
      );
  }
}

function Centered({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <p className="text-mk-body text-mk-muted">{children}</p>
    </div>
  );
}

/**
 * FinishedWritingPanel — the terminal view, mirroring readings/
 * ReadingRoomHost's `FinishedReadingPanel`: read-only by construction (no
 * coach input, no stage map, nothing that could change a finished piece),
 * showing the one thing the writing produced — her finished draft. The
 * per-audience report is a later phase; until it lands this says so honestly
 * rather than promising a date.
 */
function FinishedWritingPanel({
  writing,
  draft,
  onBack,
}: {
  writing: Writing;
  draft: WritingDraft;
  onBack: () => void;
}) {
  return (
    <div className="mx-auto flex w-full max-w-[680px] flex-col gap-6 px-6 py-14">
      <div className="flex flex-col gap-3">
        <span
          className="w-fit rounded-mk-full px-2.5 py-1 text-mk-label text-mk-success"
          style={{ background: "var(--mk-success-bg)" }}
        >
          已完成
        </span>
        <h1 className="text-mk-display text-mk-ink">{writing.title}</h1>
      </div>

      <div className="rounded-mk-md border border-mk-border bg-mk-surface p-6 shadow-mk-sm">
        <h2 className="text-mk-label text-mk-faint">成稿</h2>
        <p className="mt-3 whitespace-pre-wrap text-mk-body-lg text-mk-ink">{draft.body || "这篇没有留下正文。"}</p>
      </div>

      <p className="text-mk-small text-mk-muted">这篇写作的报告还在路上。你的过程都已经存好了，报告上线后会出现在这里。</p>

      <div>
        <Button variant="secondary" onClick={onBack}>
          回到写作
        </Button>
      </div>
    </div>
  );
}
