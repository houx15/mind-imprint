import type { LiteCard, LiteMessage, LiteTurn } from "./readingRoom";
import { apiFetch } from "./client";
import type { Writing } from "./writings";

/**
 * api/writingRoom.ts — the room-specific 写作 calls, split out of
 * api/writings.ts the same way api/readingRoom.ts is split from
 * api/readings.ts: writings.ts owns the CRUD shell (list/create/get/rename),
 * this file owns everything that happens once she is INSIDE one — stage/
 * target words, the coach turn, outline, snippets + exemplar, compose/draft/
 * review/finish, and the card lifecycle (list/activate/skip/submit/summon).
 *
 * `LiteCard`/`LiteMessage`/`LiteTurn` are reused from readingRoom.ts, not
 * redeclared: `cardDTO`/`liteMessageDTO`/`liteTurnDTO` (apps/api/internal/
 * api/reading_cards.go, reading_turn.go) are the SAME Go types the writing
 * handlers answer with too (liteListCardsFor/liteActivateCardFor/etc. are
 * curried by kind, and writing_turn.go/writing_lens.go literally construct
 * `liteTurnDTO`) — importing the type is genuine reuse, not a second
 * definition to drift out of sync. Everything else here (outline/snippets/
 * draft) is writing-only, mirroring the Go side's own naming split
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
export type WritingOutlineItem = { id: string; text: string; role: string; depth: number; position: number };

/** One skeleton in the fixed structure library, served by the API so the
 *  labels she sees and the labels stored in `role` cannot drift. */
export type WritingStructure = {
  key: string;
  lang: string;
  name: string;
  blurb: string;
  blocks: { role: string; hint: string }[];
};

/** What the coach hands back when she asks for help on ONE block: questions,
 *  never prose, plus an optional tool-card nomination she must accept. */
export type WritingBlockGuide = { questions: string[]; cardId: string; cardReason: string };
export type WritingSnippet = {
  id: string;
  outlineId: string | null;
  outlineHeading: string;
  position: number;
  text: string;
  updatedAt: string;
};
export type WritingDraft = { body: string; updatedAt: string | null };
export type WritingExemplar = { exemplar: string; prompts: string[] };

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

// --- structure --------------------------------------------------------------

/** The fixed library. Not keyed by writing: it is the same for everyone. */
export async function listWritingStructures(lang: string): Promise<WritingStructure[]> {
  const raw = await apiFetch<{ structures: WritingStructure[] }>(
    `/api/v1/writings/structures?lang=${encodeURIComponent(lang)}`,
  );
  return raw.structures ?? [];
}

/**
 * The model PICKS one from the library and says why. It persists nothing —
 * she accepts by calling `applyWritingStructure`, the same
 * propose-then-she-confirms shape the 工具卡 use.
 */
export async function recommendWritingStructure(id: string): Promise<{ structureKey: string; reason: string }> {
  return apiFetch<{ structureKey: string; reason: string }>(`${base(id)}/structure/recommend`, { method: "POST" });
}

/**
 * Lay out a skeleton's blocks. Every block arrives with a `role` and an EMPTY
 * `text` — the skeleton contributes the labels and not one word of content.
 *
 * Destructive: it replaces the outline. Without `force` the server answers
 * 409 when any block already holds her text, so switching skeletons after
 * she has written something is always a decision she makes on purpose.
 */
export async function applyWritingStructure(
  id: string,
  structureKey: string,
  opts?: { force?: boolean },
): Promise<{ structureKey: string; outline: WritingOutlineItem[] }> {
  const raw = await apiFetch<{ structureKey: string; outline: WritingOutlineItem[] }>(`${base(id)}/structure`, {
    method: "POST",
    body: JSON.stringify({ structureKey, force: opts?.force ?? false }),
  });
  return { structureKey: raw.structureKey, outline: raw.outline ?? [] };
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
  return { questions: raw.questions ?? [], cardId: raw.cardId ?? "", cardReason: raw.cardReason ?? "" };
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

/** English-only demonstration paragraph + guiding questions. NEVER
 *  persisted server-side (writing_snippets.go's own comment) — the caller
 *  must not offer any "insert into my draft" affordance for the result
 *  (铁律①: the AI never writes her prose). */
export async function generateWritingSnippetExemplar(id: string, snippetId: string): Promise<WritingExemplar> {
  return apiFetch<WritingExemplar>(`${base(id)}/snippets/${encodeURIComponent(snippetId)}/exemplar`, {
    method: "POST",
  });
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

/** Feedback only — never writes to the draft. */
export async function reviewWritingDraft(id: string): Promise<{ feedback: string }> {
  return apiFetch<{ feedback: string }>(`${base(id)}/review`, { method: "POST" });
}

export async function finishWriting(id: string): Promise<Writing> {
  return apiFetch<Writing>(`${base(id)}/finish`, { method: "POST" });
}

// --- cards --------------------------------------------------------------

export async function listWritingCards(id: string): Promise<LiteCard[]> {
  const raw = await apiFetch<{ cards: LiteCard[] }>(`${base(id)}/cards`);
  return raw.cards ?? [];
}

export async function activateWritingCard(id: string, cid: string): Promise<LiteCard> {
  return apiFetch<LiteCard>(`${base(id)}/cards/${encodeURIComponent(cid)}/activate`, { method: "POST" });
}

export async function skipWritingCard(id: string, cid: string): Promise<LiteCard> {
  return apiFetch<LiteCard>(`${base(id)}/cards/${encodeURIComponent(cid)}/skip`, { method: "POST" });
}

/**
 * `anchors` is deliberately OMITTED, not sent as `[]`. The four writing-deck
 * cards never populate `CardInstance.anchors` locally (there is no text-span
 * picker in the writing room the way the reading room has one) — it stays
 * `[]` for the whole life of the card. The lite submit endpoint
 * (liteSubmitCardFor, reading_cards.go) treats a PRESENT `anchors` key as an
 * explicit overwrite and an ABSENT one as "keep what the card already has" —
 * so forwarding `env.anchors` unconditionally (an empty array) would silently
 * erase the AI's grounded example anchor from writing_lens.go's summon at the
 * exact moment she completes the card. Pro's own WritingBlock never sends
 * anchors on this call either (`submitProposedCard(field_values, event_trace)`
 * — no anchors argument at all), which is the same avoidance, not an
 * oversight this file is introducing.
 */
export async function submitWritingCard(
  id: string,
  cid: string,
  input: { fieldValues: Record<string, unknown>; eventTrace: unknown[] },
): Promise<LiteCard> {
  return apiFetch<LiteCard>(`${base(id)}/cards/${encodeURIComponent(cid)}/submit`, {
    method: "POST",
    body: JSON.stringify({ fieldValues: input.fieldValues, eventTrace: input.eventTrace }),
  });
}

export async function summonWritingCard(id: string, cardId: string): Promise<LiteTurn> {
  return apiFetch<LiteTurn>(`${base(id)}/summon`, { method: "POST", body: JSON.stringify({ cardId }) });
}
