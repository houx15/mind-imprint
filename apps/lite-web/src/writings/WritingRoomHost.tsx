import { StudentCoachHeading } from "../learning/StudentCoachHeading";
import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { countWords } from "@/workspace/blocks/wordcount";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { LiteChatMarkdown as ChatMarkdown } from "../readings/LiteChatMarkdown";
import { getWriting, isAssignedWriting, isRevising, isWritingFinished, listWritingVersions, reviseWriting, type Writing } from "../api/writings";
import { useAlive } from "../shared/useAlive";
import { useHeartbeat } from "../shared/useHeartbeat";
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
  type WritingBoardKind,
} from "../api/writingRoom";
import type { LiteMessage } from "../api/readingRoom";
import { liteRoutePath, navigate } from "../routing";
import { StageMap, type WritingStageKey } from "./StageMap";
import { EditableTitle } from "./EditableTitle";
import { AssignmentLine } from "../inbox/AssignmentLine";
import { AssignedPromptLine } from "./AssignedPromptLine";
import { WritingSetupModal } from "./WritingSetupModal";
import { PlanningView } from "./PlanningView";
import { flushPendingSaves } from "./pendingSaves";
import { SnippetsStage } from "./SnippetsStage";
import { FlowStage } from "./FlowStage";
import { ComposeStage } from "./ComposeStage";
import { apiErrorText } from "../api/errorText";
import { FinishedWritingPage } from "./FinishedWritingPage";
import { isWritingClosedError, showFinishedPage, stageAfterRevise } from "./finishedWriting";
import { coachOpeningNeeded } from "./openingRule";
import { RevisingStrip } from "./RevisingStrip";
import { RoomTeacherFeedback } from "./RoomTeacherFeedback";
import { PaneResizer } from "../projects/PaneResizer";
import { usePaneWidth } from "../projects/usePaneWidth";

