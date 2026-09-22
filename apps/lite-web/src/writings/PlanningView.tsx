import { StudentCoachHeading } from "../learning/StudentCoachHeading";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ArrowRight, FileUp } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { Composer } from "@/studio/ai/Composer";
import { LiteChatMarkdown as ChatMarkdown } from "../readings/LiteChatMarkdown";
import {
  postWritingPlanTurn,
  postWritingOpening,
  putWritingOutline,
  type WritingOutlineItem,
} from "../api/writingRoom";
import type { LiteMessage } from "../api/readingRoom";
import type { Writing } from "../api/writings";
import { useAlive } from "../shared/useAlive";
import { EditableTitle } from "./EditableTitle";
import { AssignmentLine } from "../inbox/AssignmentLine";
import { AssignedPromptLine } from "./AssignedPromptLine";
import { MindMap } from "./MindMap";
import { planShapeLine, planShapeOf } from "./planShape";
import { moveOutlineNode, type OutlineMoveMode } from "./outlineMove";
import { outlineKindOf } from "./outlineKind";
import { handleWriteError } from "./writeErrors";
import { coachOpeningNeeded } from "./openingRule";

/**
 * 把一行提纲整理成 PUT 要的那个形状。
 *
 * 🚨 kind 一定要带上，而且老行要现算一个（outlineKindOf）：服务端按它算深度，
 * 漏掉就等于把这一行交还给关键词猜测，她拖的那一下会被静默撤销。
 */
function outlineToReq(n: WritingOutlineItem) {
  return { text: n.text, role: n.role, kind: outlineKindOf(n), depth: n.depth, source: n.source };
}

/**
 * PlanningView — 结构, as a full-screen planning conversation.
 *
 * This replaced a template picker on 2026-08-27. The product call:
 *
 *   > 结构 is like planning, is not a fixed one, but guide students to think,
 *   > then compose the structure.
 *
 * So: 印记 asks one question at a time, she answers, and what she says grows
 * onto a mind map at the right. The map is `writing_outline` rendered as a
 * tree, so when she goes to write, the thing she planned IS the outline — no
 * conversion, nothing lost in between.
 *
 * LAYOUT, as specified: full screen while planning. The chat starts centred
 * and alone; the moment the map has anything on it the view splits, chat left
 * and map right — the same shape as Claude opening an artifact panel. An empty
 * panel sitting there from the first second would be a promise the screen
 * hasn't kept yet.
 *
 * NOT A GATE. 「去写」 is available from the first render. A student who
 * already knows what she wants to say should not have to talk her way past a
 * planning screen to reach the page — 铁律②, and the reason planning is a
 * surface rather than a checkpoint.
 */
