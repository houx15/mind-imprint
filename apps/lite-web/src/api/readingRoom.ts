import type { Anchor, ReadingBrief, SelectionEval, TakeawayDraft } from "@mind-imprint/contracts";
import { CARD_REGISTRY, SelectionEval as SelectionEvalSchema } from "@mind-imprint/contracts";
import type { StudioTurnEvent } from "@/api/studioTurn";
import type { ReadingRoomApi } from "@/studio/reading/ReadingRoom";
import type { ReadingOutcome } from "@/studio/reading/readingLoop";
import { apiFetch } from "./client";

/**
 * api/readingRoom.ts — the injected `api` object `ReadingRoom` (apps/web) runs
 * on, backed entirely by the lite `/api/v1/readings/{id}/…` endpoints.
 *
 * Two shape mismatches to be deliberate about, because they are the whole job
 * of this file:
 *
 * 1. **`{id}` is ALWAYS the atom id.** Every method keeps pro's parameter
 *    positions (`projectId`, `materialId`, `rid`) so the room needs no
 *    special-casing, and every one of them is IGNORED — the reading id is
 *    closed over at construction. A lite reading has no project and no
 *    reference row; the atom IS the addressing unit.
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

export type LiteBrief = {
  phaseTag: string | null;
  readingReason: string;
  readingFocus: string;
};

export type LiteMessage = { seq: number; role: string; content: string; createdAt: string };

export type ReadingRoomApiOptions = {
  /** Called when a coach turn or a lens summon fails outright. The room's own
   *  loop has nowhere to show an `error` event, so the host surfaces it. */
  onAiError?: (message: string) => void;
};

const base = (id: string) => `/api/v1/readings/${encodeURIComponent(id)}`;

// --- the api object ---------------------------------------------------------

export function createReadingRoomApi(readingId: string, opts: ReadingRoomApiOptions = {}): ReadingRoomApi {
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
      const message = err instanceof Error ? err.message : "AI 暂时没接上，请重试。";
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

    // --- the three brief/takeaway calls the room drives directly ---------
    async putReadingBrief(_projectId, _rid, brief: ReadingBrief) {
      await apiFetch<LiteBrief>(`${root}/brief`, {
        method: "PUT",
        body: JSON.stringify({
          phaseTag: brief.phaseTag === "" ? null : brief.phaseTag,
          readingReason: brief.readingReason,
          readingFocus: brief.readingFocus,
        }),
      });
    },

    async getTakeawayDraft(_projectId, _rid): Promise<TakeawayDraft> {
      // Assembled DETERMINISTICALLY from her own confirmed cards — no model
      // call, nothing re-guessed. That is what the pro endpoint's `record`
      // half is too; lite just assembles it from atom_card instead of from a
      // reference's card_instances.
      const [{ cards }, takeaway] = await Promise.all([
        apiFetch<{ cards: LiteCard[] }>(`${root}/cards`),
        apiFetch<{ text: string }>(`${root}/takeaway`),
      ]);
      const submitted = cards.filter((c) => c.status === "submitted");
      const findings = submitted.map((c) => c.framework?.finding ?? "").filter(Boolean);
      // HER sentence only — never anchors[0], which on a card missing the
      // student anchor is the AI's example. See studentAnchorOf: quoting the
      // AI's sentence back to her as a key quote of her own is the same
      // 铁律① leak toReadingOutcomes had.
      const keyQuotes = submitted
        .map((c) => ({ quote: studentAnchorOf(c)?.quote ?? "", why: c.framework?.finding ?? "" }))
        .filter((q) => q.quote);
      return {
        record: {
          findings,
          // No credibility verdict in lite: there is no producer for one, and
          // 设计铁律① says we never invent a judgment on the student's behalf.
          credibility: { verdict: "", why: "" },
          keyQuotes,
        },
        // A lite reading has no proposal to feed leads into — the room hides
        // the field (capabilities.proposalImpact === false).
        suggestedNewLeads: [],
        // Seeded with whatever 收获 she has already written, never with a
        // model's guess at it.
        suggestedProposalImpact: takeaway.text ?? "",
      };
    },

    async postFinalizeReading(_projectId, _rid, body) {
      // Two steps, in this order: her 收获 must be persisted BEFORE finish,
      // because the finish endpoint refuses an empty takeaway (400
      // missing_takeaway) — that gate is what stops a reading being marked
      // done with nothing written.
      await apiFetch<{ text: string }>(`${root}/takeaway`, {
        method: "PUT",
        body: JSON.stringify({ text: body.proposalImpact }),
      });
      return apiFetch<unknown>(`${root}/finish`, { method: "POST" });
    },
  };
}

// --- reads the host needs before it can mount the room ---------------------

export async function getReadingBrief(id: string): Promise<LiteBrief> {
  return apiFetch<LiteBrief>(`${base(id)}/brief`);
}

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
export async function postReadingCoachTurn(
  id: string,
  text: string,
  picks: { blockId: string; quote: string }[] = [],
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
  }>(`/api/v1/readings/${encodeURIComponent(id)}/coach`, {
    method: "POST",
    body: JSON.stringify({ text, picks }),
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

export type ReadingBlockTool = { id: string; label: string };

/** Which tools exist depends on the ARTICLE's language. Fetched rather than
 *  hardcoded so the buttons she sees and the ids the server accepts cannot
 *  drift apart. */
export async function listReadingBlockTools(id: string): Promise<{ lang: string; tools: ReadingBlockTool[] }> {
  const raw = await apiFetch<{ lang: string; tools: ReadingBlockTool[] }>(
    `/api/v1/readings/${encodeURIComponent(id)}/blocks/tools`,
  );
  return { lang: raw.lang ?? "zh", tools: raw.tools ?? [] };
}

export type ReadingBlockNote = { blockId: string; tool: string; body: string };

/** Everything she has already opened, so a reload restores it instead of
 *  making her pay for it twice. */
export async function listReadingBlockNotes(id: string): Promise<ReadingBlockNote[]> {
  const raw = await apiFetch<{ notes: ReadingBlockNote[] }>(
    `/api/v1/readings/${encodeURIComponent(id)}/blocks/notes`,
  );
  return raw.notes ?? [];
}

/** Explain ONE paragraph with ONE tool. Cached server-side by (blockId, tool),
 *  so a second call is instant and free. */
export async function explainReadingBlock(
  id: string,
  blockId: string,
  tool: string,
): Promise<ReadingBlockNote & { cached: boolean }> {
  return apiFetch<ReadingBlockNote & { cached: boolean }>(
    `/api/v1/readings/${encodeURIComponent(id)}/blocks/${encodeURIComponent(blockId)}/explain`,
    { method: "POST", body: JSON.stringify({ tool }) },
  );
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
