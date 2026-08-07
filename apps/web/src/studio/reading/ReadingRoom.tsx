import { useEffect, useMemo, useRef, useState } from "react";
import type { Anchor, AnnotateState, MaterialSource, ReadingBrief, Reference, SelectionEval, TakeawayDraft } from "@mind-imprint/contracts";
import { PhaseTag } from "@mind-imprint/contracts";
import { Annotate } from "../../primitives/annotate";
import { anchorToSpan } from "../material/SourceDossier";
import { HangingCard, type HangingCardStatus, anchorBlockId } from "./HangingCard";
import { READING_DECK_IDS } from "./readingDeck";
import { LensLibrary } from "./LensLibrary";
import { ReadingOutcomes } from "./ReadingOutcomes";
import { FinalizeReadingPanel } from "./FinalizeReadingPanel";
import { useReadingLoop, type ReadingLoopApi } from "./readingLoop";
import "./ReadingRoom.css";

type AnnotateSpan = AnnotateState["spans"][number];

// Built internally (Task 10) from the live `useReadingLoop` state — no
// longer an external prop; `exampleBlockId`/`studentBlockId` feed
// `anchorBlockId` (the signature anchor-move) to decide which paragraph the
// card hangs under.
export type ReadingRoomCard = {
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
};

// ReadingRoomApi — the loop's own slice (readTurn/activateProjectCard/…) plus
// the three S2 brief/takeaway calls (Task 9), which the room drives directly
// rather than through `useReadingLoop` — they aren't part of the client-side
// card state machine, just one-off reads/writes keyed by the reference id.
export type ReadingRoomApi = ReadingLoopApi & {
  putReadingBrief(projectId: string, rid: string, brief: ReadingBrief): Promise<void>;
  getTakeawayDraft(projectId: string, rid: string): Promise<TakeawayDraft>;
  postFinalizeReading(
    projectId: string,
    rid: string,
    body: { newLeads: string[]; proposalImpact: string },
  ): Promise<Reference>;
};

export type ReadingRoomProps = {
  projectId: string;
  // The reference row this material was opened from — the brief/takeaway
  // endpoints (Task 9) are keyed by reference id (rid), NOT the material id
  // (source.id below is the material). Always the reference the student
  // opened via 进入阅读室/开始共读 in the Library.
  referenceId: string;
  source: MaterialSource;
  // A deterministic, no-model-call first-draft "why read this" — Go's
  // suggestReadingReason, merged onto enter-reading's response. Used as the
  // reason seed ONLY when the reference has no persisted readingReason yet
  // (a source opened for the first time); empty when entered via the
  // paste-body fallback (no proposal-based suggestion computed there) or
  // when the caller omits it.
  suggestedReason?: string;
  // The reference's PERSISTED brief (Task 9 fix) — phaseTag/readingReason/
  // readingFocus now surface on the Reference DTO (toReferenceDTO), so a
  // reopened source seeds the banner from her TRUE last-saved values instead
  // of re-deriving a stale default. This matters because putReadingBrief is a
  // full-replace PUT: every save resends all 3 fields, so seeding any one of
  // them from the wrong source (e.g. always "" or always the template) would
  // silently overwrite whichever field the student didn't touch this time.
  phaseTag?: PhaseTag | null;
  readingReason?: string | null;
  readingFocus?: string | null;
  // #8: the student's own freeform note on this source (我的笔记). Seeded from
  // the reference DTO; edits persist via onSaveNote (patchReference upstream).
  readingNote?: string | null;
  // #4: the reference's persisted bibliographic metadata (recovered from a DOI
  // via Crossref). The abstract is CONTEXT shown collapsibly in the header, not
  // the article body; author/year/journal fill a metadata line; url backs the
  // 打开原文 external link. All optional — a source with no DOI shows none of it.
  bib?: {
    title?: string;
    author?: string;
    year?: string;
    journal?: string;
    abstract?: string;
    url?: string;
  } | null;
  onSaveNote?: (note: string) => Promise<void>;
  // finalized tells the workspace whether the student 归纳'd this source before
  // leaving, so it can show a carry-forward acknowledgment (EA).
  onBack: (finalized: boolean) => void;
  // ReadingRoom owns the loop (`useReadingLoop`) internally, plus drives the
  // S2 brief/takeaway calls directly off the same api slice.
  api: ReadingRoomApi;
  // Reinstates the reading-time logging that used to fire from
  // SourceDossier's open/close lifecycle (Task 8 binding) — StudioContainer
  // passes its existing `onOpenLogged` callback through here. Optional so a
  // bare render (e.g. a component test with no logging concern) still works.
  onOpenLogged?: (materialId: string, timeSpentS: number) => void;
};

