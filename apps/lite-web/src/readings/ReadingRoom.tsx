import { useEffect, useMemo, useRef, useState } from "react";
import type { Anchor, AnnotateState, MaterialSource, SelectionEval, TakeawayDraft } from "@mind-imprint/contracts";
import { Annotate } from "@/primitives/annotate";
import { anchorToSpan } from "@/studio/material/SourceDossier";
import { HangingCard, type HangingCardStatus, anchorBlockId } from "@/studio/reading/HangingCard";
import { ConfirmedFindingCard } from "@/studio/reading/ConfirmedFindingCard";
import { READING_DECK_IDS } from "@/studio/reading/readingDeck";
import { LensLibrary } from "@/studio/reading/LensLibrary";
import { ReadingOutcomes } from "@/studio/reading/ReadingOutcomes";
import { FinalizeReadingPanel } from "@/studio/reading/FinalizeReadingPanel";
import { useReadingLoop, type ReadingLoopApi, type ReadingOutcome } from "@/studio/reading/readingLoop";
import "@/studio/reading/ReadingRoom.css";
import type { LiteMessage, ReadingBlockNote, ReadingBlockTool, ReadingTask } from "../api/readingRoom";
import { BlockToolsPanel } from "./BlockToolsPanel";
import { ReadingCoachPanel } from "./ReadingCoachPanel";
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
 *    and the 阶段 dropdown beside it was already lite-hidden. The slot it left
 *    left is now held by `StepIndicator`, which answers 「我现在在第几步」.
 *
 * The leaf components (Annotate, HangingCard, LensLibrary, ReadingOutcomes,
 * FinalizeReadingPanel, useReadingLoop, the stylesheet) are still IMPORTED
 * from `apps/web` — the fork copied the composition, not the parts.
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

/**
 * The loop's own slice plus the two takeaway calls the room drives directly.
 *
 * The pro parameter positions (`projectId`, `rid`) are kept because
 * `useReadingLoop` still has pro's signature — see the `readingId` note at the
 * call site below. `createReadingRoomApi` closes over the reading id and
 * ignores both.
 */
export type LiteReadingRoomApi = ReadingLoopApi & {
  getTakeawayDraft(projectId: string, rid: string): Promise<TakeawayDraft>;
  postFinalizeReading(
    projectId: string,
    rid: string,
    body: { newLeads: string[]; proposalImpact: string },
  ): Promise<unknown>;
};

