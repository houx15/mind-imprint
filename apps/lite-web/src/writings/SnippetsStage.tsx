import { useEffect, useRef, useState } from "react";
import { Plus, HelpCircle, Eye, LayoutGrid, PanelLeftClose, PanelLeftOpen, ChevronLeft, ChevronRight, Check } from "lucide-react";
import { Button, EmptyState, Icon } from "@/ui";
import { countWords } from "@/workspace/blocks/wordcount";
import { wordUnit } from "./wordUnit";
import { useAlive } from "../shared/useAlive";
import { GuideBox } from "./GuideBox";
import { CommentPanel } from "./CommentPanel";
import { DeepenDrawer } from "./DeepenDrawer";
import { RoleBoard } from "./RoleBoard";
import { MiniMap } from "./MiniMap";
import { buildSlots, slotTitle, type Slot } from "./slots";
import { splitSentences, ROLE_BOARD_MIN } from "./sentences";
import { registerPendingSave } from "./pendingSaves";
import { handleWriteError } from "./writeErrors";
import {
  putWritingSnippet,
  guideWritingBlock,
  guideWritingBlocks,
  commentOnWritingSnippet,
  listWritingComments,
  type Comment,
  type WritingOutlineItem,
  type WritingSnippet,
  type WritingBlockGuide,
  type WritingBoardKind,
} from "../api/writingRoom";

/**
 * SnippetsStage — 段落，一张卡一段。
 *
 * 🚨 2026-09-18 产品负责人（重排这一页的原话）：
 *
 *   > I hope that we have the idea of card writing. each snippet is one card.
 *   > and then we will let our guidance be at the left side and can be folded.
 *   > and we have a paper-feeling large area with beautiful font that can write.
 *   > currently it is a small input textbox which cannot trigger my desire of writing.
 *
 * 所以这一页是三件东西：
 *
 *   - **左边：引导**（能折起来）—— 这一张卡要做的事、它要提出/回到的中心论点、
 *     结构图里给这一段准备的例子、印记的写作引导。只跟着**当前这张卡**走。
 *   - **中间：卡片 + 纸**。上面一排是这一篇的卡片（开头 · 分论点 · 结尾，
 *     slots.ts 从结构图派生），下面是当前这张卡的一张纸：衬线字、大行距、
 *     没有输入框的边框。一次只摊开一张 —— 一排八个小框让人不想写。
 *   - **右边：印记**，在房间那一层（WritingRoomHost），宽度能拖。
 *
 * 下面这几条老规矩都还在，只是换了位置：
 *
 * ## 引导一进来就在（Task 11）
 * 每一块的引导存在服务端、随 `GET /outline` 回来；第一次进段落、一条都还没有
 * 的时候，整篇一次批量生成。「换一组问题」只是再要一组。
 *
 * ## AI审阅这一段（B4）
 * 和成稿同一个 `CommentPanel`，意见落在纸的下面；点一条意见，纸上选中被引的那一句。
 *
 * ## 铁律①
 * GuideBox 只渲染 `guide.questions`，服务端已经把不是问句的都丢了
 * （writing_guide.go 的 parseWritingGuide）。卡片的标题只有她的节点文字和骨架的
 * 名字（开头 / 分论点 / 结尾）—— 一个字都不是印记写的。
 */

/** The guides the server already stored, keyed by outline row id. */
function storedGuides(outline: WritingOutlineItem[]): Record<string, WritingBlockGuide> {
  const out: Record<string, WritingBlockGuide> = {};
  for (const o of outline) {
    if (o.guide) out[o.id] = o.guide;
  }
  return out;
}

/** 一张卡在卡片叠里的身份。卡的顺序会随结构变，所以不用下标。 */
const slotKey = (s: Slot) => `${s.kind}-${s.outlineId ?? `p${s.position}`}`;

const hasText = (s: Slot) => (s.snippet?.text ?? "").trim() !== "";