// Starter prompts adapted to OUR reading deck (source-checking + deep
// reading) — the demo's `.starter-row`, one tap fills + sends.
const STARTERS = ["这条来源可信吗？", "帮我看看这段的论证", "这句是事实还是观点？"];

// #7: starters that should deterministically SUMMON a card (via the reliable
// lens-library path) instead of a 克制 coach turn that rarely proposes one.
// "这条来源可信吗？" → the CRAAP source-check card. The other two stay
// conversational (they depend on a sentence the student hasn't picked yet).
const STARTER_SUMMON: Record<string, string> = { "这条来源可信吗？": "craap" };

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// The focused reading surface (spec: "read together"), rebuilt to the
// reference demo's shape: a warm coach column (heading → dialogue log →
// composer pinned at the bottom → starter row) on the left, and a reading
// pane on the right with 文章 | 阅读成果 view-tabs. The article enters
// select-mode once a card is `active`; the hanging card renders from the
// loop's live status/eval; every confirmed finding accumulates in 阅读成果.
export function ReadingRoom({
  projectId,
  referenceId,
  source,
  suggestedReason,
  phaseTag,
  readingReason,
  readingFocus,
  readingNote,
  bib,
  onSaveNote,
  onBack,
  api,
  onOpenLogged,
}: ReadingRoomProps) {
  const loop = useReadingLoop(projectId, source, api);
  const [draft, setDraft] = useState("");
  const [rightView, setRightView] = useState<"article" | "trace">("article");

  // Brief-in (S2, Task 9; data-loss fix): why-read-THIS-source + which
  // argument phase it's for. Seeded from the PERSISTED brief when one
  // exists (readingReason/phaseTag/readingFocus, plumbed from the
  // reference row) — suggestedReason is only the fallback for a source with
  // no saved reason yet. Every save resends all three fields —
  // putReadingBrief is a full-replace endpoint — so seeding from the true
  // saved values (not a stale template / always-blank) is what keeps an
  // edit to ONE field from wiping the other on save.
  const [briefReason, setBriefReason] = useState(
    readingReason && readingReason.trim() ? readingReason : suggestedReason ?? "",
  );
  const [briefEditingReason, setBriefEditingReason] = useState(false);
  const [briefFocus] = useState(readingFocus ?? "");
  const [briefPhase, setBriefPhase] = useState<PhaseTag | "">(phaseTag ?? "");

  function saveBrief(next?: { reason?: string; phase?: PhaseTag | "" }) {
    const reason = next?.reason ?? briefReason;
    const phase = next?.phase ?? briefPhase;
    void api.putReadingBrief(projectId, referenceId, {
      readingReason: reason,
      readingFocus: briefFocus,
      phaseTag: phase,
    });
  }

  // 完成这篇 (S2, Task 9): the finalize panel. draft holds the assembled
  // record (read-only) + seeded synthesis suggestions; leads/impact are the
  // student's own edits to that seed.
  const [finalizeOpen, setFinalizeOpen] = useState(false);
  const [finalizeLoading, setFinalizeLoading] = useState(false);
  const [finalizeDraft, setFinalizeDraft] = useState<TakeawayDraft | null>(null);
  const [finalizeLeads, setFinalizeLeads] = useState("");
  const [finalizeImpact, setFinalizeImpact] = useState("");
  const [finalizeSaving, setFinalizeSaving] = useState(false);
  const [finalizeDone, setFinalizeDone] = useState(false);

  // #8: the student's own freeform note on this source. Seeded from the
  // reference, saved on blur via onSaveNote (patchReference upstream). Opens
  // expanded when a note already exists so she sees it on re-entry.
  const [note, setNote] = useState(readingNote ?? "");
  const [noteOpen, setNoteOpen] = useState(Boolean(readingNote && readingNote.trim()));
  const [noteSaving, setNoteSaving] = useState(false);
  const [noteSavedAt, setNoteSavedAt] = useState(0);
  async function saveNote() {
    if (!onSaveNote) return;
    setNoteSaving(true);
    try {
      await onSaveNote(note);
      setNoteSavedAt(Date.now());
    } catch {
      /* keep her text so she can retry — never silently discard */
    } finally {
      setNoteSaving(false);
    }
  }

  async function openFinalize() {
    setFinalizeOpen(true);
    setFinalizeDone(false);
    setFinalizeLoading(true);
    try {
      const draftResult = await api.getTakeawayDraft(projectId, referenceId);
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
      await api.postFinalizeReading(projectId, referenceId, {
        newLeads: finalizeLeads
          .split("\n")
          .map((s) => s.trim())
          .filter(Boolean),
        proposalImpact: finalizeImpact.trim(),
      });
      setFinalizeDone(true);
      // #11: briefly show the ✓, then close the modal so she lands back on the
      // reading conversation instead of having to hunt for a 关闭 button.
      window.setTimeout(() => setFinalizeOpen(false), 900);
    } catch {
      // keep the panel open so she can retry — never silently discard her edits
    } finally {
      setFinalizeSaving(false);
    }
  }
  // 透镜库 (LensLibrary) — the student browses the reading deck and summons
  // a CHOSEN card onto the article herself, rather than only ever waiting
  // for the AI to propose one.
  const [libraryOpen, setLibraryOpen] = useState(false);
  // 引用原文 (focus context) — block ids the student has clicked to reference
  // in her next coach turn. Only meaningful while idle (a card in flight
  // repurposes the article for evidence-picking, not referencing).
  const [refs, setRefs] = useState<string[]>([]);

  function toggleRef(blockId: string) {
    setRefs((prev) => (prev.includes(blockId) ? prev.filter((id) => id !== blockId) : [...prev, blockId]));
  }

  const chatLogRef = useRef<HTMLDivElement | null>(null);
  const articleRef = useRef<HTMLDivElement | null>(null);

  const busyOrCarded = loop.busy || loop.status !== "idle";

  // Reinstates the reading-time logging that used to fire from
  // SourceDossier's open/close lifecycle (a Task 8 binding). ReadingRoom is
  // only ever mounted for exactly one source at a time (the container fully
  // swaps it out on 返回工作区/close), so a single mount-timestamp + unmount
  // report is sufficient; the null-out guard keeps it idempotent under
  // StrictMode's double-invoke.
  const openedAtRef = useRef<number | null>(null);
  useEffect(() => {
    openedAtRef.current = Date.now();
    return () => {
      const openedAt = openedAtRef.current;
      openedAtRef.current = null;
      if (openedAt == null || !onOpenLogged) return;
      const timeSpentS = Math.round((Date.now() - openedAt) / 1000);
      onOpenLogged(source.id, timeSpentS);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Keep the dialogue log pinned to the newest turn (chat affordance).
  useEffect(() => {
    const el = chatLogRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [loop.messages, loop.busy]);

  // Whenever a card is proposed, bring the article view forward so the newly
  // drawn example is visible — the coach "去文章看示范" also lands here.
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
        };

  // A graceful-degrade summon has no example block to hang under — fall back to
  // the first paragraph so the "pick your own sentence" card is always visible.
  // Normal cards always carry a real exampleBlockId, so this only affects the
  // no-example case (and hands off to studentBlockId the moment she picks).
  const cardBlockId = card
    ? anchorBlockId(card.exampleBlockId, card.studentBlockId, card.status) || (source.blocks[0]?.id ?? null)
    : null;

  function locateBlock(blockId: string) {
    setRightView("article");
    requestAnimationFrame(() => {
      articleRef.current?.querySelector(`[data-block-id="${blockId}"]`)?.scrollIntoView({ behavior: "smooth", block: "center" });
    });
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

  function send(text: string) {
    const t = text.trim();
    if (!t) return;
    setDraft("");
    const focusedSpans = refs.map((id) => ({ block_id: id, quote: source.blocks.find((b) => b.id === id)?.text ?? "" }));
    setRefs([]);
    void loop.sendTurn(t, focusedSpans);
  }

  return (
    <div className="mk-reading-room">
      <header className="mk-reading-room__topbar">
        <button type="button" className="mk-reading-room__back" onClick={() => onBack(finalizeDone)}>
          <BackIcon />
          返回工作区
        </button>
        <div className="mk-reading-room__brand">
          <span className="mk-reading-room__brand-name">思维印记 · 阅读工作台</span>
          <span className="mk-reading-room__brand-title">{source.title}</span>
        </div>
      </header>

      <main className="mk-reading-room__workspace">
        <section className="mk-reading-room__coach" aria-label="AI 对话工作区">
          <div className="mk-reading-room__brief">
            {briefEditingReason ? (
              <input
                autoFocus
                className="mk-reading-room__brief-input"
                value={briefReason}
                onChange={(e) => setBriefReason(e.target.value)}
                onBlur={() => {
                  setBriefEditingReason(false);
                  saveBrief();
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") e.currentTarget.blur();
                }}
                placeholder="说说你读这篇是为了什么……"
                aria-label="你读这篇是为了"
              />
            ) : (
              <button
                type="button"
                className="mk-reading-room__brief-reason"
                onClick={() => setBriefEditingReason(true)}
              >
                你读这篇是为了：
                {briefReason ? (
                  briefReason
                ) : (
                  <span className="mk-reading-room__brief-placeholder">点击填写…</span>
                )}
              </button>
            )}
            <select
              className="mk-reading-room__brief-phase"
              value={briefPhase}
              onChange={(e) => {
                const v = e.target.value as PhaseTag | "";
                setBriefPhase(v);
                saveBrief({ phase: v });
              }}
              aria-label="这篇材料用在哪个阶段"
            >
              <option value="">这篇用在哪个阶段…</option>
              {PhaseTag.options.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>

          <div className="mk-reading-room__pane-heading">
            <span className="mk-reading-room__kicker">AI 思维陪练</span>
            <h1>换一个视角，再读一遍</h1>
            <p>围绕原文对话；需要时，我会把一副短时透镜放进文章。</p>
          </div>

          <div className="mk-reading-room__chat-log" ref={chatLogRef} aria-live="polite">
            {loop.messages.map((m) =>
              m.role === "student" ? (
                <div key={m.id} className="mk-msg mk-msg--student">
                  <div className="mk-msg__content">
                    <div className="mk-msg__label">你</div>
                    {m.quotes && m.quotes.length > 0 && (
                      <div className="mk-msg__quotes">
                        {m.quotes.map((q, i) => (
                          <blockquote key={i} className="mk-msg__quote">
                            {q}
                          </blockquote>
                        ))}
                      </div>
                    )}
                    <div className="mk-msg__bubble">{m.body}</div>
                  </div>
                </div>
              ) : (
                <div key={m.id} className="mk-msg mk-msg--assistant">
                  <div className="mk-msg__avatar">印</div>
                  <div className="mk-msg__content">
                    <div className="mk-msg__label">思维陪练</div>
                    {m.kind === "lens" ? (
                      <div className="mk-msg__lens">
                        <div>
                          <strong>{m.cardName} 已就绪</strong>
                          <p>{m.body}</p>
                        </div>
                        <button type="button" onClick={() => cardBlockId && locateBlock(cardBlockId)}>
                          去文章看示范
                        </button>
                      </div>
                    ) : (
                      <div className="mk-msg__bubble">{m.body}</div>
                    )}
                  </div>
                </div>
              ),
            )}
            {loop.busy && (
              <div className="mk-msg mk-msg--assistant">
                <div className="mk-msg__avatar">印</div>
                <div className="mk-msg__content">
                  <div className="mk-msg__label">正在阅读与判断</div>
                  <div className="mk-msg__bubble mk-msg__thinking">
                    <i />
                    <i />
                    <i />
                  </div>
                </div>
              </div>
            )}
          </div>

          <div className="mk-reading-room__composer-wrap">
            {refs.length > 0 && (
              <div className="mk-reading-room__focus-context">
                <span>
                  正在引用 <strong>{refs.length}</strong> 处原文
                </span>
                <button type="button" onClick={() => setRefs([])}>
                  清除
                </button>
              </div>
            )}
            <form
              className="mk-reading-room__composer"
              onSubmit={(e) => {
                e.preventDefault();
                send(draft);
              }}
            >
              <textarea
                className="mk-reading-room__composer-input"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    send(draft);
                  }
                }}
                placeholder={busyOrCarded ? "先完成文章里的这副透镜…" : "说说你对哪一句有疑问…"}
                aria-label="输入你的问题"
                disabled={busyOrCarded}
              />
              <button
                type="submit"
                className="mk-reading-room__composer-send"
                aria-label="发送"
                disabled={busyOrCarded || !draft.trim()}
              >
                ↑
              </button>
            </form>
            <div className="mk-reading-room__starter-row">
              {STARTERS.map((prompt) => {
                // #7: the credibility starter deterministically summons the
                // source-check card (the reliable 透镜库 path) — a plain coach
                // turn is 克制-biased and rarely proposes one, so clicking
                // "这条来源可信吗？" used to surface nothing.
                const summonId = STARTER_SUMMON[prompt];
                return (
                  <button
                    key={prompt}
                    type="button"
                    onClick={() => (summonId ? void loop.summonCard(summonId) : send(prompt))}
                    disabled={busyOrCarded}
                  >
                    {prompt}
                  </button>
                );
              })}
              <button
                type="button"
                className="mk-reading-room__library-btn"
                onClick={() => setLibraryOpen(true)}
                disabled={busyOrCarded}
              >
                透镜库 · {READING_DECK_IDS.length}
              </button>
            </div>
          </div>
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
                  : refs.length > 0
                    ? `已引用 ${refs.length} 处 · 再点可取消`
                    : "点击句子可引用原文"}
            </span>
            <button type="button" className="mk-reading-room__finalize-btn" onClick={() => void openFinalize()}>
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
                  {/* #4 · bibliographic metadata line (author · year · journal),
                      recovered from a DOI via Crossref and persisted on the
                      reference — only rendered when we actually have some. */}
                  {bib && (bib.author || bib.year || bib.journal) && (
                    <div className="mk-reading-room__article-meta mk-reading-room__article-bib">
                      {bib.author && <span>{bib.author}</span>}
                      {bib.year && <span>{bib.year}</span>}
                      {bib.journal && <span className="mk-reading-room__journal">{bib.journal}</span>}
                    </div>
                  )}
                  {/* #5 · 打开原文: an honest external link to the source. The
                      readable extraction on the right IS "opening" the content;
                      this lets her open the live page in a new tab too. */}
                  {bib?.url && (
                    <a
                      className="mk-reading-room__source-link"
                      href={bib.url}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      打开原文 ↗
                    </a>
                  )}
                  {/* #4 · the abstract is CONTEXT, not the article body — a
                      collapsed 摘要 block so it never competes with the text. */}
                  {bib?.abstract && bib.abstract.trim() && (
                    <details className="mk-reading-room__abstract">
                      <summary>摘要</summary>
                      <p>{bib.abstract}</p>
                    </details>
                  )}
                </header>
                <Annotate
                  blocks={source.blocks}
                  state={{ material_id: source.id, spans }}
                  activeSpanId={null}
                  onSelectSpan={() => {}}
                  selectMode={loop.status === "active" ? { dimension: loop.cardName, onCancel: loop.repick } : null}
                  onCreateSpan={loop.pickSentence}
                  onReferenceBlock={loop.status === "idle" ? toggleRef : undefined}
                  referencedBlockIds={refs}
                  renderAfterBlock={(blockId) => {
                    if (!card || cardBlockId !== blockId) return null;
                    return (
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
                      />
                    );
                  }}
                />
                {onSaveNote && (
                  // A personal-note surface (never fed to evaluation) — taro-tinted
                  // to read as her own reflective space, distinct from the room's
                  // primary accent chrome (composer/CTAs) and from the article body.
                  <div className="mt-5 border-t border-mk-taro-bg pt-[14px]">
                    <button
                      type="button"
                      onClick={() => setNoteOpen((o) => !o)}
                      className="flex cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 font-sans text-[14px] font-bold text-mk-taro-fg"
                    >
                      <span
                        className="transition-transform duration-150 ease-mk"
                        style={{ transform: noteOpen ? "rotate(90deg)" : "none" }}
                      >
                        ▸
                      </span>
                      我的笔记{!noteOpen && note.trim() ? " ·  已记" : ""}
                    </button>
                    {noteOpen && (
                      <div className="mt-2">
                        <textarea
                          value={note}
                          onChange={(e) => setNote(e.target.value)}
                          onBlur={() => void saveNote()}
                          placeholder="随手记下你自己的想法、疑问、要引用的点——只属于你，不喂给评估。"
                          rows={4}
                          className="box-border w-full resize-y rounded-mk-sm border border-mk-taro-bg bg-mk-surface px-3 py-[10px] font-sans text-[14px] leading-[1.7] text-mk-ink outline-none"
                        />
                        <div className="mt-1 h-[14px] font-sans text-[12px] text-mk-muted">
                          {noteSaving ? "保存中…" : noteSavedAt ? "已保存" : "失焦自动保存"}
                        </div>
                      </div>
                    )}
                  </div>
                )}
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
        />
      )}
    </div>
  );
}
