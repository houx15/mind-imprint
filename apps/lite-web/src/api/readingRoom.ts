import type { Anchor, SelectionEval, TakeawayDraft } from "@mind-imprint/contracts";
import { CARD_REGISTRY, SelectionEval as SelectionEvalSchema } from "@mind-imprint/contracts";
import type { StudioTurnEvent } from "@/api/studioTurn";
import type { LiteReadingRoomApi } from "../readings/ReadingRoom";
import type { CoachCardAnswer, CoachCardSpec } from "../readings/CoachCard";
import type { ReadingOutcome } from "@/studio/reading/readingLoop";
import { apiFetch } from "./client";
import { apiErrorText } from "./errorText";

/**
 * api/readingRoom.ts — the injected `api` object lite's OWN `ReadingRoom`
 * (`../readings/ReadingRoom`) runs on, backed entirely by the lite
 * `/api/v1/readings/{id}/…` endpoints.
 *
 * It satisfies `LiteReadingRoomApi`, not pro's `ReadingRoomApi`: the fork
 * (2026-08-29) existed to stop lite depending on a file pro renders, and a
 * type import is that dependency just as much as a JSX one. The two shapes
 * differ by exactly one method — pro's `putReadingBrief`, which fed the
 * 「你读这篇是为了」 bar lite's room does not carry.
 *
 * Two shape mismatches to be deliberate about, because they are the whole job
 * of this file:
 *
 * 1. **`{id}` is ALWAYS the atom id.** Every method keeps pro's parameter
 *    positions (`projectId`, `materialId`, `rid`) so the room needs no
 *    special-casing (`useReadingLoop` is still pro's), and every one of them
 *    is IGNORED — the reading id is closed over at construction. A lite
 *    reading has no project and no reference row; the atom IS the addressing
 *    unit.
 *
 * 2. **Pro streams SSE, lite answers one JSON body.** `readTurn` /
 *    `summonCard` / `submitProjectCard` are declared as
 *    `AsyncGenerator<StudioTurnEvent>`, so this file does the adapting: one
 *    POST, then the equivalent event sequence yielded synchronously. The room
 *    never learns the difference.
 *
 * AI-failure honesty (standing user rule): a failed turn is NEVER turned into
 * a coach sentence. `useReadingLoop` drops `error` events on the floor, so a
 * bare `yield {type:"error"}` would be silent — the host passes `onAiError`
 * and renders a visible banner instead.
 */

// --- wire shapes (straight off the Go handlers) -----------------------------

/** `cardDTO` — apps/api/internal/api/reading_cards.go. */
export type LiteCard = {
  id: string;
  cardId: string;
  blockId: string | null;
  status: "proposed" | "active" | "submitted" | "skipped";
  /** Who put this lens on the article: `"router"` (the AI proposed it in a
   *  coach turn) or `"student"` (she picked it out of the 透镜库). */
  origin: "router" | "student";
  anchors: Anchor[];
  fieldValues: Record<string, unknown>;
  eventTrace: unknown[];
  framework: Partial<SelectionEval>;
  createdAt: string;
  submittedAt: string | null;
};

/** `liteTurnDTO` — the SAME body the coach turn and the lens summon answer with. */
export type LiteTurn = {
  reply: string;
  decision: string;
  card: LiteCard | null;
  nudge: string;
  hintCardId: string | null;
};

export type LiteAnnotation = {
  id: string;
  blockId: string;
  span: { start?: number; end?: number } | null;
  quote: string;
  note: string;
  createdAt: string;
};

/**
 * What a chat message carries BESIDES its words (`atom_message.payload`,
 * migration 0106) — the server's `coachMessagePayload`, verbatim.
 *
 * An envelope rather than a bare card, and for the reason the server gives:
 * a reader holding only the JSON has to be able to tell 「这条消息带了一张卡」
 * apart from 「这条消息是她对一张卡的作答」. Both halves matter here — without
 * the second one, a refresh redraws a card still waiting for a tap she has
 * already made.
 */
