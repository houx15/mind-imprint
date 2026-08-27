import { useCallback, useEffect, useMemo, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { Button, Icon } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { StudioCardSheet } from "@/studio/StudioCardSheet";
import { Sparkles } from "lucide-react";
import { ApiError } from "../api/client";
import { getWriting, isWritingFinished, type Writing } from "../api/writings";
import {
  listWritingMessages,
  getWritingOutline,
  getWritingSnippets,
  getWritingDraft,
  listWritingCards,
  postWritingTurn,
  setWritingStage,
  activateWritingCard,
  skipWritingCard,
  submitWritingCard,
  summonWritingCard,
  type WritingOutlineItem,
  type WritingSnippet,
  type WritingDraft,
} from "../api/writingRoom";
import type { LiteCard, LiteMessage } from "../api/readingRoom";
import { liteRoutePath, navigate } from "../routing";
import { StageMap, type WritingStageKey } from "./StageMap";
import { IdeateStage } from "./IdeateStage";
import { OutlineStage } from "./OutlineStage";
import { SnippetsStage } from "./SnippetsStage";
import { ComposeStage } from "./ComposeStage";

/**
 * WritingRoomHost — the 写作 room. Unlike reading, there is no pro component
 * to mount here (see the task brief's 复用边界): `WritingBlock`/
 * `WorkspaceContainer` are module-private and cannot host independently, so
 * this file assembles the room itself out of the genuinely standalone
 * primitives — `StudioCardSheet` (the real 工具卡 renderer), `ChatLog`/
 * `Composer`/`ChatMarkdown` (the shared chat primitives), and `CARD_REGISTRY`
 * — behind a layout written fresh for lite (no top title bar / stage bar,
 * per the product's own instruction that lite's writing page should not
 * look like pro's).
 *
 * Layout: a stage map up top (always clickable — 铁律②, never a gate), a
 * main panel on the left that changes with `writing.stage`, and a coach
 * rail on the right (dialogue + card shelf) that is THE SAME regardless of
 * stage — "talk first" is not something a stage switch should ever hide.
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
      cards: LiteCard[];
    };

const EMPTY_DRAFT: WritingDraft = { body: "", updatedAt: null };

// Mirrors writing_lens.go's `writingDeckIDs` — the fixed set of writing-room
// tool cards. Growing the deck server-side (adding an id there) means
// growing this list too; there is no endpoint that reports it back.
const WRITING_DECK = ["argument-map", "concession", "pee", "toulmin"];

export function WritingRoomHost({ writingId }: { writingId: string }) {
  const [state, setState] = useState<LoadState>({ phase: "loading" });
  const [roomError, setRoomError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [draftText, setDraftText] = useState("");
  const [hangingCard, setHangingCard] = useState<LiteCard | null>(null);
  const [cardNote, setCardNote] = useState<string | null>(null);

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
        const [messages, outline, snippets, draft, cards] = await Promise.all([
          listWritingMessages(writingId).catch(() => [] as LiteMessage[]),
          getWritingOutline(writingId).catch(() => [] as WritingOutlineItem[]),
          getWritingSnippets(writingId).catch(() => [] as WritingSnippet[]),
          getWritingDraft(writingId).catch(() => EMPTY_DRAFT),
          listWritingCards(writingId).catch(() => [] as LiteCard[]),
        ]);
        if (!cancelled) setState({ phase: "ready", writing, messages, outline, snippets, draft, cards });
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

  // A card left proposed/active from a previous session must still show up
  // after a reload — same reasoning as ReadingRoomHost/getOpenCard.
  useEffect(() => {
    if (state.phase !== "ready") return;
    const open = state.cards.find((c) => c.status === "proposed" || c.status === "active");
    setHangingCard(open ?? null);
  }, [state]);

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

  async function summon(cardId: string) {
    if (state.phase !== "ready" || hangingCard) return;
    setRoomError(null);
    try {
      const turn = await summonWritingCard(writingId, cardId);
      if (turn.reply.trim()) {
        setState((s) => (s.phase === "ready" ? { ...s, messages: [...s.messages, { seq: -3, role: "ai", content: turn.reply, createdAt: "" }] } : s));
      }
      if (turn.card) setHangingCard(turn.card);
    } catch (err) {
      setRoomError(err instanceof ApiError ? err.message : "召唤工具卡失败，请重试。");
    }
  }

  async function openHangingCard() {
    if (!hangingCard) return;
    try {
      const activated = await activateWritingCard(writingId, hangingCard.id);
      setHangingCard(activated);
    } catch (err) {
      setRoomError(err instanceof ApiError ? err.message : "打开工具卡失败，请重试。");
    }
  }

  async function skipHangingCard() {
    if (!hangingCard) return;
    try {
      await skipWritingCard(writingId, hangingCard.id);
    } catch {
      // 铁律④: a failed skip just leaves the card hanging — she can try again.
    } finally {
      setHangingCard(null);
    }
  }

  async function submitHangingCard(fieldValues: Record<string, unknown>, eventTrace: unknown[]) {
    if (!hangingCard) return;
    const spec = CARD_REGISTRY[hangingCard.cardId];
    try {
      await submitWritingCard(writingId, hangingCard.id, { fieldValues, eventTrace });
      setCardNote(spec ? `记下了——你在《${spec.name}》里的思考已经存进过程里。` : "记下了——已经存进过程里。");
    } catch (err) {
      setRoomError(err instanceof ApiError ? err.message : "提交工具卡失败，请重试。");
    } finally {
      setHangingCard(null);
    }
  }

  if (state.phase === "loading") return <Centered>正在打开这次写作…</Centered>;
  if (state.phase === "error") return <Centered>{state.message}</Centered>;
  if (state.phase === "finished") {
    return <FinishedWritingPanel writing={state.writing} draft={state.draft} onBack={() => navigate(liteRoutePath({ tab: "writings" }))} />;
  }

  const { writing } = state;

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
        <StageMap stage={writing.stage} onJump={(s) => void jumpStage(s)} />
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
          <StagePanel state={state} writingId={writingId} setState={setState} />
        </div>

        <div className="flex min-h-0 flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-3">
          <ChatLog messages={chatMessages} thinking={sending} className="min-h-0 flex-1" />
          <Composer
            value={draftText}
            onChange={setDraftText}
            onSend={() => void send()}
            state={sending ? "replying" : undefined}
            placeholder="想到什么，跟印记说说"
          />

          <CardShelf
            hangingCard={hangingCard}
            cardNote={cardNote}
            onSummon={(id) => void summon(id)}
            onOpen={() => void openHangingCard()}
            onSkip={() => void skipHangingCard()}
            onSubmit={submitHangingCard}
            writingId={writingId}
          />
        </div>
      </div>
    </div>
  );
}

function StagePanel({
  state,
  writingId,
  setState,
}: {
  state: Extract<LoadState, { phase: "ready" }>;
  writingId: string;
  setState: Dispatch<SetStateAction<LoadState>>;
}) {
  const { writing, outline, snippets, draft } = state;
  switch (writing.stage) {
    case "outline":
      return (
        <OutlineStage
          writingId={writingId}
          outline={outline}
          onSaved={(next) => setState((s) => (s.phase === "ready" ? { ...s, outline: next } : s))}
        />
      );
    case "snippets":
      return (
        <SnippetsStage
          writingId={writingId}
          lang={writing.lang}
          outline={outline}
          snippets={snippets}
          onSnippetsChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, snippets: next } : s))}
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
    case "ideate":
    default:
      return (
        <IdeateStage
          writingId={writingId}
          writing={writing}
          onChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, writing: next } : s))}
        />
      );
  }
}

function CardShelf({
  hangingCard,
  cardNote,
  onSummon,
  onOpen,
  onSkip,
  onSubmit,
  writingId,
}: {
  hangingCard: LiteCard | null;
  cardNote: string | null;
  onSummon: (cardId: string) => void;
  onOpen: () => void;
  onSkip: () => void;
  onSubmit: (fieldValues: Record<string, unknown>, eventTrace: unknown[]) => void;
  writingId: string;
}) {
  if (hangingCard) {
    const spec = CARD_REGISTRY[hangingCard.cardId];
    if (!spec) return null;
    if (hangingCard.status === "proposed") {
      return (
        <div className="rounded-mk-sm border border-mk-accent-200 bg-mk-accent-50 p-3">
          <p className="text-mk-small font-semibold text-mk-accent-700">要不要用《{spec.name}》看看？</p>
          <div className="mt-2 flex gap-2">
            <Button size="sm" onClick={onOpen}>
              打开
            </Button>
            <Button size="sm" variant="ghost" onClick={onSkip}>
              跳过这张卡
            </Button>
          </div>
        </div>
      );
    }
    return (
      <StudioCardSheet
        spec={spec}
        persistKey={`mi:litecarddraft:${writingId}:${hangingCard.cardId}`}
        onSubmit={(env) => onSubmit(env.field_values, env.event_trace)}
        onSkip={onSkip}
      />
    );
  }

  const available = WRITING_DECK.filter((id) => CARD_REGISTRY[id]);
  return (
    <div className="rounded-mk-sm border border-mk-border bg-mk-paper p-2">
      <p className="mb-1.5 flex items-center gap-1 px-0.5 text-mk-label text-mk-faint">
        <Icon icon={Sparkles} size={12} /> 工具卡 · 挑一张想清楚（你填，印记不替你写）
      </p>
      {cardNote && <p className="mb-1.5 px-0.5 text-mk-label text-mk-success">{cardNote}</p>}
      <div className="flex flex-col gap-1.5">
        {available.map((id) => {
          const spec = CARD_REGISTRY[id]!;
          return (
            <button
              key={id}
              type="button"
              onClick={() => onSummon(id)}
              title={spec.purpose}
              className="flex w-full flex-col items-start gap-0.5 rounded-mk-sm px-2.5 py-1.5 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50"
            >
              <span className="text-mk-small font-bold text-mk-ink">{spec.name}</span>
              <span className="truncate text-mk-label text-mk-faint">{spec.purpose}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
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
 * per-audience report is a later phase; until it lands this says so
 * honestly rather than promising a date.
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
