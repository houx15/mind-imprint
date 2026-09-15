import { StudentCoachHeading } from "../learning/StudentCoachHeading";
// 🚨 React 的 PointerEvent / KeyboardEvent 起了别名：不改名就会遮住同名的 DOM
// 全局类型，而这个文件里有一个真正的 DOM `pointerdown` 监听器要用后者。
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from "react";
import type { Anchor, AnnotateState, MaterialSource, SelectionEval } from "@mind-imprint/contracts";
import { Button } from "@/ui";
import { Annotate } from "@/primitives/annotate";
import { anchorToSpan } from "@/studio/material/SourceDossier";
import { HangingCard, type HangingCardStatus, anchorBlockId } from "@/studio/reading/HangingCard";
import { ConfirmedFindingCard } from "@/studio/reading/ConfirmedFindingCard";
import { ReadingOutcomes } from "@/studio/reading/ReadingOutcomes";
import { useReadingLoop, type ReadingLoopApi, type ReadingOutcome } from "@/studio/reading/readingLoop";
import "@/studio/reading/ReadingRoom.css";
import type {
  LiteMessage,
  ReadingBlockNote,
  ReadingBlockTool,
  ReadingLensDone,
  ReadingTask,
} from "../api/readingRoom";
import type { ReadingFigure, ReadingOutline } from "../api/readings";
import { ReadingOutlineCard } from "./ReadingOutlineCard";
import { ArticleFinder } from "./ArticleFinder";
import { BlockToolsPanel } from "./BlockToolsPanel";
import { ReadingCoachPanel } from "./ReadingCoachPanel";
import { ReadingPlanDial } from "./ReadingPlanDial";
import { StepIndicator } from "./StepIndicator";

/**
 * ReadingRoom (lite) — lite's OWN reading room.
 *
 * Forked from `apps/web/src/studio/reading/ReadingRoom.tsx` (2026-08-29) as a
 * pure migration: nothing a student sees was meant to change here, only who
 * owns the file. The two editions had grown into one 1096-line component with
 * a `RoomCapabilities` object and three render-prop slots whose only reason to
 * exist was "we didn't want to fork yet" — so every lite change was a change
 * to a file pro renders.
 *
 * What that fork resolved, once and statically:
 *
 *  - **`caps` is gone.** Every `caps.*` branch was decided against
 *    `LITE_READING_CAPABILITIES` (`apps/web/src/rooms/capabilities.ts`) and
 *    the losing side deleted: no 证据笔记 (`evidenceMap`), no 追来源 /
 *    可信度 (`explorationLeads` / `credibility`), no 新的线索
 *    (`proposalImpact`), and 返回 rather than 返回工作区 (`mode`).
 *  - **The three slots are inlined.** `renderCoach` is `ReadingCoachPanel`,
 *    `renderBlockAside` is `BlockToolsPanel`, `onBlockPick` is this file's own
 *    `pickBlock`.
 *  - **`demoMode` is gone.** Lite never enabled it; every write path is live.
 *  - **The brief bar is gone.** 「你读这篇是为了」 was write-only in lite (it
 *    fed only the room composer's own `readTurn` prompt, which 带读 replaced),
 *    and the 阶段 dropdown beside it was already lite-hidden. Nothing took the
 *    slot: 「我现在在第几步」 is answered by `StepIndicator` (the row) and
 *    `ReadingPlanDial` (the floating plan).
 *
 * The 2026-08-30 redesign then moved the two columns and folded the third:
 * the article is on the LEFT, 印记 on the RIGHT and wider, and the step list
 * that used to occupy a 262px column of its own is a hover-to-expand dial in
 * the corner. The current-step ROW stayed (asked back the same day) — the dial
 * is the plan, the row is the present tense. Everything in this file below the
 * imports is that layout; the widths themselves are `.mk-lite-room` rules in
 * `src/index.css`.
 *
 * The leaf components (Annotate, HangingCard, ReadingOutcomes, useReadingLoop,
 * the stylesheet) are still IMPORTED from `apps/web` — the fork copied the
 * composition, not the parts.
 *
 * ## 2026-09-10 · 正文那一栏只放正文
 *
 * 正文上方原来有一条 52px 的横条，上面并排着：文章 / 阅读成果 两个页签、一句
 * 「点一段可拆解；划选一句可引用」的提示、「透镜库 · 11」、和「完成这篇」。
 * 四样东西在 1360px 以下会折成两三行，把正文往下压掉近百像素。整条横条现在
 * 没有了：
 *
 *  - **提示删掉。** 段落工具条和划选引用都是点了就看得见的，一句常驻的说明
 *    换不来什么，却一直占着一行。
 *  - **透镜库藏起来。** 透镜是印记递给她的教具，不是她自己该去翻的抽屉
 *    （「it is a tool called by AI instead of triggered here by student」）。
 *    `loop.summonCard` 还在，印记照常用。
 *  - **完成这篇上顶栏。** 顶栏本来就有空位，而它是一个一整篇只按一次的按钮。
 *  - **阅读成果去右栏**，和印记的对话并列成两页。左边从此永远是文章。
 *
 * 中间多了一根可以拖的分界（`mk-reading-room__split`）：她想把正文拉宽的时候
 * 就能拉宽，位置按百分比记在 localStorage 里。
 */

