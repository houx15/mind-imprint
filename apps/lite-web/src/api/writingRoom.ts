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
  role: string;
  depth: number;
  position: number;
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
 *  her essay. */
export type WritingGuidePattern = { label: string; frame: string };

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
export type WritingBlockGuide = { job: string; methods: WritingGuideMethod[]; questions: string[] };

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
export type Comment = {
  id: string;
  scope: string;
  snippetId: string | null;
  summary: string;
  points: CommentPoint[];
  createdAt: string;
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
 * The guiding box for ONE block. Returns 2–4 questions grounded in her own
 * material — never a sentence she could paste into the essay, which is
 * enforced server-side by dropping anything that isn't a question.
 */
export async function guideWritingBlock(id: string, outlineId: string): Promise<WritingBlockGuide> {
  const raw = await apiFetch<WritingBlockGuide>(
    `${base(id)}/outline/${encodeURIComponent(outlineId)}/guide`,
    { method: "POST" },
  );
  return { job: raw.job ?? "", methods: raw.methods ?? [], questions: raw.questions ?? [] };
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
    out[outlineId] = { job: g.job ?? "", methods: g.methods ?? [], questions: g.questions ?? [] };
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

export async function postWritingTurn(id: string, text: string): Promise<LiteTurn> {
  return apiFetch<LiteTurn>(`${base(id)}/turn`, { method: "POST", body: JSON.stringify({ text }) });
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
 * `role` MUST be echoed back from what the server served. The PUT rewrites
 * both columns, so dropping it here would erase every block label the moment
 * she edits one block's text.
 */
export async function putWritingOutline(
  id: string,
  items: { text: string; role: string; depth: number }[],
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

/*
 * The writing room has NO 工具卡 (2026-08-27). listWritingCards /
 * activateWritingCard / skipWritingCard / submitWritingCard /
 * summonWritingCard and their endpoints are all deleted: pro's own writing
 * surface barely used the cards, and what a student stuck on a paragraph
 * needs is a question, not a form to fill in. The reading room keeps its
 * 学科透镜 — those cards are used against an article, which gives them
 * something real to bite on.
 */
