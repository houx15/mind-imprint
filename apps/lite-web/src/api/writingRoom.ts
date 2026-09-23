import type { LiteMessage, LiteTurn } from "./readingRoom";
import { apiFetch } from "./client";
import type { Writing } from "./writings";

/**
 * api/writingRoom.ts — the room-specific 写作 calls, split out of
 * api/writings.ts the same way api/readingRoom.ts is split from
 * api/readings.ts: writings.ts owns the CRUD shell (list/create/get/rename),
 * this file owns everything that happens once she is INSIDE one — stage/
 * target words, setup + opening, structure, the coach turn, outline,
 * snippets, and compose/draft/review/finish.
 *
 * `LiteMessage`/`LiteTurn` are reused from readingRoom.ts, not redeclared:
 * `liteMessageDTO`/`liteTurnDTO` (apps/api/internal/api/reading_turn.go) are
 * the SAME Go types the writing handlers answer with. Everything else here is
 * writing-only, mirroring the Go side's own naming split
 * (writing_outline.go's file comment: "two unrelated domains share a word,
 * not a table or a handler").
 */

/**
 * A persisted block. `text` is HER sentence for it; `role` is the generic
 * label the skeleton contributed ("反方最强的说法"). They are separate fields
 * for the same reason they are separate columns (migration 0100): once merged
 * there is no telling her thinking from the template, and that distinction is
 * the whole point.
 */
export type WritingOutlineItem = {
  id: string;
  text: string;
  /**
   * 屏幕上的小标题。**派生值**：服务端按 `kind` 算出来（writingKindLabel），
   * 不再是模型写的散文。老行（0182 之前）可能还带着模型当初写的那一句。
   */
  role: string;
  /**
   * 这一块是什么 —— 闭表，见 `writings/outlineKind.ts`（服务端
   * `writing_kind.go` 是单一真相源）。深度和父节点都由它算出来。
   *
   * 0182 之前的行没有这个字段；`outlineKindOf` 会按 role + depth 现算一个，
   * 所以读的时候走那个函数，别直接 switch 这个字符串。
   */
  kind?: string;
  /**
   * 她在行文那一步给这一块标的论证方法（vocab 的 id，服务端 0184）。
   * 空 = 还没标。段落那一步的引导据此说「这一段你打算用举例论证」。
   */
  method?: string;
  depth: number;
  position: number;
  /**
   * 这条材料从哪来（服务端迁移 0158）。
   *
   * 非空 = 她**找回来的**一份材料（一份研究、一条报道、一组数据、一次访谈）。
   * 空 = 她自己的经历，或者她没写出处 —— 两者不区分：她见过的事，出处就是
   * 她自己，编一个「本人」进去是多余的。
   *
   * 印记 拿它去查这份材料（出处是谁 / 它真的说了那句话吗 / 它撑的是不是这条
   * 分论点 / 相关有没有被写成因果），所以它必须跟着节点一起回传 —— 和 `role`
   * 同一个道理：全量替换那条路只负责别把它弄丢。
   */
  source?: string;
  /** The stored guide for this block (writing_outline.go's writingOutlineItemDTO.Guide),
   *  painted the moment GET /outline loads instead of waiting for her to open
   *  the block — absent when this block has never been guided yet. */
  guide?: WritingBlockGuide | null;
};

/*
 * `WritingStructure` and the structure-library calls (listWritingStructures /
 * recommendWritingStructure / applyWritingStructure) were DELETED on
 * 2026-08-27, along with their endpoints. Picking a skeleton off a shelf —
 * with block names like 「你承认它哪一部分是对的」 — was rigid and
 * unreadable for a middle-school student, and filling a template is not
 * thinking. 结构 is now a planning conversation (postWritingPlanTurn) whose
 * output grows into the mind map. Do not reintroduce a picker.
 */

/**
 * One worked example for a method, pre-authored about a topic no student is
 * writing on (apps/api/internal/vocab.Example). `topic` MUST be shown beside
 * `text` wherever this is rendered — that is what keeps a borrowed example
 * from ever reading as a suggestion about her own piece (铁律①, VocabExamples).
 */