type AnnotateSpan = AnnotateState["spans"][number];

// Built internally from the live `useReadingLoop` state;
// `exampleBlockId`/`studentBlockId` feed `anchorBlockId` (the signature
// anchor-move) to decide which paragraph the card hangs under.
type ReadingRoomCard = {
  status: HangingCardStatus;
  cardName: string;
  exampleBlockId: string;
  studentBlockId: string | null;
  exampleWhy: string;
  eval?: SelectionEval | null;
  onStartPick: () => void;
  onConfirm: () => void;
  onRepick: () => void;
  onSkip: () => void;
  hasExample: boolean;
  // A transient "you clicked my example, not your own sentence" line — see
  // readingLoop's pickHint.
  pickHint?: string | null;
};

/** The loop's own slice, plus the one call the room drives directly. */
export type LiteReadingRoomApi = ReadingLoopApi & {
  /** 完成这篇. One call, no form — see `finishReading` below. */
  finishReading(): Promise<unknown>;
};

export type LiteReadingRoomProps = {
  /** The atom id. A lite reading has no project and no reference row — the
   *  atom IS the addressing unit. */
  readingId: string;
  source: MaterialSource;
  api: LiteReadingRoomApi;
  onBack: () => void;
  /** 完成这篇 succeeded. The host re-reads the reading and swaps the room for
   *  the report — one owner for "a finished reading shows its report". */
  onFinished: () => void;
  /** 带读 · the plan the coach is walking her through, and the transcript it
   *  resumes from. Owned by the host (the rail beside the article reads the
   *  same list), passed down because the conversation lives in here now. */
  tasks: ReadingTask[];
  onTasks: (next: ReadingTask[]) => void;
  coachMessages: LiteMessage[];
  /** Her confirmed findings, rebuilt from the persisted card rows so a reload
   *  keeps 阅读成果 and the article highlights. */
  initialOutcomes?: ReadingOutcome[];
  /** 段落工具 — the per-paragraph tools (翻译 / 关键单词 / 语法 / 写作解析) and
   *  whatever she has already run on a paragraph. */
  blockTools: ReadingBlockTool[];
  blockNotes: ReadingBlockNote[];
  onBlockNote: (note: ReadingBlockNote) => void;
  /** 正文里的图（分级阅读库开来的那些才有）。每张跟在自己那一段之后；
   *  `after` 是空串的那张是题图，摆在第一段之前。 */
  figures?: ReadingFigure[];
  /** 要渲染成小标题的段 id。 */
  headingBlockIds?: string[];
  /** 导读：这篇在问什么、它怎么组织、哪几段承重。排读法之前没有。 */
  outline?: ReadingOutline;
};

/**
 * What the 带读 conversation is handed back.
 *
 * Deliberately small: the conversation owns the talking, the room still owns
 * the article. Anything here is something a composer genuinely cannot do for
 * itself — read the sentences she picked out of the text, or know that a card
 * is currently open and typing should wait.
 */
export type ReadingCoachSlot = {
  /** A lens is open on the article: sending anything now would talk over it. */
  locked: boolean;
  /** Sentences she picked out of the article for her next message.
   *  `blockId` is which paragraph each one came from — undefined only for a
   *  legacy/degraded entry with nowhere real to point at. */
  quotes: { key: string; quote: string; blockId?: string }[];
  removeQuote: (key: string) => void;
  clearQuotes: () => void;
  /** 文章上真的有一副敞开的透镜（而不只是「这一栏暂时锁住了」）。 */
  lensOpen?: boolean;
  /** 把她送到那副敞开的透镜跟前（滚到它挂着的那一段）。 */
  locateLens?: () => void;
  /** A lens landed on the article from OUTSIDE this room's own turn/summon
   *  flow (印记 minting one mid-带读, via a different endpoint) — so the
   *  room's own card state has no way to have picked it up on its own. */
  onCardSummoned?: () => void;
};

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// 左右宽度
// ---------------------------------------------------------------------------

const SPLIT_KEY = "mk.lite.readingSplit";
/** 左栏最少占 30%。再窄，英文正文一行放不下十来个词。 */
const SPLIT_MIN = 30;
/** 最多 78%。印记那一栏还要放得下卡片和输入框。 */
const SPLIT_MAX = 78;
/** 46% —— 和这一版之前写死的那两条 fr 轨道是同一个位置。 */
const SPLIT_DEFAULT = 46;

/** 上一次她拖到哪儿。读不出（无痕窗口、禁了站点数据、缩略图截取）就退回默认
 *  值：宽度是一个便利设置，不该成为整个阅读室打不开的理由。 */
function readStoredSplit(): number {
  try {
    const raw = Number(localStorage.getItem(SPLIT_KEY));
    if (Number.isFinite(raw) && raw >= SPLIT_MIN && raw <= SPLIT_MAX) return raw;
  } catch {
    // ignore
  }
  return SPLIT_DEFAULT;
}