export type CoachMessagePayload = {
  card?: CoachCardSpec | null;
  answer?: CoachCardAnswer | null;
  /** 这条回复没说完就交给她了。服务端两次都拿到半句话时标上。 */
  incomplete?: boolean | null;
};

export type LiteMessage = {
  seq: number;
  role: string;
  content: string;
  createdAt: string;
  payload?: CoachMessagePayload | null;
};

/** 这条回复是不是半句话。半句话本身不是错（模型真的只发出来这么多），
 *  错的是没有任何东西告诉她 —— 产品负责人 2026-09-12 报的那一幕：屏幕上是
 *  「对，调用数据是一个方向。**但」，然后就没有了，她只能自己打一个「?」去问。 */
export function replyIsIncomplete(m: LiteMessage): boolean {
  return m.payload?.incomplete === true;
}

/** The card on this message, if it carried one. Shape-checked rather than
 *  trusted: `payload` is jsonb the client never wrote, and a half-built card
 *  rendered as a real one is a dead end she cannot get out of. */
export function coachCardOf(m: LiteMessage): CoachCardSpec | null {
  const c = m.payload?.card;
  if (!c || typeof c.type !== "string" || typeof c.prompt !== "string" || !c.prompt) return null;
  // 🚨 这是一张**客户端的白名单**，加卡片形状的时候极容易忘掉它。
  //
  // 2026-09-10：服务端加了 label_roles / word_bank 两块板，校验器、prompt、
  // 渲染组件全都写好了，服务端也真的把板发出来了（`atom_message.payload` 里
  // 逐字躺着一张完整的 label_roles）—— 而这一行把它们当成不认识的类型扔了。
  // 结果是 印记 一遍遍说「现在给你一张卡片」，她屏幕上什么都没有。
  // 我为此在服务端追了四个「静默丢弃点」，而真正丢掉它的是这里。
  //
  // 一条测试守着这条线：`test/coachCardOf.test.ts` 拿五种类型逐个过。
  if (!COACH_CARD_TYPES.includes(c.type as CoachCardSpec["type"])) return null;
  const options = Array.isArray(c.options)
    ? c.options.filter((o) => o && typeof o.quote === "string" && o.quote !== "")
    : undefined;
  const words = Array.isArray(c.words)
    ? c.words.filter((w) => w && typeof w.term === "string" && w.term !== "")
    : undefined;
  // 🚨 A `choose_span` with nothing left to choose is not a card, it is a dead
  // end: the room would render a question with no way to answer it and no way
  // out. This function's whole contract is「半张卡片不许当成真卡片渲染」——
  // filtering the options empty and returning anyway breaks it from inside.
  //
  // 两块板同理，只是「空了」的判据不同：标注板没有句子、生词板没有词，
  // 渲染出来都是一块摆不了的板。
  if (
    (c.type === "choose_span" || c.type === "label_roles" || c.type === "order_events") &&
    !(options && options.length > 0)
  )
    return null;
  if (c.type === "word_bank" && !(words && words.length > 0)) return null;
  return {
    type: c.type as CoachCardSpec["type"],
    prompt: c.prompt,
    ...(options && options.length > 0 ? { options } : {}),
    ...(words && words.length > 0 ? { words } : {}),
    // 格子是服务端填的闭表，原样带过来。
    ...(Array.isArray(c.labels) && c.labels.length > 0 ? { labels: c.labels } : {}),
  };
}

/** 六种卡片形状。🚨 服务端加一种，这里必须跟着加一种，否则那种卡片会在客户端
 *  被静默丢掉 —— 它已经发生过一次了（见 coachCardOf 里那段）。 */
const COACH_CARD_TYPES: CoachCardSpec["type"][] = [
  "choose_span",
  "pick_in_article",
  "short_text",
  "label_roles",
  "word_bank",
  // 2026-09-17：排序板，只在报道和记叙上出现（reading_genre.go）。
  "order_events",
];