export type WritingGuideExample = { topic: string; text: string };

/** A sentence frame for a method (apps/api/internal/vocab.Pattern) — a shape
 *  to fill in, not filled-in words, so it never doubles as a sentence for
 *  her essay. `gloss` is the Chinese reading of the frame; `example` is one
 *  worked sentence, about a topic she is not writing on. */
export type WritingGuidePattern = {
  label: string;
  frame: string;
  gloss: string;
  example: string;
};

/**
 * A method resolved for display — writing_guide.go's writingGuideMethodDTO.
 * The bare vocab id never reaches the client (a bare id means nothing to a
 * student who has never seen vocab's registry); this is the full record.
 *
 * `formalName` is the 语文课 curriculum term (2026-08-28 ruling: offered, never
 * imposed — `name` is plain language and stays what she reads by default).
 * It is `""` when the library has no distinct formal term for this method
 * (最后提个建议), and equal to `name` for the two methods she already knows by
 * their real name (留悬念、开门见山) — in both cases there is nothing a card
 * would add, which is exactly the condition GuideBox uses to decide whether
 * the name is tappable at all.
 */
export type WritingGuideMethod = {
  name: string;
  formalName: string;
  definition: string;
  examples: WritingGuideExample[];
  patterns: WritingGuidePattern[];
};

/**
 * What the coach hands back when she asks for help on ONE block —
 * writing_guide.go's writingGuideDTO, the wire AND stored shape (also what
 * `GET /outline`'s per-block `guide` decodes into).
 *
 * `job` is about the BLOCK's task, never her content — the same job holds for
 * any student writing this kind of block, so there is nothing here for her to
 * paste into her essay. `methods` are resolved from vocab's registry, never
 * invented by the model. `questions` is the one field that could carry a
 * sentence, so it is the one the server hard-filters (？ / ? ending only) —
 * see writing_guide.go's parseWritingGuide.
 */
export type WritingBlockGuide = {
  job: string;
  methods: WritingGuideMethod[];
  questions: string[];
  /**
   * 上一组问题（只有一层）。
   *
   * 🚨 同事 2026-09-20：「每一次刷新就会变成新的东西」。「换一组问题」原来
   * 直接覆盖，她读过的那一组当场没了。现在旧的那一组留在这里，引导框给一个
   * 「看上一组」—— 她按那颗按钮是想再要一个角度，不是想把刚才那几个问题扔掉。
   */
  previous?: WritingBlockGuide;
};

export type WritingSnippet = {
  id: string;
  outlineId: string | null;
  outlineHeading: string;
  position: number;
  text: string;
  updatedAt: string;
};
export type WritingDraft = { body: string; updatedAt: string | null };

/**
 * One concrete point in a comment — apps/api/internal/api/writing_comment.go's
 * CommentPoint. `quote` is a sentence copied VERBATIM from the text being
 * commented on (the server's validateCommentPoints drops any point whose
 * quote does not appear literally in the source, so every point that reaches
 * the client is guaranteed traceable). `text` is why that sentence matters —
 * never a rewrite of it.
 *
 * 2026-09-11：一条意见从「一段话」变成「一件能做的事」。
 * `kind` 分「已经用对的」和「要改的」；`action` 是一句祈使，说清她接下来要做
 * 什么——服务端会把 action 为空的 issue 整条丢掉（writing_comment.go）。
 * `layer` 是第几层（1 立意 / 2 材料 / 3 结构 / 4 字句），同一条回复里的 issue
 * 一定同属一层：优先级在服务端就筛过了，这里不重筛。
 * 老的评论行只有 text+quote，所以除了这两个字段以外都要当可能不存在来读。
 */
export type CommentPoint = {
  kind?: "good" | "issue";
  symptom?: string;
  layer?: number;
  method?: string;
  text: string;
  action?: string;
  quote: string;
};

