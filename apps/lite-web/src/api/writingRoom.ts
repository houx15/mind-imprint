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

export type WritingOutlineItem = { id: string; text: string; depth: number; position: number };
export type WritingOutlineCandidate = { text: string; depth: number };
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

/** Full replace — every existing row is wiped and the posted array
 *  reinserted in order. An empty array clears the outline entirely. */
export async function putWritingOutline(
  id: string,
  items: { text: string; depth: number }[],
): Promise<WritingOutlineItem[]> {
  const raw = await apiFetch<{ outline: WritingOutlineItem[] }>(`${base(id)}/outline`, {
    method: "PUT",
    body: JSON.stringify({ outline: items }),
  });
  return raw.outline ?? [];
}

/** A CANDIDATE only — never auto-persisted. The student confirms (or edits
 *  first) via `putWritingOutline`. */
export async function generateWritingOutline(id: string): Promise<WritingOutlineCandidate[]> {
  const raw = await apiFetch<{ outline: WritingOutlineCandidate[] }>(`${base(id)}/outline/generate`, {
    method: "POST",
  });
  return raw.outline ?? [];
}

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