/** Her answer to a card, if this message IS one. */
export function coachAnswerOf(m: LiteMessage): CoachCardAnswer | null {
  const a = m.payload?.answer;
  if (!a || typeof a.choice !== "string" || a.choice === "") return null;
  return a;
}

export type ReadingRoomApiOptions = {
  /** Called when a coach turn or a lens summon fails outright. The room's own
   *  loop has nowhere to show an `error` event, so the host surfaces it. */
  onAiError?: (message: string) => void;
};

const base = (id: string) => `/api/v1/readings/${encodeURIComponent(id)}`;

// --- the api object ---------------------------------------------------------

export function createReadingRoomApi(readingId: string, opts: ReadingRoomApiOptions = {}): LiteReadingRoomApi {
  const root = base(readingId);

  /** One JSON turn → the SSE event sequence the room's loop expects.
   *  Reply first, then the card: on a summon she should read the coach's
   *  sentence and only then see "一副透镜被放进了原文". */
  async function* turnEvents(turn: LiteTurn): AsyncGenerator<StudioTurnEvent> {
    const reply = turn.reply.trim();
    if (reply) {
      yield {
        type: "intervention",
        interventionId: "",
        body: reply,
        anchor: "",
        // criterion carries the card id on a hint — that is what turns a 克制
        // sentence into a one-tap 「要不要用它看看」 offer instead of a remark
        // she cannot act on. Task 7 added hintCardId for exactly this.
        criterion: turn.hintCardId ?? "",
        level: turn.decision === "hint" ? "hint" : "reply",
      };
    }
    if (turn.card) {
      yield {
        type: "card",
        cardInstanceId: turn.card.id,
        cardId: turn.card.cardId,
        nudgeText: turn.nudge,
        anchors: turn.card.anchors ?? [],
        materialId: readingId,
      };
    }
    yield { type: "done" };
  }

  /** Runs one turn-shaped POST, adapting a failure into an `error` event AND a
   *  visible banner. The loop's `busy` flag is released either way — a failed
   *  turn must never leave the composer locked. */
  async function* postTurnLike(path: string, body: unknown): AsyncGenerator<StudioTurnEvent> {
    let turn: LiteTurn;
    try {
      turn = await apiFetch<LiteTurn>(path, { method: "POST", body: JSON.stringify(body) });
    } catch (err) {
      const message = apiErrorText(err);
      opts.onAiError?.(message);
      yield { type: "error", code: "ai_dialogue_failed", message };
      yield { type: "done" };
      return;
    }
    yield* turnEvents(turn);
  }

  return {
    // --- the loop's own slice -------------------------------------------
    readTurn(_projectId, _materialId, body) {
      return postTurnLike(`${root}/turn`, {
        text: body.student_text,
        focusedSpans: body.focused_spans.map((s) => ({ blockId: s.block_id, quote: s.quote })),
      });
    },

    summonCard(_projectId, _materialId, cardId) {
      return postTurnLike(`${root}/summon`, { cardId });
    },

    async activateProjectCard(_projectId, cid) {
      await apiFetch<LiteCard>(`${root}/cards/${encodeURIComponent(cid)}/activate`, { method: "POST" });
    },

    async evaluateCardSelection(_projectId, cid, body) {
      const raw = await apiFetch<unknown>(`${root}/cards/${encodeURIComponent(cid)}/evaluate`, {
        method: "POST",
        body: JSON.stringify(body),
      });
      // Parsed, not cast — the room renders every field of this straight into
      // the hanging card, so a shape drift must fail loudly here.
      return SelectionEvalSchema.parse(raw);
    },

    async *submitProjectCard(_projectId, cid, input) {
      // anchors carries WHICH SENTENCE she picked — 过程即数据. The lite submit
      // endpoint stores it on the card row.
      const card = await apiFetch<LiteCard>(`${root}/cards/${encodeURIComponent(cid)}/submit`, {
        method: "POST",
        body: JSON.stringify({
          fieldValues: input.field_values,
          eventTrace: input.event_trace,
          anchors: input.anchors,
        }),
      });
      // The loop retires the card ONLY on `cardStatus === "completed"`, so the
      // lite vocabulary ('submitted') has to be translated here rather than
      // left to leak.
      yield { type: "done", cardStatus: card.status === "submitted" ? "completed" : card.status };
    },

    async skipProjectCard(_projectId, cid) {
      // The lite skip endpoint takes no body: the row flipping to 'skipped' IS
      // the record (铁律④), and the room only ever sends an empty event_trace.
      await apiFetch<LiteCard>(`${root}/cards/${encodeURIComponent(cid)}/skip`, { method: "POST" });
    },

    async getOpenCard(_projectId, _materialId) {
      // Deadlock prevention: after a reload, a card left proposed/active would
      // otherwise be invisible to the room while still blocking every future
      // summon server-side (readingPacing's OpenCard). Derived from the card
      // list — lite needs no separate open-card endpoint.
      const { cards } = await apiFetch<{ cards: LiteCard[] }>(`${root}/cards`);
      const open = cards.find((c) => c.status === "proposed" || c.status === "active");
      if (!open) return null;
      return {
        cardInstanceId: open.id,
        cardId: open.cardId,
        status: open.status as "proposed" | "active",
        anchors: open.anchors ?? [],
      };
    },

    // --- 完成这篇 --------------------------------------------------------
    //
    // ONE call now. It used to be two — PUT the 收获 she had just typed into
    // a finalize form, THEN POST finish, in that order, because finish
    // refused an empty takeaway. Both the form and the gate are gone:
    //
    //   > we have give abundant steps for the reading. so we don't need to
    //   > ask student to enter the form again. we should jump to the reading
    //   > report page.
    //
    // `getTakeawayDraft` went with them — it existed only to seed that form's
    // fields. Nothing assembles a draft any more; the report is generated
    // server-side from the whole record.
    async finishReading() {
      return apiFetch<unknown>(`${root}/finish`, { method: "POST" });
    },
  };
}