/** 四层在界面上的名字。服务端 writingLayerNames 的镜像。 */
export const COMMENT_LAYER_NAMES: Record<number, string> = {
  1: "立意",
  2: "材料",
  3: "结构",
  4: "字句",
};

/**
 * 印记's structured critique of a piece of writing — writing_comment.go's
 * Comment, the wire AND stored shape at both zoom levels: `scope: "block"`
 * with `snippetId` set is a comment on one paragraph
 * (commentOnWritingSnippet); `scope: "draft"` with `snippetId: null` is a
 * comment on the whole piece (reviewWritingDraft). One shape, two zoom
 * levels.
 */
/**
 * 这一段现在算什么（服务端 0183 + writing_verdict.go 的闭表）。
 *
 * 🚨 `polish` 的意思是「还可以更好，但**不挡着她往下走**」——
 * 产品里以前没有这个档位，于是任何一条意见读起来都像「你得改」
 *（同事 2026-09-20 的意见 10：「将可选优化判为必改」）。
 * 所以它在屏幕上**不能长得像错误**。
 */
export type CommentVerdict = "pass" | "polish" | "revise";

/**
 * 分层那一排**多一个**值：`unchecked`（服务端 writingVerdictUnchecked，
 * 屏幕上写「本轮未看」）。
 *
 * 🚨 它不进 `CommentVerdict`：整体结论仍然只有三个值。多出来的这个只回答
 * 分层那张图独有的一个问题 —— 服务端只留最上面那一层的 issue
 * （validateCommentPoints 的 dropLowerLayer），所以一层没有 point 有两种
 * 可能：本来就干净，或者查出来了但被压下去了。把后者画成「已通过」是对她
 * 说一句没发生过的话（她改完上面那层再点一次，这一格会忽然变成「可优化」）。
 */
export type CommentLayerVerdict = CommentVerdict | "unchecked";

export type Comment = {
  id: string;
  scope: string;
  /** 空 = 0183 之前的老评论，那一版还没有分级 ⇒ 不渲染那一行标签。 */
  verdict?: CommentVerdict | "";
  snippetId: string | null;
  summary: string;
  points: CommentPoint[];
  createdAt: string;
  /**
   * 写这条意见时印记读的那一版原文。
   *
   * 拿它回答一个 quote 回答不了的问题：**这一段在这条意见之后改过没有。**
   * 空串 = 2026-09-11 之前存的老评论，不知道那一版长什么样。
   */
  sourceText: string;
  /**
   * 四层各自的等级 —— 服务端 layerVerdictsOf（writing_verdict.go）把整体的
   * `verdict` 拆开算的结果，键是 COMMENT_LAYER_NAMES 的键（1..4）转成的字符串，
   * 值在 CommentLayerVerdict 闭集里（比 verdict 多一个 `unchecked`）。
   * **服务端算，不进数据库**，是 points 的纯函数，所以四层永远都有值——
   * 不像 verdict 那样有「空 = 老数据」的历史包袱。
   */
  layer_verdicts: Record<string, CommentLayerVerdict>;
};

const base = (id: string) => `/api/v1/writings/${encodeURIComponent(id)}`;

// --- stage / target words ---------------------------------------------------

export async function setWritingStage(id: string, stage: string): Promise<Writing> {
  return apiFetch<Writing>(`${base(id)}/stage`, { method: "POST", body: JSON.stringify({ stage }) });
}

export async function setWritingTargetWords(id: string, targetWords: number): Promise<Writing> {
  return apiFetch<Writing>(`${base(id)}/target-words`, {
    method: "PUT",
    body: JSON.stringify({ targetWords }),
  });
}

// --- setup + opening --------------------------------------------------------

/**
 * The entry 设定 dialog, in one call: language, optional length, and whatever
 * else she wants to say in her own words. `targetWords: null` is a real
 * answer ("I'd rather not set one") and stays null — 铁律②, length is never a
 * precondition.
 *
 * `note` is not stored in a column of its own: the server appends it to the
 * transcript as one of HER turns, because that is what it is. Every
 * downstream prompt then sees it for free.
 */
