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
import {
  coachAnswerOf,
  coachCardOf,
  createReadingExcerpt,
  type LiteAnnotation,
  type LiteMessage,
  type ReadingBlockNote,
  type ReadingBlockTool,
  type ReadingLensDone,
  type ReadingTask,
} from "../api/readingRoom";
import type { ReadingFigure, ReadingOutline } from "../api/readings";
import { apiErrorText } from "../api/errorText";
import { ArticleFinder } from "./ArticleFinder";
import { BlockToolsPanel } from "./BlockToolsPanel";
import { ReadingCoachPanel } from "./ReadingCoachPanel";
import { PasteFullTextModal } from "./PasteFullTextModal";
import { SelectionTools } from "./SelectionTools";
import { ReadingHarvest, harvestBoards, harvestWritings, harvestWords } from "./ReadingHarvest";
import type { CoachCardAnswer, CoachCardSpec } from "./CoachCard";
import { ReadingPlanDial } from "./ReadingPlanDial";
import { AssignmentLine, useAssignmentForAtom } from "../inbox/AssignmentLine";

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
 *    slot: 「我现在在第几步」 is answered by `ReadingPlanDial`, the dial that
 *    sits on 印记's own column (hover = the steps, click = the full view).
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
  /**
   * 正文里放的只是这篇的摘要/导语。
   *
   * 从探索地图点开一条新闻、而服务端三条取正文的路都没走通时为真。正文下面
   * 因此多一条说明加一颗跳转按钮 —— 产品负责人 2026-09-16 的裁定，见
   * `explore/NewsSheet.tsx` 的文件头。**不说出来的话，两句话的摘要长得和一篇
   * 很短的文章一模一样，她会以为自己读完了。**
   */
  excerptOnly?: boolean;
  /**
   * 这份正文现在还能不能整份换掉。
   *
   * 🚨 2026-09-23 产品负责人第 3 条：「自己粘贴文本后，系统会自动分段，
   * 如果学生发现分段分错了，无法重新编辑，只能再开一个新的。」
   *
   * 服务端一直允许（只要还没有东西锚在正文上），拦住她的是这一侧：
   * 粘贴框原来只在 `excerptOnly` 的时候才挂上去，于是她粘完一次就回不去了。
   * 判据由服务端给，这一侧不自己猜。
   */
  sourceEditable?: boolean;
  /** 她把全文粘进来之后，让宿主重新加载这一间（正文、导读、清单都换了）。 */
  onSourceReplaced?: () => void;
  /**
   * 她摘抄过的那些句子（`atom_annotation` 的行）。
   *
   * 正文上那道下划线是从 `source.anchors` 画的，和这一份是同一批行 —— 这里
   * 单独再拿一次，是因为「阅读成果」那一页要按原样列出来（原句 + 段号 +
   * 一颗「和印记说」），而 anchor 是渲染用的形状，丢掉了创建时间。
   */
  excerpts?: LiteAnnotation[];
  /** 她刚摘抄了一句。房间发请求，宿主收着 —— 见宿主那边的注释。 */
  onExcerpt?: (a: LiteAnnotation) => void;
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
  /**
   * 这一栏此刻不能发东西。
   *
   * 🚨 2026-09-17 收窄了它的含义：它现在**只**表示「上一轮还在飞」。
   *
   * 在这之前，一副敞开的透镜也算锁住。产品负责人逐字报的后果：
   * 「找不到句子的时候，没法在聊天框打字求助 ai。」—— 透镜挑得不对（一篇讲读
   * 书态度的议论文被召了伦理学那副），她在文章里找不到任何一句符合的话，而屏幕
   * 上唯一还能动的东西是「跳过」。她没有办法说出「这篇里没有这种句子」，
   * 而她说的是对的（system prompt 里那条「她说文章里没有这种句子，先信她」
   * 从来没有机会被触发，因为她开不了口）。
   *
   * 透镜开着的时候，服务端那一轮会自己收着：不递新卡、不递第二副透镜、
   * 也不推进（见 reading_coach.go 的 anyOpen）。
   */
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
  excerptOnly = false,
  sourceEditable = false,
  onSourceReplaced,
  onBlockNote,
  excerpts = [],
  onExcerpt,
}: LiteReadingRoomProps) {
  // DEBT: `useReadingLoop` still carries pro's signature and wants a
  // projectId. It lives under apps/web, which lite may not touch, so the
  // reading id is passed in that position — the lite api object ignores it.
  const loop = useReadingLoop(readingId, source, api, undefined, initialOutcomes);
  const assignment = useAssignmentForAtom(readingId);

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
      setFinishError("完成阅读失败，请重试。");
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

  /**
   * 她在正文里划出的那几个字，以及工具条摆在哪儿。
   *
   * 🚨 2026-09-22：**划选本身不再做任何事。** 在这之前，划一句话有一个副作用
   * —— 那句话当场变成一条引用，摞在输入框上面。产品负责人逐字报的：
   *
   *   > 有时候划线是为了辅助阅读，但是一划线(select texts)句子就被收到右下角，
   *   > 还得一个个删除，可以划线后加一个「放入印记对话框」按键
   *
   * 她划一句只是为了读顺一点，那一句就不该跑到任何地方去。所以选区现在只做
   * 它本来该做的那一件事：标出「对哪几个字」，然后等她说要做什么。要做什么
   * 全在工具条上（摘抄 / 放入对话框 / 查词 / 语法），一件都不再自动发生。
   *
   * `start` / `end` 是这次划选的字偏移，摘抄要靠它在正文上画那道下划线。
   */
  const [selPick, setSelPick] = useState<
    { blockId: string; quote: string; start: number; end: number; x: number; y: number } | null
  >(null);

  function addSelection(
    blockId: string,
    quote: string,
    at?: { x: number; y: number },
    span?: { start: number; end: number },
  ) {
    if (!at || !span) {
      setSelPick(null);
      return;
    }
    setSelPick({ blockId, quote, start: span.start, end: span.end, x: at.x, y: at.y });
  }

  /** 把她划的那一句放进输入框上面的引用里。现在由工具条上那颗按钮调，
   *  不再是划选的副作用。 */
  function quoteSelection(blockId: string, quote: string) {
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

  /** 摘抄失败时那一句。失败要说出来 —— 她按了按钮、正文上什么都没变，
   *  不说的话她只会以为这颗按钮是坏的。 */
  const [excerptError, setExcerptError] = useState<string | null>(null);

  /** 这一句是不是已经在摘抄本里了。按**偏移**比，不按引文：同一句话在一段里
   *  出现两次的时候，引文分不出是哪一处。 */
  function alreadyExcerpted(blockId: string, start: number, end: number): boolean {
    return excerpts.some(
      (e) => e.blockId === blockId && e.span?.start === start && e.span?.end === end,
    );
  }

  async function excerptSelection(pick: { blockId: string; quote: string; start: number; end: number }) {
    setExcerptError(null);
    try {
      const saved = await createReadingExcerpt(readingId, pick);
      onExcerpt?.(saved);
    } catch (err) {
      setExcerptError(apiErrorText(err));
    }
  }

  const articleRef = useRef<HTMLDivElement | null>(null);

  /**
   * 荧光笔要标的词，`blockId → terms`。
   *
   * 她在哪一段开过「关键单词」，那一段的那几个词就一直标着 —— 关掉那张讲解卡
   * 也不消失。理由是她开这件工具的目的就是记住这几个词，而记住是靠**再读到
   * 它的时候认出来**，不是靠盯着一张卡片。
   *
   * term 是服务端拿回段落里逐字核对过的那一份写法，所以这里直接用，不猜。
   */
  const keywordTerms = useMemo(() => {
    const out: Record<string, string[]> = {};
    for (const note of blockNotes) {
      if (!note.words || note.words.length === 0) continue;
      const terms = note.words.map((w) => w.term).filter(Boolean);
      if (terms.length === 0) continue;
      out[note.blockId] = [...(out[note.blockId] ?? []), ...terms];
    }
    return out;
  }, [blockNotes]);

  /**
   * 她在哪几段上开过工具，各开了哪几件。
   *
   * 🚨 产品负责人 2026-09-17：「I want to record students' actions on these
   * bars. so once students clicked one thing, can we reveal that in the
   * paragraph? like tags after the texts? or something else? should not be
   * overlap with text, should not be too highlighted. just indicate that
   * students can view these things later.」
   *
   * 数据一直都在（`reading_block_note` 按 (blockId, tool) 存了一份重放），
   * 缺的是**她看得见**：读到第十段再想回头看第三段的翻译，屏幕上没有任何东西
   * 说那儿有一份。
   *
   * 「不压正文、不太抢眼」是他给的两条约束，所以这里只交出「哪一段、哪几件」，
   * 画成一行小字跟在那一段后面（`.mk-block-marks`）。
   */
  const blockMarks = useMemo(() => {
    const label = new Map(blockTools.map((t) => [t.id, t.label]));
    const out: Record<string, string[]> = {};
    for (const note of blockNotes) {
      // 工具目录里没有的那一件（比如 2026-09-17 并掉的「把握度」）：她确实开过，
      // 所以不能当它不存在；没有名字就不画 —— 一个没有名字的记号比没有记号更糟。
      const name = label.get(note.tool);
      if (!name) continue;
      const had = out[note.blockId] ?? [];
      if (!had.includes(name)) out[note.blockId] = [...had, name];
    }
    return out;
  }, [blockNotes, blockTools]);

  /**
   * 她**此刻正在读的那几段**，和摆在它们前面那一行小字。
   *
   * 🚨 产品负责人 2026-09-17：「the article, during 通读 stage, is becoming
   * several parts, with each parts a guidance, so students are able to read a
   * long article patiently… for each part, during that part, highlights those
   * paragraphs, with a small guidance above them.」
   *
   * 一篇十八段的文章摊在她面前，步骤名里写着「通读第1–4段」，而文章那一栏上
   * 一个记号都没有 —— 她得自己数到第四段，或者干脆一路读下去。
   *
   * 只在**通读**那几步上亮（clean：一个部分就是清单上的一步，
   * reading_plan.go 的 readingPartSteps），而且只亮当前那一步的那几段。
   * 别的步骤上一个记号都没有 —— 一直亮着的记号等于没有记号。
   */
  const activePart = useMemo(() => {
    const cur = tasks.find((t) => t.status === "pending");
    if (!cur || cur.kind !== "read" || !cur.blockId) return null;
    const part = (outline?.parts ?? []).find((p) => p.from === cur.blockId);
    if (!part) return null;
    const ids = source.blocks.map((b) => b.id);
    const from = ids.indexOf(part.from);
    const to = ids.indexOf(part.to);
    if (from < 0 || to < from) return null;
    const lead = part.does ? `现在读这一部分 · ${part.title} · ${part.does}` : `现在读这一部分 · ${part.title}`;
    return { blockIds: ids.slice(from, to + 1), lead: { [part.from]: lead } };
  }, [tasks, outline, source.blocks]);

  // 段落工具. The paragraph whose tool bar is open, together with what the bar
  // pins itself to: the paragraph element, and the x her pointer went down at.
  const [blockAnchor, setBlockAnchor] = useState<{ id: string; el: HTMLElement; x: number } | null>(null);
  // Set when the coach chose a tool for this turn; consumed once by the panel.
  const [autoTool, setAutoTool] = useState<string | null>(null);
  /** 带读那一栏手里那份转写。「阅读成果」那一页从它读她摆过的板。 */
  const [liveMessages, setLiveMessages] = useState<LiteMessage[]>(coachMessages);

  /**
   * 敞开的那张卡片上摆着的原话，`blockId → quotes`，在正文里标出来。
   *
   * 同事 2026-09-18：「希望句子在卡片里进行单独标注的时候，能够在文中也进行
   * highlight，方便让人看清其处于原文的哪些位置」。卡片上每一句只标了「第6段」，
   * 她要把它放回那一段里才判断得了它在论证里干什么。
   *
   * 「敞开」= 转写里最后一张卡，而且它后面还没有她的作答。她一交，标记就收掉 ——
   * 一直亮着的记号等于没有记号。
   */
  const cardQuotes = useMemo(() => {
    let open: CoachCardSpec | null = null;
    for (const m of liveMessages) {
      const card = coachCardOf(m);
      if (card) open = card;
      else if (coachAnswerOf(m)) open = null;
    }
    const out: Record<string, string[]> = {};
    for (const o of open?.options ?? []) {
      if (!o.blockId || !o.quote.trim()) continue;
      out[o.blockId] = [...(out[o.blockId] ?? []), o.quote.trim()];
    }
    return out;
  }, [liveMessages]);

  /** 「阅读成果」页签上那个数：她真的产出了几样东西。 */
  const harvestCount =
    excerpts.length +
    harvestWords(blockNotes).length +
    blockNotes.filter((n) => n.grammar && n.subject).length +
    harvestBoards(liveMessages).length +
    harvestWritings(liveMessages).length +
    loop.outcomes.length;

  /** autoTool 要讲的那几个字（她划出来的）。空 = 照旧请她在段落里点一次。 */
  const [autoSubject, setAutoSubject] = useState<string | null>(null);

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

  // 选区没了，工具条就跟着走。判据是**选区本身**（她点了别处、按了 Esc、
  // 又划了一次），不是某一次点击 —— 工具条说的是「对这几个字」，那几个字
  // 不再高亮着，它就没有了指称对象。
  useEffect(() => {
    const onSel = () => {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed || !sel.toString().trim()) setSelPick(null);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setSelPick(null);
    };
    document.addEventListener("selectionchange", onSel);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("selectionchange", onSel);
      window.removeEventListener("keydown", onKey);
    };
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
  function focusBlock(blockId: string, tool?: string, subject?: string) {
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
    setAutoSubject(subject ?? null);
  }

  /**
   * Her own click on a paragraph. In pro this gesture quotes the paragraph
   * into the composer; in lite the composer belongs to 带读, and this is the
   * better thing to spend the click on. Toggles, so a second click on the
   * paragraph she is already looking at puts the bar away.
   */
  function pickBlock(blockId: string) {
    setAutoTool(null);
    setSelPick(null);
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
  // 只有摘要的那一篇：一进来就请她把全文粘进来（产品负责人 2026-09-17）。
  // 她选「先读摘要」关掉之后，正文下面那一条里还有一颗按钮能再打开。
  const [pasteOpen, setPasteOpen] = useState(Boolean(excerptOnly));
  /**
   * 她在段落工具（想一想 / 仿写）底下写好、要交给 印记 的那一段，
   * 以及这一轮落地之后要告诉那个框的那一声（送到了 / 没送到）。
   *
   * 形状照 `pendingLens`：段落工具在正文那一栏，对话在另一栏，房间把前者交给
   * 后者，后者把它变成一轮真的对话。
   */
  const [pendingToolAnswer, setPendingToolAnswer] = useState<{
    answer: CoachCardAnswer;
    done: (ok: boolean) => void;
  } | null>(null);
  function sendToolAnswer(answer: CoachCardAnswer): Promise<boolean> {
    return new Promise((resolve) => setPendingToolAnswer({ answer, done: resolve }));
  }
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
    // 🚨 复核的**结论**也要带过去，不只是那句 finding。
    //
    // 产品负责人 2026-09-17：「句子匹配不通过，但是点击记录发现后，主 ai 又给出
    // 了不一样的回答。」少了这两样，这一轮就是两个模型各说各的：复核当着她的面
    // 判了「这一句撑不住」，印记 只拿到句子和 finding，于是照着「她交作业了，
    // 先说她哪里选得准」那一节夸了一句。她刚读完前一句，紧接着读到后一句。
    setPendingLens({
      cardName: o.cardName,
      quote: o.quote,
      finding: o.finding,
      verdict: o.eval?.verdict,
      verdictReason: o.eval?.verdictReason,
    });
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
                    {/* Inherits the meta row's size and colour; renders
                        nothing unless this reading came from an assignment. */}
                    <AssignmentLine atomId={readingId} className="" />
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
                  {/* 🚨 导读**不在这里**了。2026-09-17 产品负责人：
                      「the reading guide. I think it can be shown in the ai box
                      instead of paper top.」
                      它是 印记 排出来的东西，也是 印记 接下来每一步的依据 ——
                      摆在正文顶上，它读起来像文章自带的一段前言；摆在 印记
                      那一栏的开头，它是「它要带你怎么读」。
                      渲染在 ReadingCoachPanel 里，见那边的 outline 那一段。 */}
                  {leadFigure && <ArticleFigure figure={leadFigure} />}
                  {/* 查找与跳转。摆在题图之后、正文之前：它服务的是「读到一半
                      要回去找一个词」，不是开读前的那张地图。 */}
                  <ArticleFinder blocks={source.blocks} onJump={locateBlock} />
                  {loop.status === "idle" && <p className="student-selection-hint">{excerptOnly ? "划选文字后可放入对话框；导入全文后可使用摘抄功能" : "划选文字后可摘抄或放入对话框；点击段落可查看该段的阅读工具"}</p>}
                  {excerptError && (
                    <p className="student-selection-hint" role="alert" style={{ color: "var(--mk-danger)" }}>
                      摘抄失败：{excerptError}
                    </p>
                  )}
                </header>
                {selPick && (
                  <SelectionTools
                    quote={selPick.quote}
                    at={{ x: selPick.x, y: selPick.y }}
                    tools={blockTools}
                    excerpted={alreadyExcerpted(selPick.blockId, selPick.start, selPick.end)}
                    // 摘抄是正文定下来之后的事 —— 只有摘要的那一篇不摆这颗按钮。
                    excerptable={!excerptOnly}
                    onPick={(toolId) => {
                      const { blockId, quote } = selPick;
                      setSelPick(null);
                      window.getSelection()?.removeAllRanges();
                      focusBlock(blockId, toolId, quote);
                    }}
                    onExcerpt={() => {
                      const pick = selPick;
                      setSelPick(null);
                      window.getSelection()?.removeAllRanges();
                      void excerptSelection(pick);
                    }}
                    onSendToCoach={() => {
                      const { blockId, quote } = selPick;
                      setSelPick(null);
                      window.getSelection()?.removeAllRanges();
                      quoteSelection(blockId, quote);
                      // 引用摞在印记那一栏的输入框上面，而她可能正停在
                      // 「阅读成果」那一页 —— 不切过去，她按了按钮、屏幕上
                      // 什么都没有。
                      setCoachView("chat");
                    }}
                    onDismiss={() => {
                      setSelPick(null);
                      window.getSelection()?.removeAllRanges();
                    }}
                  />
                )}
                <Annotate
                  blocks={source.blocks}
                  headingBlockIds={headingBlockIds}
                  coreBlockIds={outline?.core}
                  activeBlockIds={activePart?.blockIds}
                  blockLead={activePart?.lead}
                  blockMarks={blockMarks}
                  keywordTerms={keywordTerms}
                  cardQuotes={cardQuotes}
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
                          blockText={
                            source.blocks.find((b) => b.id === blockId)?.text ?? ""
                          }
                          anchorEl={blockAnchor.el}
                          pointerX={blockAnchor.x}
                          tools={blockTools}
                          notes={blockNotes}
                          onNote={onBlockNote}
                          ordinal={ordinalOf(blockId)}
                          onToolAnswer={sendToolAnswer}
                          autoTool={autoTool}
                          autoSubject={autoSubject}
                          onAutoToolConsumed={() => {
                            setAutoTool(null);
                            setAutoSubject(null);
                          }}
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
                {excerptOnly && (
                  <ExcerptOnlyNotice url={source.sourceUrl} onPaste={() => setPasteOpen(true)} />
                )}
                {/*
                  🚨 分段分错了，她要回得去。产品负责人 2026-09-23 第 3 条：
                  「自己粘贴文本后，系统会自动分段，如果学生发现分段分错了，
                    无法重新编辑，只能再开一个新的。」

                  只在**还没有东西锚在正文上**的时候摆（服务端给的 editable）：
                  锚定之后换正文会把每一张卡、每一条批注悄悄重新指到别的句子上，
                  那比不给她改糟得多。摘要那一篇上面已经有自己的入口，不重复摆。
                */}
                {!excerptOnly && sourceEditable && (
                  <p className="mk-reading-room__resplit text-mk-small text-mk-muted">
                    分段不对？
                    <button
                      type="button"
                      onClick={() => setPasteOpen(true)}
                      className="ml-1 text-mk-accent-700 underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                    >
                      重新编辑原文
                    </button>
                  </p>
                )}
                {(excerptOnly || sourceEditable) && (
                  <PasteFullTextModal
                    open={pasteOpen}
                    onClose={() => setPasteOpen(false)}
                    readingId={readingId}
                    title={source.title ?? ""}
                    sourceUrl={source.sourceUrl}
                    abstract={source.blocks.map((b) => b.text)}
                    mode={excerptOnly ? "abstract" : "resplit"}
                    onReplaced={() => {
                      setPasteOpen(false);
                      onSourceReplaced?.();
                    }}
                  />
                )}
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
            `ReadingPlanDial` (now docked on this column's tab row), and this
            is what the width was freed for.
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
              <span className="mk-lite-coachtabs__count">{harvestCount}</span>
            </button>
            {/* 带读进度。2026-09-22 从房间左下角的浮动位置搬到这里 ——
                产品负责人：「we have a round button showing all steps. can we
                move it to the right ai side?」。悬停给步骤清单，点击就地摊开
                展开视图（原来那块常驻的 StepIndicator 整个删掉了）。 */}
            <div className="mk-lite-coachtabs__spacer" />
            <ReadingPlanDial tasks={tasks} onLocate={locateBlock} />
          </div>

          {/* 🚨 两页都挂着，用 hidden 藏一页，而不是二选一地渲染。
              ReadingCoachPanel 手里有她还没发出去的草稿、滚动位置、和一份
              乐观更新的消息列表 —— 卸载它等于她切一下页签就丢一段话。 */}
          <div className={coachView === "outcomes" ? "mk-lite-coachpane mk-lite-coachpane--scroll" : "mk-lite-coachpane mk-lite-coachpane--scroll is-hidden"}>
            <ReadingHarvest
              notes={blockNotes}
              messages={liveMessages}
              outcomes={loop.outcomes}
              excerpts={excerpts}
              ordinalOf={ordinalOf}
              onLocate={locateBlock}
              onDiscuss={(blockId, quote) => {
                quoteSelection(blockId, quote);
                setCoachView("chat");
              }}
            />
          </div>

          <div className={coachView === "chat" ? "mk-lite-coachpane" : "mk-lite-coachpane is-hidden"}>
          {/* 🚨 这里原来钉着一整块 `StepIndicator`（编号徽章 + 第几步 + 标题 +
              定位原文 + 十五个圆点）。它和页签上那个盘是两块同一件事的面，而它
              占掉的正是 印记 说话的地方 —— 产品负责人 2026-09-22 附截图红框圈
              的就是它：「导致印记的提示句被压缩在下面很小的地方，需要不停上下
              翻动」。整块删掉；它的内容搬进盘的展开视图，点一下才出来。 */}
          {/* ONE 印记. Same character, same `atom_message` table, one thread on
              screen instead of two. */}
          <ReadingCoachPanel
            readingId={readingId}
            tasks={tasks}
            initialMessages={coachMessages}
            // 导读 2026-09-17 从正文顶上搬到了这一栏的开头。
            outline={outline}
            ordinalOf={ordinalOf}
            onLocateBlock={locateBlock}
            onMessagesChange={setLiveMessages}
            slot={{
              // 🚨 `loop.busy`，不是 `busyOrCarded` —— 一副敞开的透镜不再锁住
              // 这一栏。她在文章里找不到句子的时候得能开口求助。见 `locked`。
              locked: loop.busy,
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
            toolAnswer={pendingToolAnswer?.answer ?? null}
            onToolAnswerSent={(ok) => {
              pendingToolAnswer?.done(ok);
              setPendingToolAnswer(null);
            }}
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

      {confirmFinish && (
        <div className="mk-finishask" role="dialog" aria-modal="true" aria-label="完成这篇">
          <div className="mk-finishask__card">
            <h2 className="text-mk-h3 text-mk-ink">完成这篇？</h2>
            <p className="mt-2 text-mk-body leading-relaxed text-mk-secondary">
              你走过的每一步会汇总成一份阅读报告。之后可在报告页的「查看阅读记录」里继续阅读，再次完成时报告会重新生成。
            </p>
            {/* She is handing in homework; say so. */}
            {assignment && (
              <p className="mt-2 text-mk-body leading-relaxed text-mk-secondary">
                这是老师布置的作业，完成后老师会看到你已完成。
              </p>
            )}
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

/**
 * ExcerptOnlyNotice —— 「上面那段只是摘要」。
 *
 * # 它为什么必须存在
 *
 * 从探索地图点开一条新闻，服务端会按三条路取正文：feed 自带的正文 → 现抓一次
 * 原页面 → feed 的导语。落到第三条时，正文区里放的是两三句话的摘要。
 *
 * **一段摘要和一篇很短的报道在屏幕上长得一模一样。** 不说出来，她读完两句话
 * 就以为读完了这一篇 —— 这是「看不见就是没有」那条教训的另一种形态
 * （memory: observation-tool-is-the-bug-2026-09-12）。
 *
 * 文案是产品负责人 2026-09-16 逐字给的：
 *
 *   > 「我们无法直接获取正文，如果想要阅读全文，请跳转原网站」，按钮「点击跳转」
 *
 * # 跳转是她按的，不是我们替她按的
 *
 * 上一版在「现在读」的时候自动 `window.open` 一次原文，这个组件取代的就是那件
 * 事。链接照旧带 `rel="noopener noreferrer"`。
 */
function ExcerptOnlyNotice({ url, onPaste }: { url: string; onPaste: () => void }) {
  return (
    <aside className="mk-reading-excerpt-note">
      <p className="mk-reading-excerpt-note__text">
        我们无法直接获取正文，如果想要阅读全文，请跳转原网站
      </p>
      {/* 2026-09-17：跳过去看完还得回来 —— 把全文粘进来，后面的读法才按全文排。 */}
      <button type="button" className="mk-reading-excerpt-note__cta" onClick={onPaste}>
        粘贴全文
      </button>
      {url ? (
        <a
          className="mk-reading-excerpt-note__cta"
          href={url}
          target="_blank"
          rel="noopener noreferrer"
        >
          点击跳转
        </a>
      ) : (
        // 没有链接可跳的时候不摆一颗点不动的按钮：说清楚这一篇没有留下原网址，
        // 比一颗假按钮诚实。从地图进来的那些不会走到这里（星球一定有 url），
        // 但这个组件不该假设自己只被那一条路用到。
        <p className="mk-reading-excerpt-note__text">这一篇没有留下原网址。</p>
      )}
    </aside>
  );
}