/** 她给这次阅读体验打的星（1–5）。她评我们，不是我们评她。 */
export async function putReadingRating(id: string, rating: number): Promise<number | null> {
  const r = await apiFetch<{ rating: number | null }>(`${base(id)}/rating`, {
    method: "PUT",
    body: JSON.stringify({ rating }),
  });
  return r.rating ?? null;
}

// --- reads the host needs before it can mount the room ---------------------

export async function listReadingAnnotations(id: string): Promise<LiteAnnotation[]> {
  const raw = await apiFetch<{ annotations: LiteAnnotation[] }>(`${base(id)}/annotations`);
  return raw.annotations ?? [];
}

export async function listReadingMessages(id: string): Promise<LiteMessage[]> {
  const raw = await apiFetch<{ messages: LiteMessage[] }>(`${base(id)}/messages`);
  return raw.messages ?? [];
}

export async function listReadingCards(id: string): Promise<LiteCard[]> {
  const raw = await apiFetch<{ cards: LiteCard[] }>(`${base(id)}/cards`);
  return raw.cards ?? [];
}

/**
 * The confirmed findings, rebuilt from the card rows so a reload does not wipe
 * 阅读成果 (and its article highlights) back to zero. Everything here was
 * already persisted: the sentence she picked lives in `anchors`, the review
 * that produced the finding lives in `framework_fill`. A card with either
 * missing is skipped rather than shown as a half-outcome.
 */