export function PlanningView({
  writing,
  messages,
  outline,
  onRenamed,
  onMessages,
  onOutline,
  onDone,
  onUpload,
  onBack,
  onLocked,
  banner,
}: {
  writing: Writing;
  messages: LiteMessage[];
  outline: WritingOutlineItem[];
  /** The title is editable in every step, 结构 included — this is how the new
   *  one reaches the rest of the room. */
  onRenamed?: (next: Writing) => void;
  onMessages: (next: LiteMessage[]) => void;
  onOutline: (next: WritingOutlineItem[]) => void;
  /** Leaves planning for 段落. */
  onDone: () => void;
  /** Leaves planning for 成稿, where she can upload a piece written elsewhere. */
  onUpload?: () => void;
  onBack: () => void;
  /** The deadline passed while she was revising and jumped back to 结构 (顶
   *  上那排 结构/段落/成稿 一直可点，不是关卡): every write below reloads
   *  the room into the locked finished page instead of showing a raw error
   *  on a piece she can no longer edit. */
  onLocked?: () => void;
  /** Shown under the header: the revising strip while she edits a finished
   *  writing from 结构 (spec A2). */
  banner?: ReactNode;
}) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [justAdded, setJustAdded] = useState<string[]>([]);
  /** 印记 已经说过这份计划够开始写了 —— 见下面那块邀请的注释。 */
  const [planReady, setPlanReady] = useState(false);
  // Seq numbers for optimistic local turns. Real rows come back from the
  // server on the next load; these only need to be unique and negative so
  // they can never collide with a persisted seq.
  const localSeq = useRef(-1);

  const hasMap = outline.length > 0;

  /**
   * 印记 opens. Same idempotent endpoint the room uses — calling it twice
   * replays rather than greeting her again, so a refresh mid-flight is free.
   */
  const openingNeeded = coachOpeningNeeded(writing, messages);
  /**
   * Fire once per writing, and apply the answer unless she has really left.
   *
   * Unlike the room's own opening effect, this one is already `openingNeeded`
   * at mount — so StrictMode's mount → cleanup → remount used to send TWO
   * concurrent `POST /opening` calls. The server's idempotency gate is a
   * read-then-write with no lock, so both saw an empty transcript, both billed
   * a model call, and both appended an 'ai' row: she was charged twice and
   * greeted twice on her next load.
   *
   * The fix is NOT a bare `useRef` latch on its own. A latch plus the old
   * per-invocation `let cancelled = false` cleanup flag is the exact
   * combination that hung 段落's guide box (see `shared/useAlive.ts`): the
   * cleanup cancels pass 1's closure, the latch skips pass 2, and the one
   * real reply is thrown away by the only closure left watching it. Latch +
   * `alive` keeps both properties — one call, and its answer always lands.
   */
  const openedFor = useRef<string | null>(null);
  const alive = useAlive();
  useEffect(() => {
    if (!openingNeeded || openedFor.current === writing.id) return;
    openedFor.current = writing.id;
    setOpening(true);
    void postWritingOpening(writing.id)
      .then((res) => {
        if (!alive.current) return;
        const reply = res.reply.trim();
        if (reply) onMessages([...messages, { seq: --localSeq.current, role: "ai", content: reply, createdAt: "" }]);
      })
      .catch((err: unknown) => {
        if (!alive.current) return;
        handleWriteError(err, onLocked, setError);
      })
      .finally(() => {
        if (alive.current) setOpening(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openingNeeded, writing.id, onLocked]);

  const chatMessages: ChatMessage[] = useMemo(
    () =>
      messages
        .filter((m) => m.role === "student" || m.role === "ai")
        .map((m) => ({
          id: `p${m.seq}`,
          role: m.role === "ai" ? "assistant" : "student",
          node: m.role === "ai" ? <ChatMarkdown text={m.content} /> : m.content,
        })),
    [messages],
  );

  async function send() {
    const text = draft.trim();
    if (!text || sending) return;
    setDraft("");
    setSending(true);
    setError(null);
    const optimistic: LiteMessage = { seq: --localSeq.current, role: "student", content: text, createdAt: "" };
    const withStudent = [...messages, optimistic];
    onMessages(withStudent);
    try {
      const turn = await postWritingPlanTurn(writing.id, text);
      onMessages([...withStudent, { seq: --localSeq.current, role: "ai", content: turn.reply, createdAt: "" }]);
      onOutline(turn.outline);
      setJustAdded(turn.addedIds);
      // 一旦 印记 说过「够写了」，就一直算数：她可能想再补一条理由再走，那不
      // 该把邀请收回去。只有 false → true，没有反向。
      if (turn.ready) setPlanReady(true);
    } catch (err) {
      handleWriteError(err, onLocked, setError);
      onMessages(messages);
      setDraft(text);
    } finally {
      setSending(false);
    }
  }

  /**
   * Her own edits to the map. These go through the full-replace PUT — the
   * one path that CAN change or remove a node, and it is only ever reachable
   * from her hands. The planning turn has no such call.
   *
   * 🚨 **每一条都必须带着 kind 回传。** 0182 起服务端按 kind 算深度
   *（buildWritingOutlineArrays），漏掉它会让每一次保存都退回按 role 猜 ——
   * 她刚拖出来的那个节点会被悄悄放回去，而屏幕上看不出发生过什么。
   */
  async function mutate(next: { text: string; role: string; kind?: string; depth: number; source?: string }[]) {
    try {
      onOutline(await putWritingOutline(writing.id, next));
      setJustAdded([]);
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    }
  }

  function removeNode(id: string) {
    const ordered = outline.slice().sort((a, b) => a.position - b.position);
    const target = ordered.find((n) => n.id === id);
    if (!target) return;
    // Removing a node takes its whole subtree with it: the rows that follow
    // it while staying deeper than it ARE its children (the flattened-tree
    // convention), and leaving them behind would reparent her material under
    // whatever happened to precede it.
    const keep: typeof ordered = [];
    let skipping = false;
    for (const n of ordered) {
      if (n.id === id) {
        skipping = true;
        continue;
      }
      if (skipping) {
        if (n.depth > target.depth) continue;
        skipping = false;
      }
      keep.push(n);
    }
    void mutate(keep.map(outlineToReq));
  }

  /**
   * 她把一个节点拖到另一个节点上：那一个连着它底下的东西，挂到这一个下面。
   *
   * 算新清单的是纯函数（outlineMove.ts），这里只负责落库。算不出来
   *（拖到自己身上、拖进自己底下、超过深度上限）就**什么都不做** ——
   * 一次非法的拖动不该变成一次让图变形的写入。
   */
  function moveNode(draggedId: string, targetId: string, mode: OutlineMoveMode) {
    const next = moveOutlineNode(outline, draggedId, targetId, mode);
    if (!next) return;
    void mutate(next.map(outlineToReq));
  }

  function editNode(id: string, text: string) {
    void mutate(
      outline
        .slice()
        .sort((a, b) => a.position - b.position)
        .map((n) => ({ ...outlineToReq(n), text: n.id === id ? text : n.text })),
    );
  }

  return (
    <div className="student-planning-room flex h-full w-full flex-col bg-mk-paper">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-mk-border px-5 py-3">
        <div className="flex min-w-0 flex-col">
          <button type="button" onClick={onBack} className="w-fit text-mk-small text-mk-muted hover:text-mk-accent-700">
            ← 我的写作
          </button>
          <EditableTitle writingId={writing.id} title={writing.title} onRenamed={onRenamed} onLocked={onLocked} />
          {/* Same line as the room header: an assigned writing starts here,
              in 结构, so the deadline has to show before she reaches 去写. */}
          <AssignmentLine atomId={writing.id} className="mt-0.5 block text-mk-small text-mk-muted" />
          <AssignedPromptLine writing={writing} />
        </div>
        <div className="flex items-center gap-3">
          <span className="text-mk-small text-mk-muted">先想清楚，再动笔</span>
          {/* 在别处写完的（尤其是作业）直接去成稿上传，不用先走结构和段落。 */}
          {onUpload && (
            <Button variant="ghost" onClick={onUpload} iconStart={<Icon icon={FileUp} size={14} />}>
              上传写好的文章
            </Button>
          )}
          <Button onClick={onDone} iconEnd={<Icon icon={ArrowRight} size={14} />}>
            去写
          </Button>
        </div>
      </header>

      {banner && <div className="shrink-0 px-5 pt-3">{banner}</div>}

      {error && (
        <div role="alert" className="shrink-0 px-5 py-2 text-mk-small text-mk-danger" style={{ background: "var(--mk-danger-bg)" }}>
          {error}
          <button type="button" className="ml-3 underline" onClick={() => setError(null)}>
            知道了
          </button>
        </div>
      )}

      <div className={hasMap ? "grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(440px,44%)]" : "flex min-h-0 flex-1 justify-center"}>
        <div className={hasMap ? "flex min-h-0 flex-col" : "flex min-h-0 w-full max-w-[720px] flex-col"}>
          <div className="px-5 pt-4"><StudentCoachHeading label="写作构思" /></div>
          <ChatLog messages={chatMessages} thinking={sending || opening} className="mk-scroll min-h-0 flex-1 px-5 py-4" />
          {/*
            印记 判断这份计划够写了 → 屏幕上出现一条真的邀请。

            🚨 这是「永远不会带领学生真正开启写作吗？」的答案。产品负责人的原话：
            「until student click the logic is good, ai never auto triggers and
            guides students to start writing.」——她说得准：「去写」从第一秒就
            在页眉里可点，从来不是关卡；但**没有任何一刻有人提议过它**。
            于是一个已经想清楚的学生会继续回答下一个问题，一直到自己放弃。

            服务端现在有了说这句话的通道（`writingPlanReply.Ready`，
            writing_plan.go），这里是它落地的地方：在她眼睛所在的位置（对话流
            的末尾，输入框正上方），而不是页眉里那颗一直都在的按钮。

            ⚠️ 它**不替她走**。结构不是关卡，反向也不是：把她推进段落，是同一个
            错误的镜像。这里只是把邀请说出口，按不按仍然是她的事——而且页眉那颗
            「去写」一直都在，随时可以不理这块直接走。
          */}
          {planReady && (
            <div className="shrink-0 px-5 pb-3">
              <div
                className="flex flex-col gap-2 rounded-mk-lg border p-3"
                style={{
                  // mk-* 是裸 CSS 变量：Tailwind 的 alpha 语法对它们一个字节的
                  // CSS 都不生成，半透明只能走 color-mix。
                  background: "color-mix(in srgb, var(--mk-accent-50) 80%, var(--mk-surface))",
                  borderColor: "color-mix(in srgb, var(--mk-accent-500) 30%, transparent)",
                }}
              >
                <p className="text-mk-label text-mk-accent-700">计划已可开始写作</p>
                <p className="text-mk-small leading-relaxed text-mk-ink">
                  当前思路已可用于起草。开头与结尾可以在主体段完成后再确定。
                </p>
                {/* 🚨 它凭什么说够了 —— 把数出来的那几个数摆出来。
                    产品负责人 2026-09-12 的原话：「AI 就判断已足以支撑一篇文章，
                    **判断依据不清晰**。」原来这块绿框只有上面那一句，没有一个字
                    说它数了什么，于是她既没法判断该不该信，也看不出自己还差什么。
                    这一行只报事实（几条），「够不够」那条判据留在服务端，
                    见 planShape.ts 里那段。 */}
                <p className="text-mk-small text-mk-secondary">{planShapeLine(planShapeOf(outline))}</p>
                <div className="flex justify-end">
                  <Button onClick={onDone} iconEnd={<Icon icon={ArrowRight} size={14} />}>
                    开始写作
                  </Button>
                </div>
              </div>
            </div>
          )}
          <div className="shrink-0 px-5 pb-5">
            <Composer
              value={draft}
              onChange={setDraft}
              onSend={() => void send()}
              state={sending ? "replying" : undefined}
              placeholder="说说你的想法"
            />
          </div>
        </div>

        {hasMap && (
          <aside className="relative flex min-h-0 flex-col border-t border-mk-border lg:border-l lg:border-t-0">
            {/* Floats over the canvas rather than sitting above it: a header
                band would cut the drawing surface in two, and the point of
                this panel is that it reads as one continuous sheet. */}
            <div className="pointer-events-none absolute inset-x-0 top-0 z-10 flex items-center justify-between px-4 py-2">
              <span
                className="rounded-mk-full px-2 py-0.5 text-mk-label text-mk-faint"
                style={{ background: "color-mix(in srgb, var(--mk-paper) 88%, transparent)" }}
              >
                你的思路
              </span>
              <span
                className="rounded-mk-full px-2 py-0.5 text-mk-small text-mk-faint"
                style={{ background: "color-mix(in srgb, var(--mk-paper) 88%, transparent)" }}
              >
                {/* 🚨 原来只说了改和删，没说**怎么加** —— 而这张图长什么样
                    全靠她说。2026-09-13 第十四轮走查她的原话：「它问我有几条
                    理由，但我不知道怎么把理由加到'你的思路'那个列表里去，
                    那里只有一个删除按钮。」她在找一颗「＋」，而这里没有、
                    也不该有：加一条的办法是跟印记说一句，它摆上去。
                    那条路一直在，只是没有一个字讲过。 */}
                {/* 🚨 「找来的材料也说给印记听」是 2026-09-16 加的那半句。
                    产品负责人：材料不该只有她自己的经历，她找回来的研究、报道、
                    数据同样算，而且印记要查它 —— 但她得先知道这条路存在。
                    说清楚「连出处一起说」，因为没有出处的那一份印记查不了。 */}
                想加一条，说给印记听 —— 找来的研究、报道、数据也一样，连出处一起说；点一条可以改，也能删；拖一条到另一条上面，它就挂到那一条下面
              </span>
            </div>
            <MindMap items={outline} justAdded={justAdded} onRemove={removeNode} onEdit={editNode} onMove={moveNode} />
          </aside>
        )}
      </div>
    </div>
  );
}