// The focused reading surface: 文章 on the left, 印记 | 阅读成果 on the right,
// a draggable divider between them. The article enters select-mode once a card
// is `active`; the hanging card renders from the loop's live status/eval; every
// confirmed finding accumulates in 阅读成果.
export function ReadingRoom({
  readingId,
  source,
  api,
  onBack,
  onFinished,
  tasks,
  onTasks,
  coachMessages,
  initialOutcomes,
  blockTools,
  blockNotes,
  figures,
  headingBlockIds,
  outline,
  onBlockNote,
}: LiteReadingRoomProps) {
  // DEBT: `useReadingLoop` still carries pro's signature and wants a
  // projectId. It lives under apps/web, which lite may not touch, so the
  // reading id is passed in that position — the lite api object ignores it.
  const loop = useReadingLoop(readingId, source, api, undefined, initialOutcomes);

  /**
   * 右边那一栏在放什么：印记的对话，还是这一篇已经攒下的阅读成果。
   *
   * 🚨 它原来是**左边**那一栏的页签（文章 | 阅读成果），也就是说「看成果」要
   * 用掉整块正文。报回来的原话是「make 阅读成果 a switcher of the AI bar …
   * so that article part, is only article」—— 现在左边永远是文章，右边这两页
   * 都是「关于这篇文章我做了什么」，切换成本是零。
   */
  const [coachView, setCoachView] = useState<"chat" | "outcomes">("chat");

  /**
   * 左右两栏的分界，可以拖。
   *
   * 存的是左栏占工作区的百分比。为什么是百分比而不是像素：她在 27 寸屏上拖出
   * 来的那个位置，换到笔记本上按像素还原就会把印记挤没。
   *
   * 只在 981px 以上生效 —— 以下两栏是上下堆的（共用样式表里的
   * `@media (max-width: 980px)`），一根竖着的分隔条在那里没有意义，CSS 里
   * `display:none` 把它整个拿掉。
   */
  const [leftPct, setLeftPct] = useState(readStoredSplit);
  const workspaceRef = useRef<HTMLElement | null>(null);
  const draggingRef = useRef(false);

  function commitSplit(pct: number) {
    const next = Math.min(SPLIT_MAX, Math.max(SPLIT_MIN, pct));
    setLeftPct(next);
    try {
      localStorage.setItem(SPLIT_KEY, String(Math.round(next)));
    } catch {
      // 无痕窗口 / 禁了站点数据。宽度是一个便利设置，存不下就这一次别记住，
      // 不该因此让阅读室报错。
    }
  }

  function splitFromClientX(x: number): number {
    const box = workspaceRef.current?.getBoundingClientRect();
    if (!box || box.width === 0) return leftPct;
    return ((x - box.left) / box.width) * 100;
  }

  function onSplitPointerDown(e: ReactPointerEvent<HTMLDivElement>) {
    e.preventDefault();
    draggingRef.current = true;
    e.currentTarget.setPointerCapture(e.pointerId);
  }

  function onSplitPointerMove(e: ReactPointerEvent<HTMLDivElement>) {
    if (!draggingRef.current) return;
    setLeftPct(Math.min(SPLIT_MAX, Math.max(SPLIT_MIN, splitFromClientX(e.clientX))));
  }

  function onSplitPointerUp(e: ReactPointerEvent<HTMLDivElement>) {
    if (!draggingRef.current) return;
    draggingRef.current = false;
    e.currentTarget.releasePointerCapture?.(e.pointerId);
    commitSplit(splitFromClientX(e.clientX));
  }

  /** 键盘也能调：分隔条是可聚焦的 separator，左右键一次 2%。拖拽做不到的
   *  事只有一件——用键盘的人完全够不着它。 */
  function onSplitKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (e.key === "ArrowLeft") {
      e.preventDefault();
      commitSplit(leftPct - 2);
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      commitSplit(leftPct + 2);
    } else if (e.key === "Home") {
      e.preventDefault();
      commitSplit(SPLIT_DEFAULT);
    }
  }

  /**
   * 完成这篇 — one confirm, then the report.
   *
   * There used to be a form here: 我的收获 + 新的线索, seeded from an assembled
   * draft, saved, and only then finished. It is gone.
   *
   *   > we have give abundant steps for the reading. so we don't need to ask
   *   > student to enter the form again. we should jump to the reading report
   *   > page.
   *
   * What is left is one confirm, and it stays for a reason the form was NOT
   * needed for: finishing is terminal — no more lenses, no more notes, no way
   * back — so it may not be one stray click on a button that sits in the
   * toolbar the whole time she is reading. It asks nothing of her; it only
   * makes sure she meant it.
   */
  const [confirmFinish, setConfirmFinish] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState<string | null>(null);

  async function finishReading() {
    if (finishing) return;
    setFinishing(true);
    setFinishError(null);
    try {
      await api.finishReading();
      setConfirmFinish(false);
      // The host re-reads the reading, sees `finished`, and swaps the room for
      // the report. Doing it through the host rather than navigating keeps one
      // owner of that decision — `/readings/:id` already routes a finished
      // reading to its terminal surface, and this makes the two agree.
      onFinished();
    } catch {
      setFinishError("这次没能完成，再试一下。");
    } finally {
      setFinishing(false);
    }
  }

  // Click-to-reveal: clicking a highlighted span opens its note panel
  // (dimension + answer/question) inline, right under the sentence. Only
  // takes effect outside select-mode — Annotate routes mark clicks through
  // pickSentence instead while selectMode is set, so this never fights
  // evidence-picking.
  const [activeSpanId, setActiveSpanId] = useState<string | null>(null);

  // 引用原文 (focus context) — the passages the student drag-selected for her
  // next 带读 turn. Each entry is a visible, individually-cancelable quote
  // chip, rendered by the conversation. Only meaningful while idle (a card in
  // flight repurposes the article for evidence-picking, not referencing). The
  // key is a counter so identical text can't collide.
  type QuotedRef = { key: string; blockId: string; quote: string };
  const [quoted, setQuoted] = useState<QuotedRef[]>([]);
  const selSeq = useRef(0);

  function addSelection(blockId: string, quote: string) {
    setQuoted((prev) => {
      // Skip an exact-duplicate quote (double drag on the same phrase).
      if (prev.some((q) => q.quote === quote)) return prev;
      selSeq.current += 1;
      return [...prev, { key: `sel:${selSeq.current}`, blockId, quote }];
    });
  }

  function removeQuoted(key: string) {
    setQuoted((prev) => prev.filter((q) => q.key !== key));
  }

  const articleRef = useRef<HTMLDivElement | null>(null);

  const busyOrCarded = loop.busy || loop.status !== "idle";

  // 段落工具. The paragraph whose tool bar is open, together with what the bar
  // pins itself to: the paragraph element, and the x her pointer went down at.
  const [blockAnchor, setBlockAnchor] = useState<{ id: string; el: HTMLElement; x: number } | null>(null);
  // Set when the coach chose a tool for this turn; consumed once by the panel.
  const [autoTool, setAutoTool] = useState<string | null>(null);

  // Where the last pointer press landed. `onReferenceBlock` hands over a block
  // id and nothing else, and "near my mouse" needs the mouse — so the position
  // is captured on the way down rather than threaded through the primitive.
  const pointerX = useRef(0);
  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      pointerX.current = e.clientX;
    };
    window.addEventListener("pointerdown", onDown, true);
    return () => window.removeEventListener("pointerdown", onDown, true);
  }, []);

  // 一张卡挂到正文上的时候，把右栏切回对话 —— 卡片和印记的话在同一栏里，
  // 而她可能正停在「阅读成果」那一页。左边不用管：文章现在永远在。
  useEffect(() => {
    if (loop.status !== "idle") setCoachView("chat");
  }, [loop.status]);

  const card: ReadingRoomCard | null =
    loop.status === "idle"
      ? null
      : {
          status: loop.status,
          cardName: loop.cardName,
          exampleBlockId: loop.exampleBlockId,
          studentBlockId: loop.studentSpan?.blockId ?? null,
          exampleWhy: loop.exampleWhy,
          eval: loop.eval,
          onStartPick: loop.startPick,
          onConfirm: loop.confirm,
          onRepick: loop.repick,
          onSkip: loop.skip,
          hasExample: loop.exampleBlockId !== "",
          pickHint: loop.pickHint,
        };

  // A graceful-degrade summon has no example block to hang under — and a resumed
  // card's example block_id may be stale (the extraction re-segmented, so that id
  // no longer exists among the rendered blocks). Either way, fall back to the
  // first paragraph so the card is ALWAYS visible: an invisible card whose status
  // is non-idle locks the conversation AND jams the one-active mutex, with no way
  // for the student to skip or complete it. Hands off to studentBlockId the
  // moment she picks (always a live, rendered block).
  const resolvedBlockId = card
    ? anchorBlockId(card.exampleBlockId, card.studentBlockId, card.status)
    : null;
  const cardBlockId = card
    ? resolvedBlockId && source.blocks.some((b) => b.id === resolvedBlockId)
      ? resolvedBlockId
      : source.blocks[0]?.id ?? null
    : null;

  /** 段 id → 第几段。段号是她屏幕上唯一认得的坐标 —— b3 不是，导读卡因此
   *  只说段号。找不到返回 0，调用方据此不显示那一条。 */
  function ordinalOf(blockId: string): number {
    return source.blocks.findIndex((b) => b.id === blockId) + 1;
  }

  function locateBlock(blockId: string) {
    // 左边永远是文章，所以这里不用再切视图 —— 只要滚过去。
    requestAnimationFrame(() => {
      articleRef.current?.querySelector(`[data-block-id="${blockId}"]`)?.scrollIntoView({ behavior: "smooth", block: "center" });
    });
  }

  /**
   * Scroll the article to a paragraph 印记 singled out, and open its tools.
   *
   * Reaches for the DOM rather than a ref because `data-block-id` is already
   * on every paragraph (Annotate renders it, and `locateBlock` above uses
   * exactly this).
   */
  function focusBlock(blockId: string, tool?: string) {
    const el = document.querySelector<HTMLElement>(`[data-block-id="${blockId}"]`);
    el?.scrollIntoView({ behavior: "smooth", block: "center" });
    if (el) {
      // No pointer to be near — 印记 opened this one — so the bar sits over
      // the paragraph's own left edge rather than wherever she last clicked.
      setBlockAnchor({ id: blockId, el, x: el.getBoundingClientRect().left + 140 });
    }
    // 印记 reaching for a tool is it teaching, not a suggestion she has to act
    // on — so the panel opens with that tool already running rather than
    // showing her a row of buttons and hoping she presses the right one.
    setAutoTool(tool ?? null);
  }

  /**
   * Her own click on a paragraph. In pro this gesture quotes the paragraph
   * into the composer; in lite the composer belongs to 带读, and this is the
   * better thing to spend the click on. Toggles, so a second click on the
   * paragraph she is already looking at puts the bar away.
   */
  function pickBlock(blockId: string) {
    setAutoTool(null);
    if (blockAnchor?.id === blockId) {
      setBlockAnchor(null);
      return;
    }
    const el = document.querySelector<HTMLElement>(`[data-block-id="${blockId}"]`);
    if (!el) return;
    setBlockAnchor({ id: blockId, el, x: pointerX.current });
  }

  /**
   * 印记 划过的示例句子，一旦划出来就留在文章上。
   *
   * 🚨 这是「ai划的示例句子消失」的修法。示例句原本只活在 `loop.exampleAnchor`
   * 里，而那是一个纯内存的值，两处都会把它抹掉：
   *
   *  - 共用的 `clearCard()`（`apps/web/src/studio/reading/readingLoop.ts`）在
   *    她确认或跳过之后把 `exampleAnchor` 置 null；
   *  - 服务端 `submitProjectCard` 用**她自己的**那条 anchor 覆盖了 atom_card
   *    行上的 `anchors`，所以连重新加载都救不回来。
   *
   * 于是那句「印记 当着她的面示范的那一句」在她做完之后就没了——而它恰恰是
   * 她之后回头对照「示范 vs 我自己找的」时唯一的参照。这里按 anchor id 累积，
   * 只增不减：透镜退场是 `card` 的事，不是那条划痕的事。
   */
  const [shownExamples, setShownExamples] = useState<Anchor[]>([]);
  useEffect(() => {
    const a = loop.exampleAnchor;
    if (!a) return;
    setShownExamples((prev) => (prev.some((p) => p.id === a.id) ? prev : [...prev, a]));
  }, [loop.exampleAnchor]);

  // The article's own spans = the source's persisted anchors, the confirmed
  // outcomes' spans (so findings stay highlighted after the card retires —
  // the process tree "grows"), plus every example 印记 has drawn this session
  // and the student's own live pick.
  const spans = useMemo(() => {
    const extra: Anchor[] = [];
    for (const o of loop.outcomes) {
      extra.push({
        id: o.id, material_id: source.id, block_id: o.blockId, start: o.start, end: o.end,
        quote: o.quote, dimension: o.cardId, author: "student", question: "", answer: o.finding,
      });
    }
    // 全部示例，不只是「当前这一副透镜的」——见 `shownExamples`。
    extra.push(...shownExamples);
    if (loop.studentAnchor) extra.push(loop.studentAnchor);
    return [...source.anchors, ...extra].map(anchorToSpan).filter((s): s is AnnotateSpan => s !== null);
  }, [source.anchors, source.id, loop.outcomes, shownExamples, loop.studentAnchor]);

  /**
   * 她刚做完的那副透镜，等着交给 印记。
   *
   * 由 `loop.outcomes` 长出新的一条推导，而不是去改共用的 `confirm()`：
   * 那个函数是 pro 也在用的（[[lite-must-not-break-pro]]），而「又多了一条
   * 成果」这件事在 lite 这边看得一样清楚。
   *
   * 🚨 `seenOutcomesRef` 用挂载时的条数初始化。`loop.outcomes` 在恢复一次读到
   * 一半的阅读时**本来就是非空的**，不这样做的话，每次她重新打开这篇文章，
   * 房间都会拿上次的成果再买一轮旗舰调用。
   */
  const [pendingLens, setPendingLens] = useState<ReadingLensDone | null>(null);
  const seenOutcomesRef = useRef<number | null>(null);
  useEffect(() => {
    if (seenOutcomesRef.current === null) {
      seenOutcomesRef.current = loop.outcomes.length;
      return;
    }
    if (loop.outcomes.length <= seenOutcomesRef.current) return;
    seenOutcomesRef.current = loop.outcomes.length;
    const o = loop.outcomes[loop.outcomes.length - 1];
    if (!o || !o.quote.trim()) return;
    setPendingLens({ cardName: o.cardName, quote: o.quote, finding: o.finding });
  }, [loop.outcomes]);

  // Confirmed findings, keyed by span id — clicking a finding's highlight shows
  // its full 透镜卡 recap (verdict + checks) rather than a bare note.
  const outcomeBySpanId = useMemo(() => new Map(loop.outcomes.map((o) => [o.id, o])), [loop.outcomes]);

  // 图按锚点分组。`after` 为空串的是题图，摆在正文之前；其余的插在自己那一段
  // 之后。一段可以带不止一张，所以值是数组。
  //
  // 锚点指向一个不存在的段（正文换过、库改过版）时，那张图不会消失 —— 它落到
  // 题图旁边，比在页面上凭空少一张、而且没有任何报错要好。
  const leadFigure = useMemo(
    () => (figures ?? []).find((f) => f.after === "") ?? null,
    [figures],
  );
  const figuresAfter = useMemo(() => {
    const byBlock = new Map<string, ReadingFigure[]>();
    const known = new Set(source.blocks.map((b) => b.id));
    for (const f of figures ?? []) {
      if (f.after === "" || !known.has(f.after)) continue;
      const list = byBlock.get(f.after);
      if (list) list.push(f);
      else byBlock.set(f.after, [f]);
    }
    return byBlock;
  }, [figures, source.blocks]);

  return (
    // `mk-lite-room` is lite's override hook, and the ONLY way this fork is
    // allowed to restyle the room: the stylesheet above lives under
    // `apps/web/`, which lite may not touch (that isolation is the whole
    // reason the room forked). Every lite rule is therefore written as
    // `.mk-lite-room .mk-reading-room__x` in `src/index.css` — two classes
    // beat the shared file's one, so the override wins on specificity rather
    // than on which stylesheet the bundler happened to emit last.
    <div className="mk-reading-room mk-lite-room">
      <header className="mk-reading-room__topbar">
        <button type="button" className="mk-reading-room__back" onClick={onBack}>
          <BackIcon />
          {/* A lite student never sees a project workspace, so the label must
              not promise a place that isn't there. */}
          返回
        </button>
        <div className="mk-reading-room__brand">
          <span className="mk-reading-room__brand-name">思维印记 · 阅读工作台</span>
          <span className="mk-reading-room__brand-title">{source.title}</span>
        </div>
        {/* 完成这篇 在顶栏右上角。
            它原来和「文章 / 阅读成果」两个页签、一句提示、还有「透镜库 · 11」
            挤在正文上方那一条 52px 的横条里 —— 那条横条在 1360px 以下会折成
            两三行，把正文往下压掉近百像素。整条横条现在没有了：页签去了右栏，
            提示删掉了，透镜库藏了，只剩这一颗按钮，而顶栏本来就有空位。 */}
        <button
          type="button"
          className="mk-reading-room__finalize-btn"
          onClick={() => setConfirmFinish(true)}
        >
          完成这篇
        </button>
      </header>

      <main className="mk-reading-room__workspace" ref={workspaceRef} style={{ "--mk-room-left": `${leftPct}%` } as CSSProperties}>
        <section className="mk-reading-room__reading" aria-label="阅读材料区">
          <article className="mk-reading-room__article" ref={articleRef}>
              <div className="mk-reading-room__article-inner">
                <header className="mk-reading-room__article-header">
                  <div className="mk-reading-room__article-type">课堂阅读材料</div>
                  <h2>{source.title}</h2>
                  <div className="mk-reading-room__article-meta">
                    {source.origin && <span>来源 · {source.origin}</span>}
                    <span>{source.blocks.length} 段 · 课堂讨论材料</span>
                  </div>
                  {/* 打开原文: an honest external link to the source. The
                      readable extraction on the right IS "opening" the content;
                      this lets her open the live page in a new tab too. */}
                  {source.sourceUrl && (
                    <a
                      className="mk-reading-room__source-link"
                      href={source.sourceUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      打开原文 ↗
                    </a>
                  )}
                  {/* 🚨 导读在题图**之前**。它是她开读之前要看的那张地图，
                      而题图是一张 400px 高的照片 —— 摆在照片下面，导读就在
                      第一屏之外，走查的截图里正是这样（她得先往下滚才看得到
                      「这篇在问什么」，而那时候她已经开始读了）。 */}
                  {outline && (
                    <ReadingOutlineCard
                      outline={outline}
                      ordinalOf={ordinalOf}
                      onLocate={locateBlock}
                    />
                  )}
                  {leadFigure && <ArticleFigure figure={leadFigure} />}
                  {/* 查找与跳转。摆在题图之后、正文之前：它服务的是「读到一半
                      要回去找一个词」，不是开读前的那张地图。 */}
                  <ArticleFinder blocks={source.blocks} onJump={locateBlock} />
                  {loop.status === "idle" && <p className="student-selection-hint">划选文字可引用到对话；点击段落可查看该段的阅读工具</p>}
                </header>
                <Annotate
                  blocks={source.blocks}
                  headingBlockIds={headingBlockIds}
                  coreBlockIds={outline?.core}
                  state={{ material_id: source.id, spans }}
                  activeSpanId={activeSpanId}
                  onSelectSpan={setActiveSpanId}
                  lensMarkSpanId={loop.outcomes[0]?.id}
                  renderActiveCard={(span) => {
                    const o = outcomeBySpanId.get(span.id);
                    // A confirmed finding → the real 透镜卡 feedback recap.
                    if (o?.eval) return <ConfirmedFindingCard cardName={o.cardName} eval={o.eval} finding={o.finding} />;
                    // A plain 印记 flag (dimension + the question it hung on the
                    // sentence) → a lighter taro card in the same family.
                    return (
                      <div className="overflow-hidden rounded-mk-sm border border-mk-taro-bg bg-mk-surface shadow-mk-sm">
                        <div className="h-1 bg-mk-taro-fg" />
                        <div className="px-[15px] pb-[14px] pt-[12px]">
                          {span.tag && <div className="mb-2 font-sans text-[12px] font-bold text-mk-taro-fg">{span.tag}</div>}
                          <div className="font-sans text-[14px] leading-[1.6] text-mk-secondary">{span.note}</div>
                        </div>
                      </div>
                    );
                  }}
                  selectMode={loop.status === "active" ? { dimension: loop.cardName, onCancel: loop.repick } : null}
                  onCreateSpan={loop.pickSentence}
                  onReferenceBlock={loop.status === "idle" ? pickBlock : undefined}
                  onReferenceSelection={loop.status === "idle" ? addSelection : undefined}
                  renderAfterBlock={(blockId) => {
                    // Composed, not either/or: a paragraph can carry the
                    // hanging card AND the paragraph tools at the same time,
                    // and neither may hide the other.
                    const hanging =
                      card && cardBlockId === blockId ? (
                        <HangingCard
                          cardName={card.cardName}
                          status={card.status}
                          exampleWhy={card.exampleWhy}
                          eval={card.eval}
                          onStartPick={card.onStartPick}
                          onConfirm={card.onConfirm}
                          onRepick={card.onRepick}
                          onSkip={card.onSkip}
                          hasExample={card.hasExample}
                          pickHint={card.pickHint}
                        />
                      ) : null;
                    const aside =
                      blockTools.length > 0 && blockAnchor?.id === blockId ? (
                        <BlockToolsPanel
                          readingId={readingId}
                          blockId={blockId}
                          anchorEl={blockAnchor.el}
                          pointerX={blockAnchor.x}
                          tools={blockTools}
                          notes={blockNotes}
                          onNote={onBlockNote}
                          autoTool={autoTool}
                          onAutoToolConsumed={() => setAutoTool(null)}
                          onClose={() => {
                            setBlockAnchor(null);
                            setAutoTool(null);
                          }}
                        />
                      ) : null;
                    const pictures = (figuresAfter.get(blockId) ?? []).map((f) => (
                      <ArticleFigure key={f.url} figure={f} />
                    ));
                    if (!hanging && !aside && pictures.length === 0) return null;
                    return (
                      <>
                        {pictures}
                        {hanging}
                        {aside}
                      </>
                    );
                  }}
                />
              </div>
            </article>
        </section>

        {/* 分界，可以拖。role="separator" 是它真实的语义；aria-valuenow 报的是
            左栏占的百分比，和 CSS 变量是同一个数。 */}
        <div
          className="mk-reading-room__split"
          role="separator"
          aria-orientation="vertical"
          aria-label="调整文章和印记的宽度"
          aria-valuenow={Math.round(leftPct)}
          aria-valuemin={SPLIT_MIN}
          aria-valuemax={SPLIT_MAX}
          tabIndex={0}
          onPointerDown={onSplitPointerDown}
          onPointerMove={onSplitPointerMove}
          onPointerUp={onSplitPointerUp}
          onPointerCancel={onSplitPointerUp}
          onKeyDown={onSplitKeyDown}
          onDoubleClick={() => commitSplit(SPLIT_DEFAULT)}
        >
          <span aria-hidden="true" />
        </div>

        {/* 印记, on the RIGHT and wider than the article now — the 262px step
            rail that used to eat the left edge of this screen folded into
            `ReadingPlanDial` below, and this is what the width was freed for.
            The section is second in the DOM as well as second on screen, so
            reading order and tab order agree with the layout. */}
        <section className="mk-reading-room__coach" aria-label="AI 对话工作区">
          <StudentCoachHeading label="阅读与思考" />
          {/* 印记 | 阅读成果。左边那一栏永远是文章，所以这两页在这里并列：
              一页是她和印记正在说的话，一页是这一篇已经攒下的东西。 */}
          <div className="mk-lite-coachtabs" role="tablist" aria-label="右栏视图">
            <button
              type="button"
              role="tab"
              aria-selected={coachView === "chat"}
              className={coachView === "chat" ? "is-active" : ""}
              onClick={() => setCoachView("chat")}
            >
              印记
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={coachView === "outcomes"}
              className={coachView === "outcomes" ? "is-active" : ""}
              onClick={() => setCoachView("outcomes")}
            >
              阅读成果
              <span className="mk-lite-coachtabs__count">{loop.outcomes.length}</span>
            </button>
          </div>

          {/* 🚨 两页都挂着，用 hidden 藏一页，而不是二选一地渲染。
              ReadingCoachPanel 手里有她还没发出去的草稿、滚动位置、和一份
              乐观更新的消息列表 —— 卸载它等于她切一下页签就丢一段话。 */}
          <div className={coachView === "outcomes" ? "mk-lite-coachpane mk-lite-coachpane--scroll" : "mk-lite-coachpane mk-lite-coachpane--scroll is-hidden"}>
            <ReadingOutcomes outcomes={loop.outcomes} onLocate={locateBlock} />
          </div>

          <div className={coachView === "chat" ? "mk-lite-coachpane" : "mk-lite-coachpane is-hidden"}>
          {/* 我现在在第几步 — the present tense, always visible. The DIAL is the
              plan (every step, one hover away); this row is the one step she
              is on. Deliberately two surfaces, deliberately different jobs. */}
          <StepIndicator tasks={tasks} onLocate={locateBlock} />
          {/* ONE 印记. Same character, same `atom_message` table, one thread on
              screen instead of two. */}
          <ReadingCoachPanel
            readingId={readingId}
            tasks={tasks}
            initialMessages={coachMessages}
            slot={{
              locked: busyOrCarded,
              // 🚨 「锁住了」和「文章上真的有一副透镜」不是一回事：一次还在飞
              // 的请求也会锁住这一栏。面板那条「请到文章里选一句」只能挂在后者
              // 上 —— 挂错的话，她会点「带我过去」然后发现屏幕纹丝不动
              // （走查逐字报过这一条）。
              lensOpen: Boolean(cardBlockId) && loop.status !== "idle",
              quotes: quoted,
              removeQuote: removeQuoted,
              clearQuotes: () => setQuoted([]),
              // Narrow re-check, not a reload: nothing here unmounts the
              // room, so her transcript/draft/scroll position survive.
              onCardSummoned: () => void loop.refetchOpenCard(),
              // 透镜挂在哪一段，只有房间知道。面板锁住的时候那颗「带我过去」
              // 按钮就调这个。
              locateLens: () => {
                if (cardBlockId) locateBlock(cardBlockId);
              },
            }}
            onTasks={onTasks}
            onFocusBlock={focusBlock}
            // 她做完的透镜交给 印记：一轮真的回应 + 一次真的推进。见
            // `pendingLens` 上面的注释。
            lensDone={pendingLens}
            onLensDoneSent={() => setPendingLens(null)}
            // 读法走完之后，面板上出现「完成阅读，生成报告」。只有面板看得见
            // 「每一步都做完了」（它持有 tasks），而确认框和 finish 调用是房间的。
            // 见面板里那一段注释：在这之前，「全部完成」在屏幕上的全部体现是
            // 输入框换了句 placeholder，于是线上 23 篇阅读有 17 篇永远停在 active。
            onFinish={() => setConfirmFinish(true)}
          />
          </div>
        </section>
      </main>

      {/* 带读进度 · folded. Floats over the room's bottom-left corner at every
          width — unlike the rail it replaces, which hung in an `lg:`-gated
          aside and simply was not there on a phone. */}
      <ReadingPlanDial tasks={tasks} />

      {confirmFinish && (
        <div className="mk-finishask" role="dialog" aria-modal="true" aria-label="完成这篇">
          <div className="mk-finishask__card">
            <h2 className="text-mk-h3 text-mk-ink">完成这篇？</h2>
            <p className="mt-2 text-mk-body leading-relaxed text-mk-secondary">
              完成之后这篇就不能再改了——透镜、批注、对话都会停在这里。
              你走过的每一步会变成一份阅读报告。
            </p>
            {finishError && <p className="mt-3 text-mk-small text-mk-danger">{finishError}</p>}
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setConfirmFinish(false)} disabled={finishing}>
                再读一会儿
              </Button>
              <Button onClick={() => void finishReading()} loading={finishing}>
                完成，看报告
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * ArticleFigure —— 正文里的一张照片，连同图注与署名。
 *
 * 宽高写在标签上，所以浏览器在图到位之前就留好了位置，正文不会在图加载完的
 * 那一刻往下跳一截。链接是签过名的、有有效期的 —— 签不出来的那些服务端根本
 * 不会发过来，所以这里不必处理空 url。
 *
 * 它不是一段：这张图不带 `data-block-id`，选不中、也不会被工具卡挂上。理由见
 * apps/api/internal/library 的包注释。
 */
function ArticleFigure({ figure }: { figure: ReadingFigure }) {
  return (
    <figure className="mk-reading-figure">
      <img
        src={figure.url}
        alt={figure.caption}
        width={figure.width}
        height={figure.height}
        loading="lazy"
      />
      <figcaption>
        {figure.caption}
        {figure.credit && <span className="mk-reading-figure__credit">{figure.credit}</span>}
      </figcaption>
    </figure>
  );
}