/**
 * The sentence SHE picked, or nothing.
 *
 * A card carries up to two anchors: the AI's example (author `"ai"`, written
 * at summon time to show her what the lens is for) and her own pick (author
 * `"student"`, written at submit). There used to be a `?? anchors[0]` fallback
 * here, which on a card missing the student anchor quietly promoted the AI's
 * example sentence into her finding — rendered as 阅读成果 and carried into
 * the takeaway draft's key quotes.
 *
 * That is a 铁律① leak: the AI never writes her prose, and being QUOTED BACK
 * to her as her own work is the same violation wearing a different hat.
 * Mis-attribution is worse than omission, so a card with no student anchor is
 * simply skipped. (The pro side's Go assembly, readingOutcomesFromCards, has
 * always filtered on `Author == "student"` for the same reason.)
 */
function studentAnchorOf(c: LiteCard): Anchor | undefined {
  return c.anchors?.find((a) => a.author === "student");
}

// ---------------------------------------------------------------------------
// 任务清单 + 段落工具 (0101)
//
// The step BELOW the lens. A lite student has to be able to read a paragraph
// before "read it through a lens" means anything, so the room now offers
// per-paragraph explainers and a task list that says what to do in what order.
// ---------------------------------------------------------------------------

export type ReadingTask = {
  id: string;
  position: number;
  /** 'read' | 'focus_block' | 'lens' | 'reflect' | 'connect' | 'hunt' — what
   *  the step renders as. Adding a kind is a code change; adding a ROUTINE is
   *  not. */
  kind: string;
  label: string;
  detail: string;
  /** Only 'focus_block' carries one: the paragraph 印记 picked out. */
  blockId: string;
  status: "pending" | "done" | "skipped";
  completedAt: string | null;
};

export type ReadingPlan = { routineKey: string; routineName: string; tasks: ReadingTask[] };

/** The plan as it stands. An empty task list is the honest "not planned yet"
 *  answer, not an error. */
export async function getReadingPlan(id: string): Promise<ReadingPlan> {
  const raw = await apiFetch<ReadingPlan>(`/api/v1/readings/${encodeURIComponent(id)}/plan`);
  return { routineKey: raw.routineKey ?? "", routineName: raw.routineName ?? "", tasks: raw.tasks ?? [] };
}

/**
 * Ask 印记 to lay out a reading plan for this article. One model call.
 *
 * REPLACES any existing plan — which is what makes 重新排一份 possible. The old
 * statuses go with it, and that is right: a new plan is a new set of steps,
 * not the old ones renumbered.
 */
export async function generateReadingPlan(id: string): Promise<ReadingPlan> {
  const raw = await apiFetch<ReadingPlan>(`/api/v1/readings/${encodeURIComponent(id)}/plan`, { method: "POST" });
  return { routineKey: raw.routineKey ?? "", routineName: raw.routineName ?? "", tasks: raw.tasks ?? [] };
}

/**
 * One guided turn (带读). Empty `text` means she pressed 开始 — the coach
 * plans on demand and leads her into step one.
 *
 * She never sets a step's status herself: `advance` is the coach's judgement,
 * applied server-side. That is the whole point of this endpoint existing
 * separately from the checklist below.
 *
 * `picks` — the sentence(s) she POINTED AT in the article for this turn
 * (distinct from typing "I think it's this one"), keyed by block id. Sent
 * ALONGSIDE `text` (which still carries the same quotes inlined as markdown
 * blockquotes) rather than instead of it, so a server build that hasn't
 * learned about `picks` yet still sees exactly what it always has.
 */
/**
 * `readingLensDone` — apps/api/internal/api/reading_coach.go. What she just
 * finished doing with a lens, sent so 印记 can react to it and advance the
 * step.
 *
 * 🚨 Why the room has to send this at all: the lens loop is shared with pro
 * (`apps/web/src/studio/reading/readingLoop.ts`), and its `confirm()`
 * announces the finished outcome by appending a line to `loop.messages` —
 * an array LITE NEVER RENDERS (lite renders `ReadingCoachPanel`, a different
 * thread over the same `atom_message` table). So finishing a lens used to
 * produce exactly nothing on screen: no reply, no advance. Reported as
 * 「透镜应用完毕之后，没有响应，没有推进到下一步」.
 *
 * `finding` is 印记's own earlier evaluation (`agent.EvaluateSelection`),
 * not her words — the server labels the two separately in the prompt and
 * must keep doing so.
 */