/** 这一张卡要做的事 —— 骨架的说明，和她写什么无关，所以是写死的。 */
function slotJob(s: Slot): string {
  if (s.kind === "opening") return "提出这篇要证明的中心论点，让读者知道你要说什么、为什么值得读下去。";
  if (s.kind === "closing") return "回到中心论点，把它说得比开头更准；可以写读者读完应该带走的判断。";
  if (s.kind === "free") return "放在全文最后，也可以在成稿里挪到合适的位置。";
  if (s.needsPoint) return "下面的例子还没有对应的分论点。请先用一句话写出这些例子证明了什么，再展开例子。";
  return "先写出这条分论点，再用下面的例子证明它，最后说明例子和论点的关系。";
}

const FOLD_KEY = "lite:writingGuideFolded";
function readFolded(): boolean {
  try {
    return window.localStorage.getItem(FOLD_KEY) === "1";
  } catch {
    return false;
  }
}

export function SnippetsStage({
  writingId,
  outline,
  snippets,
  onSnippetsChange,
  lang,
  onGoToStructure,
  onGoToDraft,
  onSay,
  onLocked,
}: {
  writingId: string;
  outline: WritingOutlineItem[];
  snippets: WritingSnippet[];
  onSnippetsChange: (next: WritingSnippet[]) => void;
  /** 一块板摆完了：把结果当成她说的一句话发出去，印记 在右栏接住它。 */
  onSay: (text: string, board?: WritingBoardKind) => Promise<void>;
  /** 这一篇是中文还是英文 —— 决定篇幅按「字」还是「词」说。见 wordUnit.ts。 */
  lang: string;
  /** Sends her to 结构 from the empty state. */
  onGoToStructure: () => void;
  /** 去成稿。不是关卡 —— 顶上那条导航一直都能点。 */
  onGoToDraft: () => void;
  /** The deadline passed while she was mid-edit: reload into the locked page. */
  onLocked?: () => void;
}) {
  const slots = buildSlots(outline, snippets);

  // 当前摊开的那一张。默认是第一张还没写的；都写过了就第一张。
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const active =
    slots.find((s) => slotKey(s) === activeKey) ?? slots.find((s) => !hasText(s)) ?? slots[0] ?? null;
  const activeIndex = active ? slots.indexOf(active) : -1;
  // 🚨 定下来就钉住：不钉的话，「第一张还没写的」在她写完第一个字、存下去之后
  // 就变成了下一张，纸会在她手底下换掉。
  const resolvedKey = active ? slotKey(active) : null;
  useEffect(() => {
    if (activeKey === null && resolvedKey !== null) setActiveKey(resolvedKey);
  }, [activeKey, resolvedKey]);

  const [folded, setFolded] = useState(readFolded);
  function toggleFold() {
    setFolded((f) => {
      try {
        window.localStorage.setItem(FOLD_KEY, f ? "0" : "1");
      } catch {
        // 记不住就只是这一次有效。
      }
      return !f;
    });
  }

  /** 现在开着的是哪一张的标注板。一次只开一块（见 RoleBoard）。 */
  const [openBoardFor, setOpenBoardFor] = useState<string | null>(null);

  // 引导放在这一层：批量那一次调用一次回答整篇，每一张都要能收到自己那一份。
  const [guides, setGuides] = useState<Record<string, WritingBlockGuide>>(() => storedGuides(outline));
  useEffect(() => {
    setGuides((prev) => ({ ...storedGuides(outline), ...prev }));
  }, [outline]);

  const [batching, setBatching] = useState(false);
  const [batchError, setBatchError] = useState<string | null>(null);
  // One attempt per mount — a failed batch must not become a billed retry loop.
  const batchTried = useRef(false);
  // useAlive, NOT a per-invocation cancelled flag: see shared/useAlive.ts for
  // the StrictMode latch trap that hung this box for 180s on 2026-08-28.
  const alive = useAlive();

  const anyGuide = outline.some((o) => guides[o.id]);
  const needsBatch = outline.length > 0 && !anyGuide;

  useEffect(() => {
    if (!needsBatch || batchTried.current) return;
    batchTried.current = true;
    setBatching(true);
    void guideWritingBlocks(writingId)
      .then((next) => {
        if (alive.current) setGuides((prev) => ({ ...next, ...prev }));
      })
      .catch((err: unknown) => {
        if (alive.current) handleWriteError(err, onLocked, setBatchError);
      })
      .finally(() => {
        if (alive.current) setBatching(false);
      });
  }, [needsBatch, writingId, alive, onLocked]);

  const [deepen, setDeepen] = useState<{ outlineId: string; heading: string } | null>(null);

  // 每一段最新的那条意见，按片段 id。只取段落这一层的，成稿那一层归成稿。
  const [comments, setComments] = useState<Record<string, Comment>>({});
  useEffect(() => {
    let cancelled = false;
    void listWritingComments(writingId)
      .then((rows) => {
        if (cancelled) return;
        const byBlock: Record<string, Comment> = {};
        for (const c of rows) {
          if (c.scope !== "block" || !c.snippetId) continue;
          if (!byBlock[c.snippetId]) byBlock[c.snippetId] = c;
        }
        setComments(byBlock);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [writingId]);

  // Free paragraphs live in a position range an outline can never reach —
  // see the long note that used to sit here (writing_snippet upserts by
  // position; a free paragraph at N+1 would be overwritten by the next block).
  const FREE_POSITION_BASE = 1000;
  const freePositions = snippets.map((s) => s.position).filter((p) => p >= FREE_POSITION_BASE);
  const nextFreePosition = freePositions.length === 0 ? FREE_POSITION_BASE : Math.max(...freePositions) + 1;

  const [addError, setAddError] = useState<string | null>(null);
  async function addFreeParagraph() {
    setAddError(null);
    try {
      const next = await putWritingSnippet(writingId, { position: nextFreePosition, text: "" });
      onSnippetsChange(next);
      setActiveKey(`free-p${nextFreePosition}`);
    } catch (err) {
      handleWriteError(err, onLocked, (message) => setAddError(`添加失败：${message}`));
    }
  }

  if (slots.length === 0) {
    return (
      <div className="mk-scroll h-full overflow-y-auto p-6">
        <h2 className="sr-only">段落</h2>
        <EmptyState
          illustration="writing"
          title="还没有可以写的卡片"
          body="段落是跟着结构写的：开头、每一条分论点、结尾，各是一张卡片。请先去「结构」把思路理一理，或者直接加一段自由写。"
          action={{ label: "去理思路", onClick: onGoToStructure }}
        />
        <div className="mt-4 flex flex-col items-center gap-2">
          <Button variant="secondary" size="sm" onClick={() => void addFreeParagraph()} iconStart={<Icon icon={Plus} size={14} />}>
            加一段
          </Button>
          {addError && (
            <p role="alert" className="break-words text-mk-small text-mk-danger">
              {addError}
            </p>
          )}
        </div>
      </div>
    );
  }

  let pointNo = 0;
  const titles = slots.map((s) => slotTitle(s, s.kind === "point" ? ++pointNo : 0));
  const allWritten = slots.every(hasText);
  const someWritten = slots.some(hasText);
  const activeGuide = active?.outlineId ? (guides[active.outlineId] ?? null) : null;

  return (
    <div className="flex h-full min-h-0 flex-col lg:flex-row">
      {/* 读屏的人需要知道这是哪一步；看得见的人看卡片叠就知道。 */}
      <h2 className="sr-only">段落</h2>
      {/* ── 左：引导（可折叠） ─────────────────────────────── */}
      {folded ? (
        <div className="hidden shrink-0 flex-col items-center gap-2 border-r border-mk-border bg-mk-surface py-3 lg:flex lg:w-12">
          <button
            type="button"
            onClick={toggleFold}
            aria-label="展开引导"
            title="展开引导"
            className="rounded-mk-sm p-1.5 text-mk-secondary hover:bg-mk-accent-50 hover:text-mk-accent-700"
          >
            <Icon icon={PanelLeftOpen} size={18} />
          </button>
          <span className="text-mk-small text-mk-muted [writing-mode:vertical-rl]">引导</span>
        </div>
      ) : (
        <aside
          aria-label="引导"
          className="mk-scroll flex shrink-0 flex-col gap-4 overflow-y-auto border-b border-mk-border bg-mk-surface p-4 lg:w-[320px] lg:border-b-0 lg:border-r"
        >
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-mk-body font-semibold text-mk-ink">引导</h2>
            <button
              type="button"
              onClick={toggleFold}
              aria-label="收起引导"
              title="收起引导"
              className="hidden rounded-mk-sm p-1.5 text-mk-secondary hover:bg-mk-accent-50 hover:text-mk-accent-700 lg:block"
            >
              <Icon icon={PanelLeftClose} size={18} />
            </button>
          </div>

          {/* 🚨 那张思维导图留在这里 —— 同事 2026-09-20 在截图上画了个箭头
              指着左栏顶部：「我觉得可以在这里保留刚刚的思维导图，然后把引导
              往下放」。到了这一步她眼前只剩一张卡和一张纸，「这一段在整篇里
              是第几块」只能靠记。只读；要改结构回上一步改。 */}
          <MiniMap outline={outline} />

          {active && (
            <CardGuidance
              key={slotKey(active)}
              writingId={writingId}
              slot={active}
              title={titles[activeIndex] ?? ""}
              guide={activeGuide}
              batching={batching}
              batchError={batchError}
              onDismissBatchError={() => setBatchError(null)}
              onGuide={(next) => {
                const oid = active.outlineId;
                if (oid) setGuides((prev) => ({ ...prev, [oid]: next }));
              }}
              onDeepen={() => {
                if (active.outlineId) setDeepen({ outlineId: active.outlineId, heading: active.heading || titles[activeIndex] || "" });
              }}
              onLocked={onLocked}
            />
          )}
        </aside>
      )}

      {/* ── 中：卡片 + 纸 ─────────────────────────────────── */}
      <div className="mk-scroll flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto" style={{ background: "var(--mk-surface-2, var(--mk-surface))" }}>
        <nav aria-label="卡片" className="flex shrink-0 gap-2 overflow-x-auto px-4 pb-2 pt-4 sm:px-8">
          {slots.map((s, i) => {
            const on = s === active;
            const done = hasText(s);
            return (
              <button
                key={slotKey(s)}
                type="button"
                data-write-card={titles[i]}
                aria-current={on ? "true" : undefined}
                onClick={() => setActiveKey(slotKey(s))}
                className={`flex w-[148px] shrink-0 flex-col gap-1 rounded-mk-md border px-3 py-2 text-left transition-[transform,box-shadow,border-color] duration-[160ms] ease-mk hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 ${
                  on ? "border-mk-accent-500 shadow-mk-md" : "border-mk-border shadow-mk-sm"
                }`}
                style={{ background: on ? "var(--mk-paper)" : "var(--mk-surface)" }}
              >
                <span className="flex items-center justify-between gap-1 text-mk-label">
                  <span className={`font-semibold ${on ? "text-mk-accent-700" : "text-mk-secondary"}`}>
                    {i + 1} · {titles[i]}
                  </span>
                  {done && (
                    <span
                      className="flex h-4 w-4 items-center justify-center rounded-mk-full"
                      style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
                      aria-label="已写"
                    >
                      <Icon icon={Check} size={11} />
                    </span>
                  )}
                </span>
                <span className="line-clamp-2 text-mk-small text-mk-ink">
                  {s.heading || s.claim || (s.materials[0] ?? "") || "待写"}
                </span>
                <span className="text-mk-label text-mk-faint">
                  {done ? `${countWords(s.snippet?.text ?? "")} ${wordUnit(lang)}` : "待写"}
                </span>
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => void addFreeParagraph()}
            className="flex w-[92px] shrink-0 flex-col items-center justify-center gap-1 rounded-mk-md border border-dashed border-mk-border px-2 py-2 text-mk-small text-mk-secondary hover:border-mk-accent-300 hover:text-mk-accent-700"
          >
            <Icon icon={Plus} size={16} />
            加一段
          </button>
        </nav>
        {addError && (
          <p role="alert" className="break-words px-4 text-mk-small text-mk-danger sm:px-8">
            {addError}
          </p>
        )}

        {active && (
          <CardPaper
            key={slotKey(active)}
            writingId={writingId}
            slot={active}
            title={titles[activeIndex] ?? ""}
            comment={active.snippet ? (comments[active.snippet.id] ?? null) : null}
            onCommented={(c) => {
              const sid = c.snippetId;
              if (sid) setComments((prev) => ({ ...prev, [sid]: c }));
            }}
            onSaved={onSnippetsChange}
            onSay={onSay}
            lang={lang}
            boardOpen={openBoardFor === slotKey(active)}
            onBoardOpen={(on) => setOpenBoardFor(on ? slotKey(active) : null)}
            onLocked={onLocked}
            prev={activeIndex > 0 ? () => setActiveKey(slotKey(slots[activeIndex - 1]!)) : undefined}
            next={activeIndex < slots.length - 1 ? () => setActiveKey(slotKey(slots[activeIndex + 1]!)) : undefined}
            nextTitle={titles[activeIndex + 1]}
          />
        )}

        {/* 每一张都写好了 → 一条真的邀请（2026-09-11：她两次写完停在这儿，
            没把顶上的「成稿」读成下一步）。写了一部分 → 直说空卡不进成稿。 */}
        <div className="mx-auto w-full max-w-[760px] px-4 pb-8 sm:px-8">
          {allWritten ? (
            <div
              className="flex flex-wrap items-center gap-3 rounded-mk-lg border p-3"
              style={{
                background: "color-mix(in srgb, var(--mk-accent-50) 80%, var(--mk-surface))",
                borderColor: "color-mix(in srgb, var(--mk-accent-500) 30%, transparent)",
              }}
            >
              <span className="text-mk-body text-mk-ink">每一张卡片都写好了。下一步把它们拼成整篇。</span>
              <Button size="sm" onClick={onGoToDraft}>
                去成稿
              </Button>
            </div>
          ) : (
            someWritten && (
              <div className="flex flex-wrap items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-3">
                <span className="text-mk-small text-mk-secondary">
                  需要的段落写完后，请到「成稿」把它们拼成整篇，并在那里点「完成这篇」提交。空着的卡片不会进入成稿。
                </span>
                <Button size="sm" variant="secondary" onClick={onGoToDraft}>
                  去成稿
                </Button>
              </div>
            )
          )}
        </div>
      </div>

      {deepen && (
        <DeepenDrawer
          writingId={writingId}
          outlineId={deepen.outlineId}
          heading={deepen.heading}
          onClose={() => setDeepen(null)}
          onLocked={onLocked}
        />
      )}
    </div>
  );
}

/**
 * 左栏：当前这张卡的引导。
 *
 * 写过字的卡进来时引导是折着的（2026-09-13 第十三轮：那几个问题是对着白纸写的，
 * 她写完三百字后还挂着，读起来像过期的反馈）。空卡照旧摊开 —— 最需要引导的学生
 * 正是最不会主动去点的那个。
 */
function CardGuidance({
  writingId,
  slot,
  title,
  guide,
  batching,
  batchError,
  onDismissBatchError,
  onGuide,
  onDeepen,
  onLocked,
}: {
  writingId: string;
  slot: Slot;
  title: string;
  guide: WritingBlockGuide | null;
  batching: boolean;
  batchError: string | null;
  onDismissBatchError: () => void;
  onGuide: (next: WritingBlockGuide) => void;
  onDeepen: () => void;
  onLocked?: () => void;
}) {
  const [collapsed, setCollapsed] = useState(() => hasText(slot));
  const [guiding, setGuiding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function regenerate() {
    if (!slot.outlineId) return;
    setGuiding(true);
    setError(null);
    try {
      onGuide(await guideWritingBlock(writingId, slot.outlineId));
      setCollapsed(false);
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setGuiding(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-1">
        <span className="text-mk-label font-semibold text-mk-accent-700">{title}</span>
        <p className="text-mk-body text-mk-ink">{slotJob(slot)}</p>
      </section>

      {slot.claim && (
        <section className="flex flex-col gap-1">
          <h3 className="text-mk-label font-semibold text-mk-secondary">中心论点</h3>
          <p className="font-mk-piece text-mk-body-lg text-mk-ink">{slot.claim}</p>
        </section>
      )}

      {slot.kind === "point" && slot.heading && (
        <section className="flex flex-col gap-1">
          <h3 className="text-mk-label font-semibold text-mk-secondary">分论点</h3>
          <p className="font-mk-piece text-mk-body-lg text-mk-ink">{slot.heading}</p>
        </section>
      )}

      {slot.materials.length > 0 && (
        <section className="flex flex-col gap-1.5">
          <h3 className="text-mk-label font-semibold text-mk-secondary">这一段的材料</h3>
          <ul className="flex list-none flex-col gap-1.5">
            {slot.materials.map((m, i) => (
              <li
                key={i}
                className="rounded-mk-sm border border-mk-border px-2.5 py-1.5 text-mk-small text-mk-ink"
                style={{ background: "var(--mk-paper)" }}
              >
                {m}
              </li>
            ))}
          </ul>
        </section>
      )}

      {batching && (
        <p className="text-mk-small text-mk-muted" role="status">
          印记正在为每一张卡片准备引导…
        </p>
      )}
      {batchError && (
        <div role="alert" className="rounded-mk-sm px-3 py-2 text-mk-small text-mk-danger" style={{ background: "var(--mk-danger-bg)" }}>
          {batchError}
          <button type="button" className="ml-3 underline" onClick={onDismissBatchError}>
            知道了
          </button>
        </div>
      )}

      {/* 自由段落和虚拟的开头/结尾卡没有结构图节点，服务端的引导是按节点存的，
          所以这里不摆一颗点了也拿不到引导的按钮；上面那句「要做的事」就是它的引导。 */}
      {slot.outlineId && guide && !collapsed && (
        <GuideBox guide={guide} kind={slot.outlineKind} onDismiss={() => setCollapsed(true)} onDeepen={onDeepen} />
      )}
      {slot.outlineId && (guide === null || collapsed) && (
        <div className="flex flex-wrap gap-2">
          {guide !== null ? (
            <Button variant="secondary" size="sm" onClick={() => setCollapsed(false)}>
              打开引导
            </Button>
          ) : (
            !batching && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => void regenerate()}
                loading={guiding}
                iconStart={<Icon icon={HelpCircle} size={14} />}
              >
                获取引导
              </Button>
            )
          )}
        </div>
      )}
      {slot.outlineId && guide && !collapsed && (
        <Button
          variant="ghost"
          size="sm"
          className="w-fit"
          onClick={() => void regenerate()}
          loading={guiding}
          iconStart={<Icon icon={HelpCircle} size={14} />}
        >
          换一组问题
        </Button>
      )}
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}
    </div>
  );
}

/**
 * 中间：当前这张卡的那一张纸。
 *
 * 一张大纸、衬线字、没有框线 —— 「a paper-feeling large area with beautiful font」。
 * 字号行距取 `mk-piece` 那一套（成稿、报告里她的文章也用这一个字体），
 * 所以她在这里写下的字和最后成稿里的字长得一样。
 */
function CardPaper({
  writingId,
  slot,
  title,
  comment,
  onCommented,
  onSaved,
  onSay,
  boardOpen,
  onBoardOpen,
  lang,
  onLocked,
  prev,
  next,
  nextTitle,
}: {
  writingId: string;
  slot: Slot;
  title: string;
  comment: Comment | null;
  onCommented: (next: Comment) => void;
  onSaved: (next: WritingSnippet[]) => void;
  onSay: (text: string, board?: WritingBoardKind) => Promise<void>;
  boardOpen: boolean;
  onBoardOpen: (on: boolean) => void;
  lang: string;
  onLocked?: () => void;
  prev?: () => void;
  next?: () => void;
  nextTitle?: string;
}) {
  const [text, setText] = useState(slot.snippet?.text ?? "");
  const [saving, setSaving] = useState(false);
  const [commenting, setCommenting] = useState(false);
  const [boardBusy, setBoardBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sentenceCount = splitSentences(text).length;
  // 「已保存」＝ 服务端那一行的字和纸上的字一模一样。
  const saved = text.trim() !== "" && slot.snippet?.text === text;
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const commentRef = useRef<HTMLDivElement | null>(null);

  // 纸跟着字变高 —— 一张纸不该在里面再滚一层。
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.max(el.scrollHeight, 360)}px`;
  }, [text]);

  useEffect(() => {
    // 🚨 她手正放在纸上的时候，绝不把服务端那一份写回去（第一次存会把片段行建出来，
    // snippet id 从无到有，这里会跑一次 —— 不挡的话会吞掉往返期间她又敲的字）。
    if (textareaRef.current !== null && document.activeElement === textareaRef.current) return;
    setText(slot.snippet?.text ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slot.snippet?.id]);

  // 打字停下来就存（2026-09-12：顶上「已写 0」和这一段「85 字」并排打脸）。
  useEffect(() => {
    if (text === (slot.snippet?.text ?? "")) return;
    const t = setTimeout(() => void save(), 1200);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, slot.snippet?.text]);

  // 她一敲完就问印记：say() 先等这一张存完（pendingSaves.ts）。
  useEffect(() => {
    return registerPendingSave(async () => {
      if (text === (slot.snippet?.text ?? "")) return;
      await save();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, slot.snippet?.text]);

  // 🚨 换到另一张卡时这张纸会卸掉，而防抖的那 1.2 秒也跟着没了。
  // 卸掉之前把还没存的字存下去 —— 丢她写的字是这个房间最不能犯的错。
  const latest = useRef({ text, stored: slot.snippet?.text ?? "" });
  latest.current = { text, stored: slot.snippet?.text ?? "" };
  useEffect(() => {
    return () => {
      if (latest.current.text !== latest.current.stored) void save(latest.current.text);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function save(body: string = text): Promise<WritingSnippet | null> {
    setSaving(true);
    setError(null);
    try {
      const rows = await putWritingSnippet(writingId, {
        // Only send outlineId to ESTABLISH a link on this card's first save;
        // after that, absent = keep whatever link the server holds.
        outlineId: slot.snippet ? undefined : slot.outlineId,
        position: slot.position,
        text: body,
      });
      onSaved(rows);
      return rows.find((s) => (slot.outlineId ? s.outlineId === slot.outlineId : s.position === slot.position)) ?? null;
    } catch (err) {
      handleWriteError(err, onLocked, setError);
      return null;
    } finally {
      setSaving(false);
    }
  }

  async function askForComment() {
    if (!text.trim()) {
      setError("这一段还没有内容，请先写再审阅。");
      return;
    }
    setCommenting(true);
    setError(null);
    try {
      const row = slot.snippet && slot.snippet.text === text ? slot.snippet : await save();
      if (!row) return;
      onCommented(await commentOnWritingSnippet(writingId, row.id));
      // 意见落在纸的下面 —— 纸长的时候它在屏幕外，要把她带过去（同成稿的 bug 3）。
      requestAnimationFrame(() => commentRef.current?.scrollIntoView?.({ behavior: "smooth", block: "start" }));
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setCommenting(false);
    }
  }

  /** 点一条意见 → 纸上选中被引的那一句。找不到就什么都不做（别选错一句）。 */
  function trace(quote: string) {
    const el = textareaRef.current;
    if (!el) return;
    const at = el.value.indexOf(quote);
    if (at < 0) return;
    el.focus();
    el.setSelectionRange(at, at + quote.length);
  }

  const heading = slot.kind === "point" ? slot.heading : "";

  return (
    <div data-write-block={title} className="mx-auto flex w-full max-w-[760px] flex-col gap-3 px-4 pb-4 pt-2 sm:px-8">
      <article
        className="rounded-mk-lg border border-mk-border shadow-mk-md"
        style={{ background: "var(--mk-paper)" }}
      >
        <header className="flex flex-wrap items-baseline justify-between gap-2 px-6 pt-6 sm:px-12 sm:pt-10">
          <div className="min-w-0">
            <p className="text-mk-label font-semibold text-mk-accent-700">
              第 {slot.number} 张 · {title}
            </p>
            {heading && <h3 className="mt-1 font-mk-piece text-mk-h2 text-mk-ink">{heading}</h3>}
            {slot.kind !== "point" && slot.claim && (
              <p className="mt-1 font-mk-piece text-mk-body text-mk-secondary">{slot.claim}</p>
            )}
          </div>
        </header>
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onBlur={() => {
            if (text !== (slot.snippet?.text ?? "")) void save();
          }}
          placeholder="写这一段……"
          spellCheck={false}
          className="block w-full resize-none border-none bg-transparent px-6 pb-10 pt-4 font-mk-piece text-mk-ink outline-none placeholder:text-[#B8ADA2] focus:outline-none focus:ring-0 sm:px-12"
          style={{ fontSize: "18px", lineHeight: 2, minHeight: 360, caretColor: "var(--mk-accent-500)" }}
        />
      </article>

      {/* 这一段交上去了没有：状态留在屏幕上，还有一颗真的按钮（2026-09 走查两次找「发送」）。 */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-mk-small text-mk-faint">
          {saving ? (
            <span>处理中…</span>
          ) : saved ? (
            <span>
              已保存 · {countWords(text)} {wordUnit(lang)}
            </span>
          ) : text.trim() !== "" ? (
            <>
              <Button variant="secondary" size="sm" onClick={() => void save()}>
                保存这一段
              </Button>
              <span>
                {countWords(text)} {wordUnit(lang)}
              </span>
            </>
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void askForComment()}
            loading={commenting}
            iconStart={<Icon icon={Eye} size={14} />}
          >
            AI审阅这一段
          </Button>
          {prev && (
            <Button variant="ghost" size="sm" onClick={prev} aria-label="上一张" iconStart={<Icon icon={ChevronLeft} size={14} />}>
              上一张
            </Button>
          )}
          {next && (
            <Button variant="ghost" size="sm" onClick={next} iconEnd={<Icon icon={ChevronRight} size={14} />}>
              下一张{nextTitle ? `：${nextTitle}` : ""}
            </Button>
          )}
        </div>
      </div>
      {commenting && (
        <p role="status" className="text-mk-small text-mk-muted">
          印记正在读这一段…
        </p>
      )}
      {error && <p role="alert" className="text-mk-small text-mk-danger">{error}</p>}

      {/* 标注板：她写出两句以上才递过来，长在这一段下面（屏幕上有它 ≠ 她手上有它）。 */}
      {!boardOpen && sentenceCount >= ROLE_BOARD_MIN && (
        <div className="flex flex-wrap items-center gap-2 rounded-mk-sm border border-mk-border bg-mk-surface px-3 py-2">
          <span className="text-mk-body text-mk-muted">
            这一段有 {sentenceCount} 句。标一下每一句在做什么，就看得出缺了哪一种。
          </span>
          <Button variant="secondary" size="sm" onClick={() => onBoardOpen(true)} iconStart={<Icon icon={LayoutGrid} size={14} />}>
            标一下这一段
          </Button>
        </div>
      )}
      {boardOpen && sentenceCount >= ROLE_BOARD_MIN && (
        <RoleBoard
          text={text}
          snippetId={slot.snippet?.id ?? `pos-${slot.position}`}
          heading={heading || title}
          busy={boardBusy}
          onCancel={() => onBoardOpen(false)}
          onSubmit={(message) => {
            setBoardBusy(true);
            void onSay(message, "role")
              .then(() => onBoardOpen(false))
              .catch(() => setError("发送失败，请再试一次。"))
              .finally(() => setBoardBusy(false));
          }}
        />
      )}

      {comment && (
        <div ref={commentRef}>
          <CommentPanel
            comment={comment}
            onTrace={trace}
            currentText={text}
            onRecheck={() => void askForComment()}
            rechecking={commenting}
          />
        </div>
      )}
    </div>
  );
}