export async function setWritingSetup(
  id: string,
  input: { lang: "zh" | "en"; targetWords: number | null; note: string },
): Promise<Writing> {
  return apiFetch<Writing>(`${base(id)}/setup`, {
    method: "PUT",
    body: JSON.stringify({ lang: input.lang, targetWords: input.targetWords, note: input.note }),
  });
}

/**
 * The coach's first line. Idempotent server-side: calling it again replays
 * the existing opening instead of greeting her twice, so a refresh (or a
 * double-invoked effect) is harmless and costs nothing.
 */
export async function postWritingOpening(id: string): Promise<{ reply: string; generated: boolean }> {
  return apiFetch<{ reply: string; generated: boolean }>(`${base(id)}/opening`, { method: "POST" });
}

// --- planning (结构) ---------------------------------------------------

/**
 * One turn of the planning conversation. Returns 印记's reply, the WHOLE map
 * after the turn, and which node ids are new — so the map can highlight what
 * just grew rather than silently re-rendering.
 *
 * The server can only ever ADD nodes on this path (writing_plan.go has no
 * update or delete call in it), so a turn can never rewrite or remove
 * something she put on the map.
 */
export async function postWritingPlanTurn(
  id: string,
  text: string,
): Promise<{ reply: string; outline: WritingOutlineItem[]; addedIds: string[]; ready: boolean }> {
  const raw = await apiFetch<{
    reply: string;
    outline: WritingOutlineItem[];
    addedIds: string[];
    /** 印记 judges the plan is enough to start writing on — see
     *  `writingPlanReply.Ready` (writing_plan.go). Absent on a server that
     *  predates the field, which correctly reads as "not yet". */
    ready?: boolean;
  }>(`${base(id)}/plan/turn`, { method: "POST", body: JSON.stringify({ text }) });
  return {
    reply: raw.reply,
    outline: raw.outline ?? [],
    addedIds: raw.addedIds ?? [],
    ready: raw.ready ?? false,
  };
}

/**
 * 把一份引导整形成前端能安全渲染的样子。
 *
 * 🚨 **逐层兜底，而且 `previous` 要递归。** 服务端把 nil 切片 marshal 成
 * `null`，前端一个 `.map()` 就崩（[[go-nil-slice-becomes-null]]）——
 * 而「上一组」只在她按过一次「换一组问题」之后才出现，正好是那种
 *「上一趟有、这一趟没有」的字段，最容易漏。
 */
function normalizeGuide(raw: Partial<WritingBlockGuide> | null | undefined): WritingBlockGuide {
  return {
    job: raw?.job ?? "",
    methods: raw?.methods ?? [],
    questions: raw?.questions ?? [],
    ...(raw?.previous ? { previous: normalizeGuide(raw.previous) } : {}),
  };
}

/**
 * The guiding box for ONE block. Returns 2–4 questions grounded in her own
 * material — never a sentence she could paste into the essay, which is
 * enforced server-side by dropping anything that isn't a question.
 */
export async function guideWritingBlock(id: string, outlineId: string): Promise<WritingBlockGuide> {
  const raw = await apiFetch<WritingBlockGuide>(
    `${base(id)}/outline/${encodeURIComponent(outlineId)}/guide`,
    { method: "POST" },
  );
  return normalizeGuide(raw);
}

/**
 * The WHOLE outline, guided in ONE model call — POST /writings/{id}/guide
 * (writing_guide.go's guideWritingBlocks). The server STORES what it
 * generates, so this is called at most once per piece: every later load gets
 * the same guidance back for free on `WritingOutlineItem.guide`, which is why
 * 段落 can paint guidance on arrival instead of waiting for a click.
 *
 * Keyed by outline row id, deliberately — a guide lands on the exact block it
 * belongs to rather than walking two lists and hoping the order lines up. The
 * map is normalised here so a block whose guide came back without `methods`
 * or `questions` still renders instead of throwing on `.map`.
 */