export type ReadingLensDone = {
  /** The lens's display name, for 印记 to say back to her — never its id. */
  cardName: string;
  /** The sentence SHE picked out of the article. */
  quote: string;
  /** What the room already concluded from that pick. */
  finding: string;
  /**
   * 复核对这一句的结论，以及它给出的理由 —— **她屏幕上已经读过这两样**。
   *
   * 🚨 少了它们，印记 这一轮会跟几秒钟前屏幕上那个结论对着干（产品负责人
   * 2026-09-17：「句子匹配不通过，但是点击记录发现后，主 ai 又给出了不一样的
   * 回答」）。服务端拿它们只是转述，不会再问一次模型。
   * 可选：老的成果行里没有 eval。
   */
  verdict?: "strong" | "partial" | "rethink";
  verdictReason?: string;
};

export async function postReadingCoachTurn(
  id: string,
  text: string,
  picks: { blockId: string; quote: string }[] = [],
  /** Set when this turn IS a tap on the card 印记 wrote into its last reply.
   *  Sent through UNCHANGED — the server checks `choice` against the article
   *  as a literal substring to tell 「文章原文」 from 「她自己的话」, so trimming
   *  a comma here silently deletes the fact that she pointed at anything. */
  cardAnswer: CoachCardAnswer | null = null,
  /** Set when the ROOM is reporting a finished lens rather than something she
   *  said — see `ReadingLensDone`. Never both this and `cardAnswer`: a lens
   *  and a tappable card are two different instruments. */
  lensDone: ReadingLensDone | null = null,
): Promise<{
  reply: string;
  tasks: ReadingTask[];
  currentTaskId: string;
  focusBlock: string;
  /** The paragraph tool the coach reached for this turn, if any. */
  tool: string;
  finished: boolean;
  /** The lens 印记 aimed at a paragraph this turn, if any — same shape the
   *  student-summon endpoint returns. */
  card: LiteCard | null;
  /** Why this lens, in the coach's own words — only meaningful alongside `card`. */
  nudge: string;
  /** 🚨 NOT `card`. That one is the LENS (a row in atom_card, opened and
   *  closed); this one is the tappable card 印记 wrote into this very reply
   *  and which lives only on the message. The server keeps them under two
   *  keys for exactly this reason — one name would let them overwrite each
   *  other on any turn that produced both. */
  coachCard: CoachCardSpec | null;
  /** The model's thinking for this turn, to be shown FOLDED next to the reply.
   *  Empty whenever the capability class this call routes to has thinking off —
   *  the panel then renders no fold at all, because an empty fold reads as
   *  「它没有想」 when the truth is 「没有可读的东西」.
   *
   *  Never stored: it is absent from the transcript, so a refresh loses it.
   *  The model's scratch work is not her record; the process tree is. */
  thinking: string;
}> {
  const raw = await apiFetch<{
    reply: string;
    tasks: ReadingTask[];
    currentTaskId: string;
    focusBlock: string;
    tool: string;
    finished: boolean;
    card?: LiteCard | null;
    nudge?: string;
    coachCard?: CoachCardSpec | null;
    thinking?: string;
  }>(`/api/v1/readings/${encodeURIComponent(id)}/coach`, {
    method: "POST",
    body: JSON.stringify({ text, picks, cardAnswer, lensDone }),
  });
  return {
    reply: raw.reply,
    tasks: raw.tasks ?? [],
    currentTaskId: raw.currentTaskId ?? "",
    focusBlock: raw.focusBlock ?? "",
    tool: raw.tool ?? "",
    finished: Boolean(raw.finished),
    card: raw.card ?? null,
    nudge: raw.nudge ?? "",
    // Rebuilt through the same shape check the stored payload goes through,
    // so a live card and a reloaded one can never disagree about what counts.
    coachCard: coachCardOf({ seq: 0, role: "ai", content: "", createdAt: "", payload: { card: raw.coachCard } }),
    thinking: raw.thinking ?? "",
  };
}

