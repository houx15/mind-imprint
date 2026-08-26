import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Anchor, MaterialSource, SelectionEval } from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { CreatedSpan } from "../../primitives/annotate/selection";
import type { StudioTurnEvent } from "../../api/studioTurn";
import { READING_DECK_IDS } from "./readingDeck";

// The client-only read-together loop state machine (spec §9):
//   idle → proposed → active → evaluating → feedback → idle
// "idle" is the resting state (no card in flight, or the previous one just
// completed) — it is NOT one of HangingCard's own statuses (HangingCard only
// ever renders when there IS a card, i.e. status !== "idle").
export type ReadingLoopStatus = "idle" | "proposed" | "active" | "evaluating" | "feedback";

// One turn in the coach dialogue log (the demo's `.chat-log`). A student
// message is her own words; an assistant message is either a coach hint
// (kind "text") or the notice that a lens was drawn into the article (kind
// "lens", carrying the card name so the bubble can render "去文章看示范").
export type ChatMessage =
  // quotes carries the sentence(s) she had referenced (via 引用原文) when she
  // sent THIS message — undefined/empty when she referenced nothing. Persisting
  // it on the message is what keeps the reference visible after send: the
  // article's own highlight clears on send (ReadingRoom's `refs` resets), but
  // the quote now lives here instead of vanishing with it.
  | { id: string; role: "student"; kind: "text"; body: string; quotes?: string[] }
  // offerCardId/offerCardName: a router "hint" identified a lens that would help
  // but stayed 克制 — instead of dropping that on the floor, the bubble carries
  // a one-tap offer to summon it (touch is automatic, opening stays the
  // student's confirmed choice — 铁律②). undefined on a plain reply.
  | { id: string; role: "assistant"; kind: "text"; body: string; offerCardId?: string; offerCardName?: string }
  | { id: string; role: "assistant"; kind: "lens"; body: string; cardName: string };

// A confirmed reading outcome — the note card that accumulates in the
// 阅读成果 view. It integrates the student's own selected sentence, the
// finding/judgment she reached, and the full AI review (verdict + checks +
// nextStep), plus the span coordinates so 回到原文 can re-focus the source.
export type ReadingOutcome = {
  id: string;
  cardId: string;
  cardName: string;
  blockId: string;
  start: number;
  end: number;
  quote: string;
  finding: string;
  judgment: string;
  support: string;
  caveat: string;
  eval: SelectionEval;
};

// The minimal structural slice of the real ApiClient the loop needs — a Pick
// so tests can inject a plain object literal (`as any`) exercising only
// these five calls, mirroring StudioContainer's own StudioApi pattern.
export type ReadingLoopApi = {
  readTurn(
    projectId: string,
    materialId: string,
    body: { student_text: string; focused_spans: { block_id: string; quote: string }[] },
  ): AsyncGenerator<StudioTurnEvent>;
  // The lens-library summon path — the student picks a card herself rather
  // than waiting for the router to propose one.
  summonCard(projectId: string, materialId: string, cardId: string): AsyncGenerator<StudioTurnEvent>;
  activateProjectCard(projectId: string, cid: string): Promise<void>;
  evaluateCardSelection(
    projectId: string,
    cid: string,
    body: { block_id: string; start: number; end: number; quote: string; dimension: string },
  ): Promise<SelectionEval>;
  submitProjectCard(
    projectId: string,
    cid: string,
    input: { field_values: Record<string, unknown>; event_trace: unknown[]; anchors: Anchor[] },
  ): AsyncGenerator<StudioTurnEvent>;
  skipProjectCard(projectId: string, cid: string, input: { event_trace: unknown[] }): Promise<void>;
  // getOpenCard — the deadlock-prevention fix: on mount, ask whether a
  // proposed/active card is already open for this material (a leftover from
  // before the room reloaded) so it can be resumed instead of vanishing
  // behind the one-active mutex. Optional: a bare test fake that only
  // exercises the other five calls still works (guarded with a typeof check
  // below), and older ApiClient shapes without it degrade to "assume idle".
  getOpenCard?(
    projectId: string,
    materialId: string,
  ): Promise<{ cardInstanceId: string; cardId: string; status: "proposed" | "active"; anchors: Anchor[] } | null>;
};