export async function guideWritingBlocks(id: string): Promise<Record<string, WritingBlockGuide>> {
  const raw = await apiFetch<{ guides: Record<string, Partial<WritingBlockGuide>> }>(`${base(id)}/guide`, {
    method: "POST",
  });
  const out: Record<string, WritingBlockGuide> = {};
  for (const [outlineId, g] of Object.entries(raw.guides ?? {})) {
    out[outlineId] = normalizeGuide(g);
  }
  return out;
}

// --- 深入一层 (the block-scoped side conversation) ---------------------------

/**
 * One turn of 深入一层 — POST /outline/{oid}/deepen (writing_deepen.go).
 *
 * It is the SAME 印记. Server-side this is a context-isolated sub-agent
 * briefed on one block; the student must never meet a second character, so
 * nothing in this call's drawer, heading or copy introduces one.
 *
 * The endpoint has no write path to `writing_outline` or `writing_snippet`,
 * so it structurally cannot author her outline or her prose — which is why
 * the drawer has no "把这句放进去" affordance to build on top of it.
 */
export async function postWritingBlockDeepen(id: string, outlineId: string, text: string): Promise<string> {
  const raw = await apiFetch<{ reply: string }>(`${base(id)}/outline/${encodeURIComponent(outlineId)}/deepen`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
  return raw.reply ?? "";
}

/**
 * This block's whole deepen thread, oldest first — GET the same path. The
 * thread persists per block, so reopening a block returns to the conversation
 * rather than to a blank slate. No model call, no spend.
 */
export async function getWritingBlockThread(id: string, outlineId: string): Promise<LiteMessage[]> {
  const raw = await apiFetch<{ messages: LiteMessage[] }>(
    `${base(id)}/outline/${encodeURIComponent(outlineId)}/deepen`,
  );
  return raw.messages ?? [];
}

// --- coach turn ---------------------------------------------------------

/**
 * 一块板的种类。闭表，服务端 `writingBoardKinds` 的镜像——认不出来的值服务端
 * 会当成没给，那一轮就退化成一条普通的学生消息（不报错：她摆的东西是真的，
 * 少一句上下文也不该把这一轮弄丢）。
 */
export type WritingBoardKind = "role";

/**
 * 说一句话。
 *
 * `board` 只在这条消息是**摆完一块板**产生的时候给：服务端会据此在那一轮的
 * 上文里加一句说明，好让 印记 知道她刚交了作业，而不是在闲聊。消息本身仍然
 * 是她的话——她怎么摆就是她的判断。
 */
export async function postWritingTurn(id: string, text: string, board?: WritingBoardKind): Promise<LiteTurn> {
  return apiFetch<LiteTurn>(`${base(id)}/turn`, {
    method: "POST",
    body: JSON.stringify(board ? { text, board } : { text }),
  });
}

export async function listWritingMessages(id: string): Promise<LiteMessage[]> {
  const raw = await apiFetch<{ messages: LiteMessage[] }>(`${base(id)}/messages`);
  return raw.messages ?? [];
}

// --- outline --------------------------------------------------------------

export async function getWritingOutline(id: string): Promise<WritingOutlineItem[]> {
  const raw = await apiFetch<{ outline: WritingOutlineItem[] }>(`${base(id)}/outline`);
  return raw.outline ?? [];
}

/**
 * Full replace — every existing row is wiped and the posted array reinserted
 * in order. An empty array clears the outline entirely.
 *
 * `role` and `source` MUST be echoed back from what the server served. The PUT
 * rewrites both columns, so dropping either here would erase every block label
 * — or every material's provenance — the moment she edits one block's text.
 */
export async function putWritingOutline(
  id: string,
  items: { text: string; role: string; kind?: string; method?: string; depth: number; source?: string }[],
): Promise<WritingOutlineItem[]> {
  const raw = await apiFetch<{ outline: WritingOutlineItem[] }>(`${base(id)}/outline`, {
    method: "PUT",
    body: JSON.stringify({ outline: items }),
  });
  return raw.outline ?? [];
}

/*
 * `generateWritingOutline` (POST /outline/generate) was DELETED on
 * 2026-08-27, along with its endpoint. It asked the model to read everything
 * she had said and hand back a finished outline — which is the one thing the
 * ruling forbids: the AI never authors an outline. What replaced it is
 * `recommendWritingStructure` (pick a generic skeleton) plus
 * `guideWritingBlock` (ask her questions about a block). Do not reintroduce
 * it.
 */

// --- snippets ---------------------------------------------------------------

export async function getWritingSnippets(id: string): Promise<WritingSnippet[]> {
  const raw = await apiFetch<{ snippets: WritingSnippet[] }>(`${base(id)}/snippets`);
  return raw.snippets ?? [];
}

/** Upsert by position — NOT a full replace (unlike outline's PUT). Usually a
 *  single-item array: she writes one paragraph at a time. */
export async function putWritingSnippet(
  id: string,
  item: { outlineId?: string | null; position: number; text: string },
): Promise<WritingSnippet[]> {
  const raw = await apiFetch<{ snippets: WritingSnippet[] }>(`${base(id)}/snippets`, {
    method: "PUT",
    body: JSON.stringify({
      snippets: [{ outlineId: item.outlineId ?? null, position: item.position, text: item.text }],
    }),
  });
  return raw.snippets ?? [];
}

// --- compose / draft / review / finish --------------------------------------

/** Deterministic assembly of her own snippets, in outline order — no model
 *  call (composeWritingDraft's own file comment). */
export async function composeWritingDraft(id: string): Promise<WritingDraft> {
  return apiFetch<WritingDraft>(`${base(id)}/compose`, { method: "POST" });
}

export async function getWritingDraft(id: string): Promise<WritingDraft> {
  return apiFetch<WritingDraft>(`${base(id)}/draft`);
}

export async function putWritingDraft(id: string, body: string): Promise<WritingDraft> {
  return apiFetch<WritingDraft>(`${base(id)}/draft`, { method: "PUT", body: JSON.stringify({ body }) });
}

/**
 * Feedback only — never writes to the draft. As of Task 5 this is a
 * structured, PERSISTED `Comment` (scope="draft", snippetId=null), not the
 * old `{feedback: "<prose>"}` wall of text that evaporated on navigation —
 * see writing_compose.go's reviewWritingDraft.
 */
export async function reviewWritingDraft(id: string): Promise<Comment> {
  const raw = await apiFetch<{ comment: Comment }>(`${base(id)}/review`, { method: "POST" });
  return raw.comment;
}

/**
 * Comment on ONE paragraph — POST /snippets/{sid}/comment
 * (writing_comment.go's commentOnSnippet). Points are validated against
 * THIS SNIPPET's text only, never the whole draft.
 */
export async function commentOnWritingSnippet(id: string, snippetId: string): Promise<Comment> {
  const raw = await apiFetch<{ comment: Comment }>(
    `${base(id)}/snippets/${encodeURIComponent(snippetId)}/comment`,
    { method: "POST" },
  );
  return raw.comment;
}

/**
 * Every stored comment for this writing, both scopes together, newest first
 * — GET /comments (writing_comment.go's listWritingComments). No model
 * call, no entitlement gate.
 */
export async function listWritingComments(id: string): Promise<Comment[]> {
  const raw = await apiFetch<{ comments: Comment[] }>(`${base(id)}/comments`);
  return raw.comments ?? [];
}

export async function finishWriting(id: string): Promise<Writing> {
  return apiFetch<Writing>(`${base(id)}/finish`, { method: "POST" });
}

/** What `suggestWritingTitles` answers — writing_title.go's own wire shape.
 *
 *  `needsName` false means she already renamed the piece herself, and the
 *  server did NO model call to tell us so: it is two queries and an equality
 *  check, so the common case costs nothing and 完成这篇 goes straight through.
 *  `ideas` is then empty and must not be shown.
 *
 *  When `needsName` is true, `ideas` may STILL be empty — she pressed 完成这篇
 *  on a draft with nothing in it, so there was nothing to name from. The
 *  dialog then offers a plain box rather than an error. */
export type TitleIdeas = { needsName: boolean; ideas: string[] };

/**
 * Asks whether this piece still needs a name, and if so what it could be
 * called — POST /title-ideas (writing_title.go).
 *
 * The server never applies any of these: it returns candidates, and the
 * client saves whichever one SHE picks through `renameWriting`. See that
 * file's header for why that separation is the whole point.
 */
export async function suggestWritingTitles(id: string): Promise<TitleIdeas> {
  const raw = await apiFetch<Partial<TitleIdeas>>(`${base(id)}/title-ideas`, { method: "POST" });
  return { needsName: raw.needsName === true, ideas: raw.ideas ?? [] };
}

/**
 * 「需要提示」—— 几个摘自她正文的关键词，不是标题（2026-09-18）。
 * 服务端逐个验过都逐字出现在她的正文里（writing_title.go validateTitleKeywords）。
 */
export async function suggestWritingTitleKeywords(id: string): Promise<string[]> {
  const raw = await apiFetch<{ keywords?: string[] }>(`${base(id)}/title-keywords`, { method: "POST" });
  return raw.keywords ?? [];
}

/*
 * The writing room has NO 工具卡 (2026-08-27). listWritingCards /
 * activateWritingCard / skipWritingCard / submitWritingCard /
 * summonWritingCard and their endpoints are all deleted: pro's own writing
 * surface barely used the cards, and what a student stuck on a paragraph
 * needs is a question, not a form to fill in. The reading room keeps its
 * 学科透镜 — those cards are used against an article, which gives them
 * something real to bite on.
 */

// --- 行文（第四步的第二步，服务端 writing_flow.go）-------------------------

/** 那个下拉里的一项（只有论证方法那一层）。服务端 `writingFlowMethodDTO`。 */
export type FlowMethodDTO = { id: string; name: string; definition: string };

/** 服务端 `writingFlowStructureDTO`。 */
export type FlowStructureDTO = {
  id: string;
  name: string;
  definition: string;
  /**
   * 判断办法：什么时候该挑这一条。
   *
   * 🚨 2026-09-23 产品负责人：「英文的四个（Thesis-body-conclusion、
   * Claim-counterargument-refutation、Point-by-point comparison、
   * Block comparison）只有定义和例子，没有判断方法。」
   *
   * 定义说它是什么，例子说它长什么样，两样都答不了「我这一篇该用哪一条」——
   * 而那正是这一屏要她做的决定。老的响应里没有这个字段 ⇒ undefined ⇒ 整行不显示。
   */
  whenToUse?: string;
  example: string;
};

/**
 * 那四条论证结构（总分式 / 并列式 / 层进式 / 对照式）。
 *
 * 从服务端取而不是前端写第二份：名字和定义的真相在 vocab，
 * 两份词表迟早分岔，而分岔的那天她在板上选的结构服务端不认。
 */
export async function getWritingFlowStructures(
  id: string,
): Promise<{ structures: FlowStructureDTO[]; methods: FlowMethodDTO[] }> {
  const raw = await apiFetch<{ structures: FlowStructureDTO[]; methods: FlowMethodDTO[] }>(
    `${base(id)}/flow/structures`,
  );
  // 🚨 逐层兜底：Go 把 nil 切片 marshal 成 null，一个 .map() 就崩。
  return { structures: raw.structures ?? [], methods: raw.methods ?? [] };
}

/**
 * 存行文那一步：整篇的论证结构 + 每一块标的论证方法。
 *
 * 🚨 **不收正文**。同事 2026-09-20 那句括号里的话就是这条路的边界：
 * 「并非填充内容」。
 */
export async function putWritingFlow(
  id: string,
  structureKey: string,
  methods: Record<string, string>,
): Promise<WritingOutlineItem[]> {
  const raw = await apiFetch<{ outline: WritingOutlineItem[] }>(`${base(id)}/flow`, {
    method: "PUT",
    body: JSON.stringify({ structureKey, methods }),
  });
  return raw.outline ?? [];
}