/**
 * 铁律②: 'skipped' is a real outcome, recorded rather than prevented.
 *
 * NOT used by the guided flow — there the coach decides. This stays for the
 * explicit plan surface, where 重排 and manual correction still make sense.
 */
export async function setReadingTaskStatus(
  id: string,
  taskId: string,
  status: "pending" | "done" | "skipped",
): Promise<ReadingTask> {
  return apiFetch<ReadingTask>(
    `/api/v1/readings/${encodeURIComponent(id)}/plan/tasks/${encodeURIComponent(taskId)}`,
    { method: "POST", body: JSON.stringify({ status }) },
  );
}

export type ReadingBlockTool = {
  id: string;
  label: string;
  /**
   * 这件工具讲的是哪一级：`""` = 整段，`"sentence"` = 她点的那一句。
   *
   * 语法 2026-09-16 改成按句子讲，所以点它之后先请她在这一段里点一句，再发
   * 请求。服务端发这一位而不是让前端写死，理由和这个端点本来就存在的理由是
   * 同一条：写死会和服务端漂开。
   */
  subject?: string;
};

/** Which tools exist depends on the ARTICLE's language. Fetched rather than
 *  hardcoded so the buttons she sees and the ids the server accepts cannot
 *  drift apart. */
export async function listReadingBlockTools(id: string): Promise<{ lang: string; tools: ReadingBlockTool[] }> {
  const raw = await apiFetch<{ lang: string; tools: ReadingBlockTool[] }>(
    `/api/v1/readings/${encodeURIComponent(id)}/blocks/tools`,
  );
  return { lang: raw.lang ?? "zh", tools: raw.tools ?? [] };
}

/**
 * 一张词卡。关键单词那件工具的产物（2026-09-16）。
 *
 * 🚨 `term` 是**这一段里的原样**，服务端已经拿它回段落里逐字核对过 ——
 * 核不上的卡片根本不会发过来。这正是荧光笔敢直接拿它去正文里找的依据。
 */
export type ReadingWord = {
  term: string;
  /** 词性，中文。 */
  pos: string;
  /** 它在这一句里的意思，不是词典里的第一条。 */
  meaning: string;
  /** 为什么这个词值得学：词根、近义词的差别、常见搭配。 */
  note: string;
  /** 一个新造的例句，不是原文那一句。 */
  example: string;
  exampleZh: string;
};

export type ReadingBlockNote = {
  blockId: string;
  tool: string;
  body: string;
  /** 讲的是哪一句。空 = 整段。只有语法那件工具非空。 */
  subject?: string;
  /** 词卡。只有关键单词那件工具有。 */
  words?: ReadingWord[];
  /** 语法卡（2026-09-17 起）。老的语法笔记是一段散文，没有这一项。 */
  grammar?: ReadingGrammar;
};

/** 语法卡里句子被标出来的一段。`text` 逐字来自那一句（服务端核对过）。
 *  `label` 是它是什么（从句种类 / 成分 / 词性词形 / 时态）；第一版的卡片把
 *  成分名写在 `role` 里，读的时候用 `label ?? role`。 */
export type ReadingGrammarSpan = {
  text: string;
  label?: string;
  role?: string;
  note?: string;
  example?: string;
  exampleZh?: string;
};
export type ReadingGrammarPart = ReadingGrammarSpan;
export type ReadingGrammarPoint = { name: string; why?: string; example?: string; exampleZh?: string };
/** 第二版（2026-09-17 晚些）分三层：句法（clauses 从句 + parts 句子成分）·
 *  词法（words）· 时态（tenses）。主句不单独给：句子里不属于任何从句的部分就是主句。
 *  第一版只有 backbone / parts / points。 */