export type UseReadingLoop = {
  status: ReadingLoopStatus;
  cardId: string | null;
  cardName: string;
  // The AI's own example anchor (author "ai") from the `card` event — the
  // article hangs the card under this block while `status === "proposed"`.
  exampleAnchor: Anchor | null;
  exampleBlockId: string;
  exampleWhy: string;
  // The student's own pick, once she has made one — `null` until then.
  studentSpan: CreatedSpan | null;
  studentAnchor: Anchor | null;
  eval: SelectionEval | null;
  // A transient line for HangingCard (D1): set when pickSentence rejects a
  // click on the AI's own example, cleared automatically a few seconds later
  // (or immediately on the next real pick/repick). Null the rest of the time.
  pickHint: string | null;
  // The coach dialogue log (student + assistant turns), oldest first.
  messages: ChatMessage[];
  // Confirmed reading outcomes, oldest first — the 阅读成果 accumulation.
  outcomes: ReadingOutcome[];
  busy: boolean;
  sendTurn: (text: string, focusedSpans?: { block_id: string; quote: string }[]) => Promise<void>;
  // Lens-library summon: the student picked cardId herself from the deck.
  summonCard: (cardId: string) => Promise<void>;
  startPick: () => Promise<void>;
  pickSentence: (span: CreatedSpan) => Promise<void>;
  confirm: () => Promise<void>;
  repick: () => void;
  skip: () => Promise<void>;
};

let msgSeq = 0;
function msgId() {
  msgSeq += 1;
  return `m${msgSeq}`;
}

const GREETING: ChatMessage = {
  id: "greeting",
  role: "assistant",
  kind: "text",
  body: "文章已经准备好了。直接说出你的疑问——只有在某个视角确实有帮助时，我才会把一副透镜放进原文。",
};

function studentSpanToAnchor(source: MaterialSource, span: CreatedSpan, dimension: string): Anchor {
  return {
    id: "sel0",
    material_id: source.id,
    block_id: span.blockId,
    start: span.start,
    end: span.end,
    quote: span.text,
    dimension,
    author: "student",
    question: "",
    answer: "",
  };
}