export type LiteReadingRoomProps = {
  /** The atom id. A lite reading has no project and no reference row — the
   *  atom IS the addressing unit. */
  readingId: string;
  source: MaterialSource;
  api: LiteReadingRoomApi;
  onBack: () => void;
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

// The focused reading surface: 带读 on the left, the article on the right with
// 文章 | 阅读成果 view-tabs. The article enters select-mode once a card is
// `active`; the hanging card renders from the loop's live status/eval; every
// confirmed finding accumulates in 阅读成果.
export function ReadingRoom({
  readingId,
  source,
  api,
  onBack,
  tasks,
  onTasks,
  coachMessages,
  initialOutcomes,
  blockTools,
  blockNotes,
  onBlockNote,
}: LiteReadingRoomProps) {
  // DEBT (next task): `useReadingLoop` still carries pro's signature and wants
  // a projectId; `getTakeawayDraft`/`postFinalizeReading` still want a
  // (projectId, rid) pair. It lives under apps/web, which this task may not
  // touch, so the reading id is passed into both positions — the lite api
  // object ignores them anyway.
  const loop = useReadingLoop(readingId, source, api, undefined, initialOutcomes);

  const [rightView, setRightView] = useState<"article" | "trace">("article");

  // 完成这篇: the finalize panel. draft holds the assembled record (read-only)
  // + seeded synthesis suggestions; impact is her own 我的收获.
  const [finalizeOpen, setFinalizeOpen] = useState(false);
  const [finalizeLoading, setFinalizeLoading] = useState(false);
  const [finalizeDraft, setFinalizeDraft] = useState<TakeawayDraft | null>(null);
  const [finalizeLeads, setFinalizeLeads] = useState("");
  const [finalizeImpact, setFinalizeImpact] = useState("");
  const [finalizeSaving, setFinalizeSaving] = useState(false);
  const [finalizeDone, setFinalizeDone] = useState(false);

  async function openFinalize() {
    setFinalizeOpen(true);
    setFinalizeDone(false);
    setFinalizeLoading(true);
    try {
      const draftResult = await api.getTakeawayDraft(readingId, readingId);
      setFinalizeDraft(draftResult);
      setFinalizeLeads(draftResult.suggestedNewLeads.join("\n"));
      setFinalizeImpact(draftResult.suggestedProposalImpact);
    } catch {
      setFinalizeDraft(null);
    } finally {
      setFinalizeLoading(false);
    }
  }

  async function confirmFinalize() {
    if (finalizeSaving) return;
    setFinalizeSaving(true);
    try {
      await api.postFinalizeReading(readingId, readingId, {
        newLeads: finalizeLeads
          .split("\n")
          .map((s) => s.trim())
          .filter(Boolean),
        proposalImpact: finalizeImpact.trim(),
      });
      setFinalizeDone(true);
      // No `onFinalized` callback: the fork briefly grew one, and nothing ever
      // consumed it — pro's room has no such prop either. A finished reading
      // is re-read from the server the next time `/readings/:id` opens, which
      // is where the terminal 已完成 surface lives.
      // Briefly show the ✓, then close the modal so she lands back on the
      // reading conversation instead of having to hunt for a 关闭 button.
      window.setTimeout(() => setFinalizeOpen(false), 900);
    } catch {
      // keep the panel open so she can retry — never silently discard her edits
    } finally {
      setFinalizeSaving(false);
    }
  }

  // Click-to-reveal: clicking a highlighted span opens its note panel
  // (dimension + answer/question) inline, right under the sentence. Only
  // takes effect outside select-mode — Annotate routes mark clicks through
  // pickSentence instead while selectMode is set, so this never fights
  // evidence-picking.
  const [activeSpanId, setActiveSpanId] = useState<string | null>(null);

  // 透镜库 (LensLibrary) — the student browses the reading deck and summons
  // a CHOSEN card onto the article herself, rather than only ever waiting
  // for the AI to propose one.
  const [libraryOpen, setLibraryOpen] = useState(false);

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

  // Keep the article view forward whenever a card is proposed, so the newly
  // drawn example is visible.
  useEffect(() => {
    if (loop.status !== "idle") setRightView("article");
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

  function locateBlock(blockId: string) {
    setRightView("article");
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

  // The article's own spans = the source's persisted anchors, the confirmed
  // outcomes' spans (so findings stay highlighted after the card retires —
  // the process tree "grows"), plus (once the loop has them) the AI's live
  // example anchor and the student's own live pick.
  const spans = useMemo(() => {
    const extra: Anchor[] = [];
    for (const o of loop.outcomes) {
      extra.push({
        id: o.id, material_id: source.id, block_id: o.blockId, start: o.start, end: o.end,
        quote: o.quote, dimension: o.cardId, author: "student", question: "", answer: o.finding,
      });
    }
    if (loop.exampleAnchor) extra.push(loop.exampleAnchor);
    if (loop.studentAnchor) extra.push(loop.studentAnchor);
    return [...source.anchors, ...extra].map(anchorToSpan).filter((s): s is AnnotateSpan => s !== null);
  }, [source.anchors, source.id, loop.outcomes, loop.exampleAnchor, loop.studentAnchor]);

  // Confirmed findings, keyed by span id — clicking a finding's highlight shows
  // its full 透镜卡 recap (verdict + checks) rather than a bare note.
  const outcomeBySpanId = useMemo(() => new Map(loop.outcomes.map((o) => [o.id, o])), [loop.outcomes]);

  return (
    <div className="mk-reading-room">
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
      </header>

      <main className="mk-reading-room__workspace">
        <section className="mk-reading-room__coach" aria-label="AI 对话工作区">
          {/* The slot pro fills with 「你读这篇是为了」. Here it answers 「我
              现在在第几步」 instead — and because ReadingPlanRail hangs in an
              `lg:`-gated aside, on a narrow screen this is the ONLY place she
              can see that. */}
          <StepIndicator tasks={tasks} />
          {/* ONE 印记. Same character, same `atom_message` table, one thread on
              screen instead of two. */}
          <ReadingCoachPanel
            readingId={readingId}
            tasks={tasks}
            initialMessages={coachMessages}
            slot={{
              locked: busyOrCarded,
              quotes: quoted,
              removeQuote: removeQuoted,
              clearQuotes: () => setQuoted([]),
              // Narrow re-check, not a reload: nothing here unmounts the
              // room, so her transcript/draft/scroll position survive.
              onCardSummoned: () => void loop.refetchOpenCard(),
            }}
            onTasks={onTasks}
            onFocusBlock={focusBlock}
          />
        </section>

        <section className="mk-reading-room__reading" aria-label="阅读材料区">
          <div className="mk-reading-room__toolbar">
            <div className="mk-reading-room__view-tabs" role="tablist" aria-label="右侧视图">
              <button
                type="button"
                role="tab"
                aria-selected={rightView === "article"}
                className={rightView === "article" ? "is-active" : ""}
                onClick={() => setRightView("article")}
              >
                文章
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={rightView === "trace"}
                className={rightView === "trace" ? "is-active" : ""}
                onClick={() => setRightView("trace")}
              >
                阅读成果 <span className="mk-reading-room__count">{loop.outcomes.length}</span>
              </button>
            </div>
            <span className="mk-reading-room__hint">
              <i />
              {loop.status === "active"
                ? "点击 1 句话作答"
                : loop.status === "proposed"
                  ? "先看示范，再开始选句"
                  : quoted.length > 0
                    ? `已引用 ${quoted.length} 处 · 可在下方逐条取消`
                    : "点一段，看这一段能怎么拆开"}
            </span>
            {/* 透镜库 sits in the toolbar rather than a starter row: it acts on
                the article, which is what this toolbar is for, and
                「换一个透镜再看」 is a real step in every 带读 routine. */}
            <button
              type="button"
              className="mk-reading-room__trace-btn"
              onClick={() => setLibraryOpen(true)}
              disabled={busyOrCarded}
              title="换一副透镜，把这篇再看一遍"
            >
              透镜库 · {READING_DECK_IDS.length}
            </button>
            <button
              type="button"
              className="mk-reading-room__finalize-btn"
              onClick={() => void openFinalize()}
            >
              完成这篇
            </button>
          </div>

          {rightView === "article" ? (
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
                </header>
                <Annotate
                  blocks={source.blocks}
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
                    if (!hanging && !aside) return null;
                    return (
                      <>
                        {hanging}
                        {aside}
                      </>
                    );
                  }}
                />
              </div>
            </article>
          ) : (
            <div className="mk-reading-room__article">
              <ReadingOutcomes outcomes={loop.outcomes} onLocate={locateBlock} />
            </div>
          )}
        </section>
      </main>

      {libraryOpen && (
        <LensLibrary
          onPick={(id) => {
            setLibraryOpen(false);
            void loop.summonCard(id);
          }}
          onClose={() => setLibraryOpen(false)}
        />
      )}

      {finalizeOpen && (
        <FinalizeReadingPanel
          loading={finalizeLoading}
          draft={finalizeDraft}
          leadsText={finalizeLeads}
          onLeadsChange={setFinalizeLeads}
          impactText={finalizeImpact}
          onImpactChange={setFinalizeImpact}
          saving={finalizeSaving}
          done={finalizeDone}
          onConfirm={() => void confirmFinalize()}
          onClose={() => setFinalizeOpen(false)}
          // Both were `caps.proposalImpact` / `caps.credibility`, both false in
          // lite: there is no 立题 for 新的线索 to feed, and no CRAAP-style
          // producer behind a 可信度 verdict.
          proposalImpact={false}
          credibility={false}
        />
      )}
    </div>
  );
}