export type ReadingGrammar = {
  clauses?: ReadingGrammarSpan[];
  /** 第二版里可以是空的（这一句的难点不在结构）。 */
  parts?: ReadingGrammarSpan[];
  words?: ReadingGrammarSpan[];
  tenses?: ReadingGrammarSpan[];
  meaning?: string;
  backbone?: string;
  points?: ReadingGrammarPoint[];
};

/** 这张语法卡有没有可摆的东西（四层、老卡的 parts 任何一层不空）。 */
export function grammarHasContent(g: ReadingGrammar | undefined): boolean {
  if (!g) return false;
  return [g.clauses, g.parts, g.words, g.tenses].some((l) => (l?.length ?? 0) > 0);
}

/** Everything she has already opened, so a reload restores it instead of
 *  making her pay for it twice. */
export async function listReadingBlockNotes(id: string): Promise<ReadingBlockNote[]> {
  const raw = await apiFetch<{ notes: ReadingBlockNote[] }>(
    `/api/v1/readings/${encodeURIComponent(id)}/blocks/notes`,
  );
  return raw.notes ?? [];
}

/** Explain ONE paragraph with ONE tool. Cached server-side by
 *  (blockId, tool, sentence), so a second call is instant and free.
 *
 *  `sentence` 只有 `subject === "sentence"` 的工具要传（语法）。服务端会拿它
 *  回这一段里逐字核对，对不上就是 400。 */
export async function explainReadingBlock(
  id: string,
  blockId: string,
  tool: string,
  sentence?: string,
): Promise<ReadingBlockNote & { cached: boolean }> {
  return apiFetch<ReadingBlockNote & { cached: boolean }>(
    `/api/v1/readings/${encodeURIComponent(id)}/blocks/${encodeURIComponent(blockId)}/explain`,
    { method: "POST", body: JSON.stringify({ tool, sentence: sentence ?? "" }) },
  );
}

// ---------------------------------------------------------------------------
// 阅读问题 (Task 10)
//
// What a finished reading leaves her with besides a full stop. The product
// ruling this exists for: *"just a suggestion, but suggestion is very
// important, some interesting questions would grow from this reading."*
// Generated on first call, cheap thereafter (server-side cache) — this file
// just reads the answer, same shape as every other list-fetch here.
// ---------------------------------------------------------------------------

export type ReadingQuestion = { id: string; text: string; anchorQuote: string; anchorBlock: string };

/** An empty list is a designed outcome — the article was too thin to grow at
 *  least two anchored questions — not an error. */
export async function getReadingQuestions(id: string): Promise<ReadingQuestion[]> {
  const raw = await apiFetch<{ questions: ReadingQuestion[] }>(
    `/api/v1/readings/${encodeURIComponent(id)}/questions`,
  );
  return raw.questions ?? [];
}

export function toReadingOutcomes(cards: LiteCard[]): ReadingOutcome[] {
  const out: ReadingOutcome[] = [];
  for (const c of cards) {
    if (c.status !== "submitted") continue;
    const anchor = studentAnchorOf(c);
    if (!anchor || anchor.end <= anchor.start) continue;
    const parsed = SelectionEvalSchema.safeParse(c.framework);
    if (!parsed.success) continue;
    out.push({
      id: `outcome-${c.id}`,
      cardId: c.cardId,
      cardName: CARD_REGISTRY[c.cardId]?.name ?? c.cardId,
      blockId: anchor.block_id,
      start: anchor.start,
      end: anchor.end,
      quote: anchor.quote,
      finding: parsed.data.finding,
      judgment: parsed.data.judgment,
      support: parsed.data.support,
      caveat: parsed.data.caveat,
      eval: parsed.data,
    });
  }
  return out;
}