export function useReadingLoop(
  projectId: string,
  source: MaterialSource,
  api: ReadingLoopApi,
  // initialMessages — the guided-tour demo replay (Task 4): seeds the chat
  // log with a hand-authored transcript instead of the live GREETING, so a
  // read-only demo room opens already "mid-conversation". Undefined/empty →
  // unchanged default behavior ([GREETING]).
  initialMessages?: ChatMessage[],
  // initialOutcomes — the guided-tour demo replay (P8): seeds the 阅读成果
  // accumulation with one already-confirmed finding, so the 阅读成果 tab shows a
  // real result (选句 + 结论 + 完整复核) instead of an empty placeholder. Its
  // span stays highlighted in the article too (see the `spans` memo). Undefined/
  // empty → unchanged default ([]).
  initialOutcomes?: ReadingOutcome[],
): UseReadingLoop {
  const [status, setStatus] = useState<ReadingLoopStatus>("idle");
  const [cardInstanceId, setCardInstanceId] = useState<string | null>(null);
  const [cardId, setCardId] = useState<string | null>(null);
  const [exampleAnchor, setExampleAnchor] = useState<Anchor | null>(null);
  const [exampleWhy, setExampleWhy] = useState("");
  const [studentSpan, setStudentSpan] = useState<CreatedSpan | null>(null);
  const [evalResult, setEvalResult] = useState<SelectionEval | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>(
    initialMessages && initialMessages.length > 0 ? initialMessages : [GREETING],
  );
  const [outcomes, setOutcomes] = useState<ReadingOutcome[]>(
    initialOutcomes && initialOutcomes.length > 0 ? initialOutcomes : [],
  );
  const [busy, setBusy] = useState(false);
  // pickHint (Task 18 / D1) — a transient line shown on HangingCard when
  // pickSentence silently rejects a pick. Before this, clicking the AI's
  // underlined example (the single most natural click on screen) did
  // NOTHING — no message, no shake — because the guard below just
  // `return`ed. The rejection itself is correct (she must find her OWN
  // sentence); only the silence was the bug. Auto-clears after a few
  // seconds, or immediately on the next real pick/repick/card change, so it
  // never lingers as stale copy.
  const [pickHint, setPickHint] = useState<string | null>(null);
  const pickHintTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const clearPickHintTimer = useCallback(() => {
    if (pickHintTimerRef.current != null) {
      clearTimeout(pickHintTimerRef.current);
      pickHintTimerRef.current = null;
    }
  }, []);
  useEffect(() => clearPickHintTimer, [clearPickHintTimer]);

  const cardName = cardId ? (CARD_REGISTRY[cardId]?.name ?? cardId) : "";

  // Deadlock prevention: on mount, ask whether a card is already open
  // (proposed/active) for THIS material — a leftover from before the room
  // reloaded, invisible until now behind the project-wide one-active mutex —
  // and if so, resume the loop into it rather than leaving the student stuck
  // with no card and no way to summon a new one. Keyed on projectId+source.id
  // and guarded by the ref so it fires exactly once per material, including
  // under StrictMode's dev double-invoke.
  const openCardLoadedRef = useRef(false);
  useEffect(() => {
    if (openCardLoadedRef.current) return;
    openCardLoadedRef.current = true;
    if (typeof api.getOpenCard !== "function") return;
    let cancelled = false;
    void (async () => {
      const open = await api.getOpenCard!(projectId, source.id);
      if (cancelled || !open) return;
      const anchor = open.anchors[0] ?? null;
      setCardInstanceId(open.cardInstanceId);
      setCardId(open.cardId);
      setExampleAnchor(anchor);
      setExampleWhy(anchor?.question || "");
      setStatus(open.status);
      const name = CARD_REGISTRY[open.cardId]?.name ?? open.cardId;
      setMessages((prev) => [
        ...prev,
        {
          id: msgId(),
          role: "assistant",
          kind: "lens",
          body: "这副透镜还没有完成——你可以继续，或者跳过它。",
          cardName: name,
        },
      ]);
    })();
    return () => {
      cancelled = true;
    };
  }, [api, projectId, source.id]);

  // Shared by confirm() (on a completed submit) and skip() — both retire the
  // in-flight card and return the loop to its resting state.
  const clearCard = useCallback(() => {
    setStatus("idle");
    setCardInstanceId(null);
    setCardId(null);
    setExampleAnchor(null);
    setExampleWhy("");
    setStudentSpan(null);
    setEvalResult(null);
    setPickHint(null);
    clearPickHintTimer();
  }, [clearPickHintTimer]);

  // applyTurnEvents drains one SSE turn (readTurn OR summonCard both yield
  // the same StudioTurnEvent vocabulary) and applies its card/intervention
  // events to loop state — shared by sendTurn (router-proposed summon) and
  // summonCard (student-chosen summon) below.
  const applyTurnEvents = useCallback(async (events: AsyncGenerator<StudioTurnEvent>) => {
    for await (const ev of events) {
      if (ev.type === "card") {
        const anchor = ev.anchors[0] ?? null;
        setCardInstanceId(ev.cardInstanceId);
        setCardId(ev.cardId);
        setExampleAnchor(anchor);
        setExampleWhy(anchor?.question || ev.nudgeText);
        setStudentSpan(null);
        setEvalResult(null);
        setStatus("proposed");
        const name = CARD_REGISTRY[ev.cardId]?.name ?? ev.cardId;
        setMessages((prev) => [
          ...prev,
          { id: msgId(), role: "assistant", kind: "lens", body: ev.nudgeText, cardName: name },
        ]);
      } else if (ev.type === "intervention") {
        // A coach reply/hint — appended, never replacing the thread. When it's a
        // "hint" naming a real reading-deck lens (ev.criterion = card id), carry
        // that as a tappable offer so a 克制 hint still gives her a way in.
        const offerCardId =
          ev.level === "hint" && ev.criterion && (READING_DECK_IDS as readonly string[]).includes(ev.criterion)
            ? ev.criterion
            : undefined;
        const offerCardName = offerCardId ? (CARD_REGISTRY[offerCardId]?.name ?? offerCardId) : undefined;
        setMessages((prev) => [
          ...prev,
          { id: msgId(), role: "assistant", kind: "text", body: ev.body, offerCardId, offerCardName },
        ]);
      }
      // "done" / "review" / "gate" / "error" — nothing to render here.
    }
  }, []);

  const sendTurn = useCallback(
    async (text: string, focusedSpans?: { block_id: string; quote: string }[]) => {
      const trimmed = text.trim();
      if (!trimmed || busy || status !== "idle") return;
      const quotes = focusedSpans?.map((s) => s.quote).filter(Boolean);
      setMessages((prev) => [
        ...prev,
        { id: msgId(), role: "student", kind: "text", body: trimmed, quotes: quotes?.length ? quotes : undefined },
      ]);
      setBusy(true);
      try {
        await applyTurnEvents(
          api.readTurn(projectId, source.id, { student_text: trimmed, focused_spans: focusedSpans ?? [] }),
        );
      } finally {
        setBusy(false);
      }
    },
    [api, projectId, source.id, busy, status, applyTurnEvents],
  );

  // summonCard — the lens-library path: the student picked cardId herself
  // from the deck (LensLibrary.onPick), rather than waiting for the router
  // to propose one. No student chat bubble (she didn't type anything) — the
  // card/intervention frame the server streams back carries the whole
  // response, same as sendTurn's.
  const summonCard = useCallback(
    async (pickedCardId: string) => {
      if (busy || status !== "idle") return;
      setBusy(true);
      try {
        await applyTurnEvents(api.summonCard(projectId, source.id, pickedCardId));
      } finally {
        setBusy(false);
      }
    },
    [api, projectId, source.id, busy, status, applyTurnEvents],
  );

  const startPick = useCallback(async () => {
    if (!cardInstanceId) return;
    await api.activateProjectCard(projectId, cardInstanceId);
    setStatus("active");
  }, [api, projectId, cardInstanceId]);

  const pickSentence = useCallback(
    async (span: CreatedSpan) => {
      if (!cardInstanceId || !cardId) return;
      // Guardrail: the AI's EXAMPLE sentence itself is not a valid pick — she
      // must choose a DIFFERENT one. Now that a click selects a single sentence
      // (#9), this is a precise range-overlap test against the example, so a
      // different sentence IN THE SAME block is allowed. Exception: if she
      // picked the whole block (single-sentence fallback — an abstract-only
      // source with one sentence), there is no alternative, so allow it rather
      // than freeze the room (#20).
      if (exampleAnchor && span.blockId === exampleAnchor.block_id) {
        const block = source.blocks.find((b) => b.id === span.blockId);
        const blockLen = block ? Array.from(block.text).length : 0;
        const pickedWholeBlock = span.start === 0 && span.end >= blockLen;
        const overlapsExample = span.start < exampleAnchor.end && exampleAnchor.start < span.end;
        // Skip the rejection ONLY when there is genuinely nothing else to pick:
        // a single-block source where she picked the whole (one-sentence) block.
        // With other blocks present she still has an alternative, so reject.
        const noAlternative = source.blocks.length <= 1 && pickedWholeBlock;
        if (overlapsExample && !noAlternative) {
          // D1: tell her plainly, in the product's voice ("示范"/"证据句" are
          // the same terms HangingCard already uses), instead of doing
          // nothing. Transient — clears itself so it never reads as a
          // lingering error banner.
          clearPickHintTimer();
          setPickHint("这句是示范句——换一句你自己的证据句。");
          pickHintTimerRef.current = setTimeout(() => setPickHint(null), 2600);
          return;
        }
      }
      clearPickHintTimer();
      setPickHint(null);
      setStudentSpan(span);
      setStatus("evaluating");
      const result = await api.evaluateCardSelection(projectId, cardInstanceId, {
        block_id: span.blockId,
        start: span.start,
        end: span.end,
        quote: span.text,
        dimension: cardId,
      });
      // BINDING (Task 9 review): eval must be set BEFORE status flips to
      // "feedback" — HangingCard must never render status:"feedback" with a
      // null eval.
      setEvalResult(result);
      setStatus("feedback");
    },
    [api, projectId, cardInstanceId, cardId, exampleAnchor, source, clearPickHintTimer],
  );

  const confirm = useCallback(async () => {
    if (!cardInstanceId || !cardId || !studentSpan || !evalResult) return;
    const anchor = studentSpanToAnchor(source, studentSpan, cardId);
    // Snapshot the outcome up front (the demo's completeLens: the confirmed
    // finding is SAVED, not discarded) so clearCard's reset can't race it.
    const outcome: ReadingOutcome = {
      id: `outcome-${cardInstanceId}`,
      cardId,
      cardName,
      blockId: studentSpan.blockId,
      start: studentSpan.start,
      end: studentSpan.end,
      quote: studentSpan.text,
      finding: evalResult.finding,
      judgment: evalResult.judgment,
      support: evalResult.support,
      caveat: evalResult.caveat,
      eval: evalResult,
    };
    for await (const ev of api.submitProjectCard(projectId, cardInstanceId, {
      field_values: {},
      event_trace: [],
      anchors: [anchor],
    })) {
      if (ev.type === "done" && ev.cardStatus === "completed") {
        setOutcomes((prev) => [...prev, outcome]);
        setMessages((prev) => [
          ...prev,
          {
            id: msgId(),
            role: "assistant",
            kind: "text",
            body: "这条阅读成果已保存到「阅读成果」——你的选句、发现和完整复核都合并在一起了。现在可以回到原问题继续。",
          },
        ]);
        clearCard();
      }
    }
  }, [api, projectId, cardInstanceId, cardId, studentSpan, evalResult, cardName, source, clearCard]);

  const repick = useCallback(() => {
    setStudentSpan(null);
    setEvalResult(null);
    setPickHint(null);
    clearPickHintTimer();
    setStatus("active");
  }, [clearPickHintTimer]);

  const skip = useCallback(async () => {
    if (!cardInstanceId) return;
    await api.skipProjectCard(projectId, cardInstanceId, { event_trace: [] });
    clearCard();
  }, [api, projectId, cardInstanceId, clearCard]);

  const studentAnchor = useMemo(
    () => (studentSpan && cardId ? studentSpanToAnchor(source, studentSpan, cardId) : null),
    [studentSpan, cardId, source],
  );

  return {
    status,
    cardId,
    cardName,
    exampleAnchor,
    exampleBlockId: exampleAnchor?.block_id ?? "",
    exampleWhy,
    studentSpan,
    studentAnchor,
    eval: evalResult,
    pickHint,
    messages,
    outcomes,
    busy,
    sendTurn,
    summonCard,
    startPick,
    pickSentence,
    confirm,
    repick,
    skip,
  };
}