/** Leave room for the manuscript and its guidance; the coach remains resizable. */
const COACH_PANE_DEFAULT = 320;
const COACH_PANE_MAX = 720;

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
  | { phase: "finished"; writing: Writing }
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
  // A teacher grading's quote, clicked in `RoomTeacherFeedback`: `text` is
  // already the RESOLVED substring of the draft (`draftQuoteMatch`, not the
  // teacher's raw quote — see that function's doc comment for why that
  // distinction matters to `ProseSurface`'s literal highlighter). Threaded
  // down to `ComposeStage`'s `pendingHighlight`, which highlights and
  // scrolls to it in the draft. A fresh object on every click, even a repeat
  // click on the same quote, so `ComposeStage`'s effect always re-fires.
  const [draftHighlight, setDraftHighlight] = useState<{ text: string } | null>(null);
  // Bumped to re-run the load effect after 修改/放弃修改 change whether the
  // writing is revising, without touching `writingId` itself.
  const [reloadNonce, setReloadNonce] = useState(0);
  // 2026-09-18「ai sidebar right side, can adjust width」：右栏宽度她自己拖，
  // 记在本机上（和项目房间那一栏各记各的）。
  const coachPane = usePaneWidth("lite:writingCoachWidth", COACH_PANE_DEFAULT);
  const workspaceRef = useRef<HTMLDivElement | null>(null);
  const reload = useCallback(() => setReloadNonce((n) => n + 1), []);

  // The report's minute count. `state.phase === "ready"` is exactly "loaded
  // and not finished" — a finished writing takes the "finished" phase below,
  // which never reaches this line — so no extra state is needed for `enabled`.
  useHeartbeat("writing", writingId, state.phase === "ready");

  useEffect(() => {
    let cancelled = false;
    setState({ phase: "loading" });
    void (async () => {
      try {
        const writing = await getWriting(writingId);
        if (isWritingFinished(writing)) {
          // Revising but past the deadline: the room would refuse every
          // write, so the page shows the latest version instead. A failed
          // versions lookup is a load error, not "not locked": opening the
          // room on a guess would send a write on mount, get 403, reload and
          // repeat.
          const locked = isRevising(writing) ? (await listWritingVersions(writingId)).locked : false;
          if (showFinishedPage(writing, locked)) {
            if (!cancelled) setState({ phase: "finished", writing });
            return;
          }
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
          message: `加载失败：${apiErrorText(err)}`,
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [writingId, reloadNonce]);

  /**
   * The coach's opening line. Fired once setup is done and the transcript
   * holds nothing from 印记 yet.
   *
   * Safe to call more than once: the endpoint replays an existing opening
   * rather than generating a second one, so a double-invoked effect or a
   * refresh mid-flight cannot produce two greetings or two charges. The local
   * guard below is about not showing a spinner twice, not about correctness.
   */
  //
  // Explicitly NOT while planning: PlanningView owns the opening on that
  // screen. Both effects live in components that mount for the same writing,
  // and the hook here runs whichever branch renders — so without this guard
  // two greetings land in the transcript. The server is idempotent, so it is
  // one charge and one stored message either way; the damage is purely that
  // she is greeted twice, which is exactly the thing the opening exists to
  // avoid feeling like.
  //
  // Also NOT for a finished writing reopened with 修改 (`coachOpeningNeeded`,
  // openingRule.ts): it already has a draft and a submitted version, and the
  // opening endpoint would make a model call for a greeting that restarts the
  // conversation.
  const openingNeeded =
    state.phase === "ready" && state.writing.stage !== "outline" && coachOpeningNeeded(state.writing, state.messages);

  /**
   * Once per writing, and the answer always lands.
   *
   * The `alive` ref replaces the per-invocation `let cancelled = false` this
   * effect used to carry. A once-latch and a `cancelled` closure disagree
   * under StrictMode's mount → cleanup → remount: the cleanup cancels pass
   * 1's closure, the latch skips pass 2, and the single in-flight reply is
   * discarded by the only closure watching it — the room then waits forever
   * on a request the server already answered. That is exactly how 段落's
   * guide box hung the writing walk on 2026-08-28; the full write-up lives in
   * `shared/useAlive.ts`. Without the latch the opposite bill arrives: two
   * concurrent `POST /opening` calls (the server's idempotency gate is a
   * read-then-write with no lock) means two model charges and two stored
   * greetings. Both guards are needed, and they must not be able to disagree.
   */
  const openedFor = useRef<string | null>(null);
  const alive = useAlive();
  useEffect(() => {
    if (!openingNeeded || openedFor.current === writingId) return;
    openedFor.current = writingId;
    setOpening(true);
    void postWritingOpening(writingId)
      .then((res) => {
        if (!alive.current) return;
        const reply = res.reply.trim();
        if (!reply) return;
        setState((s) =>
          s.phase === "ready" ? { ...s, messages: [...s.messages, { seq: -10, role: "ai", content: reply, createdAt: "" }] } : s,
        );
      })
      .catch((err: unknown) => {
        if (!alive.current) return;
        // USER RULE: an AI failure is surfaced, never masked by a canned
        // greeting. She can still type — the room is usable, just not greeted.
        setRoomError(apiErrorText(err));
      })
      .finally(() => {
        if (alive.current) setOpening(false);
      });
  }, [openingNeeded, writingId, alive]);

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
        // The deadline passed while she was revising: every write from here
        // is refused the same way, so reload straight into the locked
        // finished page rather than leave her stuck in a room that cannot
        // save anything.
        if (isWritingClosedError(err)) {
          reload();
          return;
        }
        setRoomError(apiErrorText(err));
      }
    },
    [state.phase, writingId, reload],
  );

  /**
   * 说一句话，走房间那条对话。
   *
   * 2026-09-11 从 `send()` 里抽出来，因为多了第二个调用方：**一块板摆完之后，
   * 摆的结果原样变成一条真的学生消息**，走的是和她自己打字完全同一条路
   * （阅读室那两块板从第一天起就是这么接的，所以它们不需要第二套接线）。
   *
   * `board` 只是告诉服务端「这一条是摆完一块板产生的」，好让它在那一轮的
   * 上文里加一句说明。消息本身仍然是她的话——她摆的就是她的判断。
   */
  async function say(text: string, board?: WritingBoardKind) {
    const t = text.trim();
    if (!t || sending || state.phase !== "ready") return;
    setSending(true);
    setRoomError(null);
    const optimistic: LiteMessage = { seq: -1, role: "student", content: t, createdAt: "" };
    setState((s) => (s.phase === "ready" ? { ...s, messages: [...s.messages, optimistic] } : s));
    try {
      // 🚨 先把她还没存下去的字存完，再问印记。
      // 陪练读的是服务端那一份；她敲完直接发问的时候，防抖还没到，
      // 于是它照着一份少了那句话的正文说「你缺 X」。见 pendingSaves.ts。
      await flushPendingSaves();
      const turn = await postWritingTurn(writingId, t, board);
      const reply = turn.reply.trim();
      if (reply) {
        setState((s) =>
          s.phase === "ready" ? { ...s, messages: [...s.messages, { seq: -2, role: "ai", content: reply, createdAt: "" }] } : s,
        );
      }
    } catch (err) {
      // Same reasoning as `jumpStage`: past the deadline, reload straight
      // to the locked page rather than roll the message back into a room
      // that would just refuse the retry too.
      if (isWritingClosedError(err)) {
        reload();
        return;
      }
      setRoomError(apiErrorText(err));
      setState((s) => (s.phase === "ready" ? { ...s, messages: s.messages.filter((m) => m !== optimistic) } : s));
      throw err;
    } finally {
      setSending(false);
    }
  }

  async function send() {
    const text = draftText.trim();
    if (!text || sending || state.phase !== "ready") return;
    setDraftText("");
    try {
      await say(text);
    } catch {
      // 发不出去就把她打的字还给她——这条路原来就是这么做的。
      setDraftText(text);
    }
  }

  async function changeTargetWords(next: number) {
    if (state.phase !== "ready") return;
    try {
      const wr = await setWritingTargetWords(writingId, next);
      setState((s) => (s.phase === "ready" ? { ...s, writing: wr } : s));
    } catch (err) {
      if (isWritingClosedError(err)) {
        reload();
        return;
      }
      setRoomError(apiErrorText(err));
    }
  }

  if (state.phase === "loading") return <Centered>正在打开这次写作…</Centered>;
  if (state.phase === "error") return <Centered>{state.message}</Centered>;
  if (state.phase === "finished") {
    return (
      <FinishedWritingPage
        writing={state.writing}
        onBack={() => navigate(liteRoutePath({ tab: "writings" }))}
        onRevise={async () => {
          const revised = await reviseWriting(writingId);
          // Controller ruling: 修改 always opens the compose/write view, even
          // for a writing that was finished on the 结构 stage — otherwise
          // reload() would land her in PlanningView's full-screen 结构
          // conversation instead of the room the revising strip lives in.
          const nextStage = stageAfterRevise(revised.stage);
          if (nextStage) {
            await setWritingStage(writingId, nextStage);
          }
          reload();
        }}
      />
    );
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

  /**
   * 结构 is a FULL-SCREEN planning conversation, not a panel inside the room.
   * It branches here, before the room chrome exists, because a stage bar and a
   * length countercompete for attention with the one thing this screen is for —
   * thinking. She lands back in the room the moment she chooses 去写.
   */
  if (writing.stage === "outline") {
    return (
      <PlanningView
        writing={writing}
        messages={state.messages}
        outline={state.outline}
        onRenamed={(next) => setState((s) => (s.phase === "ready" ? { ...s, writing: next } : s))}
        onMessages={(next) => setState((s) => (s.phase === "ready" ? { ...s, messages: next } : s))}
        onOutline={(next) => setState((s) => (s.phase === "ready" ? { ...s, outline: next } : s))}
        // 结构走完之后先去行文（想清楚怎么组织），再去段落。
        onDone={() => void jumpStage("flow")}
        onUpload={() => void jumpStage("draft")}
        onBack={() => navigate(liteRoutePath({ tab: "writings" }))}
        onLocked={reload}
        banner={isRevising(writing) ? <RevisingStrip writingId={writingId} onDiscarded={reload} /> : null}
      />
    );
  }

  /**
   * 行文（2026-09-20，同事的意见 4）—— 和结构那一步一样占整屏：
   * 这一步要的是「看见整篇」，挤在一个面板里就又变回一段一段看了。
   *
   * 🚨 它**不是关卡**：顶上那条导航一直点得动，结构那一步的「去写」也照旧
   * 直通段落。这一步是一条明路，不是一道门。
   */
  if (writing.stage === "flow") {
    return (
      <FlowStage
        title={writing.title}
        onJump={(stage) => void jumpStage(stage)}
        writingId={writingId}
        outline={state.outline}
        structureKey={writing.structureKey ?? ""}
        onOutline={(next) => setState((s) => (s.phase === "ready" ? { ...s, outline: next } : s))}
        onStructureKey={(next) =>
          setState((s) =>
            s.phase === "ready" ? { ...s, writing: { ...s.writing, structureKey: next } } : s,
          )
        }
        onDone={() => void jumpStage("snippets")}
        onBack={() => void jumpStage("outline")}
        onLocked={reload}
      />
    );
  }

  // 成稿 is a writing PAGE, not a panel: it wants the room's width and its own
  // paper surface, and it does its own scrolling (the prose column and the
  // rail scroll independently, which a single scrolling panel cannot do).
  // 段落 is the same since 2026-09-18: guidance column + card paper, each
  // scrolling on its own.
  const onPage = writing.stage === "draft" || writing.stage === "finished";
  const onCards = !onPage;

  const teacherFeedback = (
    <RoomTeacherFeedback
      writingId={writingId}
      stage={writing.stage}
      draftBody={state.draft.body}
      onQuote={(matchedText) => setDraftHighlight({ text: matchedText })}
    />
  );

  return (
    <div
      className={`student-writing-room mx-auto flex h-full w-full flex-col gap-4 p-4 sm:p-6 max-w-[1680px]`}
    >
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <button
            type="button"
            onClick={() => navigate(liteRoutePath({ tab: "writings" }))}
            className="text-mk-small text-mk-muted hover:text-mk-accent-700"
          >
            ← 我的写作
          </button>
          <EditableTitle
            writingId={writingId}
            title={writing.title}
            onRenamed={(next) => setState((s) => (s.phase === "ready" ? { ...s, writing: next } : s))}
            onLocked={reload}
          />
          <AssignmentLine atomId={writingId} className="mt-0.5 block text-mk-small text-mk-muted" />
          <AssignedPromptLine writing={writing} />
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <LengthMeter writing={writing} state={state} onChange={(n) => void changeTargetWords(n)} />
          <StageMap stage={writing.stage} onJump={(s) => void jumpStage(s)} />
        </div>
      </header>

      {isRevising(writing) && <RevisingStrip writingId={writingId} onDiscarded={reload} />}

      {roomError && (
        <div role="alert" className="rounded-mk-sm px-3 py-2 text-mk-small text-mk-danger" style={{ background: "var(--mk-danger-bg)" }}>
          {roomError}
          <button type="button" className="ml-3 underline" onClick={() => setRoomError(null)}>
            知道了
          </button>
        </div>
      )}

      {/* 2026-09-18：成稿那一页上，老师批改放进这一页自己的右栏；放在编辑区下面时，
          1000px 高的屏幕上正文只剩两百多像素。段落那一步仍然放在下面。 */}
      <div ref={workspaceRef} className="student-writing-workspace flex min-h-0 flex-1 flex-col gap-4 lg:flex-row">
        {/* Wrapped in its own flex column, not a bare cell: 老师批改 sits
            BELOW the stage content here, inside the same left-hand slot the
            editor already owns — never inside the 380px coach column on the
            right (`student-coach-panel`, untouched below), and never on top
            of either. At `grid-cols-1` (narrow) the two stack, so 老师批改
            lands directly under the draft/段落 content and above the coach
            panel — exactly "below the draft" on narrow screens; at the wide
            two-column width it stays in the left column, beside (never
            over) 印记's rail. */}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
          {/* `min-h-[280px]`: the 老师批改 panel
              below can grow up to `min(320px, 33vh)` when open, on a short
              viewport that would otherwise be enough to squeeze the editor
              down to nothing. This floor keeps it usable regardless. */}
          <div
            className={
              onPage
                ? "min-h-[280px] flex-1 overflow-hidden rounded-mk-md border border-mk-border bg-mk-paper"
                : onCards
                  ? "min-h-[280px] flex-1 overflow-hidden rounded-mk-md border border-mk-border bg-mk-surface"
                  : "mk-scroll min-h-[280px] flex-1 overflow-y-auto rounded-mk-md border border-mk-border bg-mk-surface p-5"
            }
          >
            <StagePanel
              state={state}
              writingId={writingId}
              setState={setState}
              onGoToStructure={() => void jumpStage("outline")}
              onGoToDraft={() => void jumpStage("draft")}
              onSay={say}
              onLocked={reload}
              pendingHighlight={draftHighlight}
              teacherFeedback={isWritingFinished(writing) && onPage ? teacherFeedback : null}
            />
          </div>
          {/* Only mounted once the essay has been finished at least once
              (`isWritingFinished` — `status` stays "finished" for the whole
              time she's revising, per `Writing.revisingAt`'s own doc
              comment) — a brand-new essay can have no sent grading, so this
              avoids firing `listWritingGradings` on every room load. */}
          {isWritingFinished(writing) && !onPage && teacherFeedback}
        </div>

        <div
          className="student-coach-panel relative flex min-h-0 shrink-0 flex-col gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-3"
          style={coachPane.desktop ? { width: coachPane.width } : undefined}
        >
          {/* 拖左边这道缝改宽度。PaneResizer 按「窗口右边到指针」算，
              这里减掉房间右边的留白，宽度才跟着指针走。 */}
          <PaneResizer
            onResize={(w) => {
              const right = workspaceRef.current?.getBoundingClientRect().right ?? window.innerWidth;
              coachPane.setWidth(Math.min(COACH_PANE_MAX, w - (window.innerWidth - right)));
            }}
            onDoubleClick={() => coachPane.setWidth(COACH_PANE_DEFAULT)}
          />
          <StudentCoachHeading />
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

  // An assigned writing's target is the teacher's: shown, never edited here
  // (the server answers 409 assigned_target_locked).
  if (isAssignedWriting(writing)) {
    return (
      <span className="rounded-mk-full border border-mk-border px-2.5 py-1 text-mk-small text-mk-secondary">
        {writing.targetWords != null ? (
          <>
            已写 <span className="font-semibold text-mk-ink">{written}</span> / 目标 {writing.targetWords}
          </>
        ) : (
          <>已写 {written}</>
        )}
      </span>
    );
  }

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
      {/* 🚨 「/ 800」单摆在那儿，读起来是**配额**，不是目标。
          2026-09-12 第二十八轮，中文那个学生停在这儿：
            「正文框已经790/800字了，按它说的挪句子肯定会超字数限制，
              不知道超了会怎样」
          没有任何东西在拦她 —— 这个数是她自己在上面设的，框上没有 maxLength，
          超了照样存。她把一个目标读成了一道门，然后不敢动。
          补一个名词就够了：目标。多写不用问谁。 */}
      {writing.targetWords != null ? (
        <>
          已写 <span className="font-semibold text-mk-ink">{written}</span> / 目标 {writing.targetWords}
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
  onGoToDraft,
  onSay,
  onLocked,
  pendingHighlight,
  teacherFeedback,
}: {
  state: Extract<LoadState, { phase: "ready" }>;
  writingId: string;
  setState: Dispatch<SetStateAction<LoadState>>;
  onGoToStructure: () => void;
  /** 去成稿。不是关卡 —— 顶上那条导航一直都能点。 */
  onGoToDraft: () => void;
  /** 一块板摆完了：把结果当成她说的一句话发出去。 */
  onSay: (text: string, board?: WritingBoardKind) => Promise<void>;
  /** The deadline passed mid-edit: every write inside 段落/成稿 reloads the
   *  room into the locked finished page — see `writeErrors.ts`. */
  onLocked: () => void;
  /** A teacher grading's quote clicked in `RoomTeacherFeedback`: only
   *  `ComposeStage` (成稿) can highlight it — 段落 has no single draft
   *  surface to point into. */
  pendingHighlight?: { text: string } | null;
  /** 老师批改, shown in 成稿's own rail. */
  teacherFeedback?: ReactNode;
}) {
  const { writing, outline, snippets, draft } = state;
  switch (writing.stage) {
    case "snippets":
      return (
        <SnippetsStage
          writingId={writingId}
          outline={outline}
          snippets={snippets}
          onSnippetsChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, snippets: next } : s))}
          lang={writing.lang}
          onGoToStructure={onGoToStructure}
          onGoToDraft={onGoToDraft}
          onSay={onSay}
          onLocked={onLocked}
        />
      );
    case "draft":
    case "finished":
      return (
        <ComposeStage
          origin={writing.origin}
          lang={writing.lang}
          writingId={writingId}
          draft={draft}
          snippets={snippets}
          onDraftChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, draft: next } : s))}
          onFinished={(w) => setState({ phase: "finished", writing: w })}
          // Naming the piece at 完成这篇 renames it for real, so the header's
          // EditableTitle must see it immediately — the same lift PlanningView
          // and the room header already do for a rename typed in place.
          onRenamed={(w) => setState((s) => (s.phase === "ready" ? { ...s, writing: w } : s))}
          onLocked={onLocked}
          pendingHighlight={pendingHighlight}
          railTop={teacherFeedback}
        />
      );
    // 'outline' (结构) never reaches here: it takes the WHOLE screen as
    // PlanningView, branched before the room layout is rendered at all. The
    // retired 'ideate' falls through to 段落 rather than to a blank panel.
    default:
      return (
        <SnippetsStage
          writingId={writingId}
          outline={outline}
          snippets={snippets}
          onSnippetsChange={(next) => setState((s) => (s.phase === "ready" ? { ...s, snippets: next } : s))}
          lang={writing.lang}
          onGoToStructure={onGoToStructure}
          onGoToDraft={onGoToDraft}
          onSay={onSay}
          onLocked={onLocked}
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
