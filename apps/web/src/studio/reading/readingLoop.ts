import { useCallback, useMemo, useState } from "react";
import type { Anchor, MaterialSource, SelectionEval } from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { CreatedSpan } from "../../primitives/annotate/selection";
import type { StudioTurnEvent } from "../../api/studioTurn";

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
  | { id: string; role: "student"; kind: "text"; body: string }
  | { id: string; role: "assistant"; kind: "text"; body: string }
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

export function useReadingLoop(projectId: string, source: MaterialSource, api: ReadingLoopApi): UseReadingLoop {
  const [status, setStatus] = useState<ReadingLoopStatus>("idle");
  const [cardInstanceId, setCardInstanceId] = useState<string | null>(null);
  const [cardId, setCardId] = useState<string | null>(null);
  const [exampleAnchor, setExampleAnchor] = useState<Anchor | null>(null);
  const [exampleWhy, setExampleWhy] = useState("");
  const [studentSpan, setStudentSpan] = useState<CreatedSpan | null>(null);
  const [evalResult, setEvalResult] = useState<SelectionEval | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([GREETING]);
  const [outcomes, setOutcomes] = useState<ReadingOutcome[]>([]);
  const [busy, setBusy] = useState(false);

  const cardName = cardId ? (CARD_REGISTRY[cardId]?.name ?? cardId) : "";

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
  }, []);

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
        // A coach hint — appended, never replacing the thread.
        setMessages((prev) => [...prev, { id: msgId(), role: "assistant", kind: "text", body: ev.body }]);
      }
      // "done" / "review" / "gate" / "error" — nothing to render here.
    }
  }, []);

  const sendTurn = useCallback(
    async (text: string, focusedSpans?: { block_id: string; quote: string }[]) => {
      const trimmed = text.trim();
      if (!trimmed || busy || status !== "idle") return;
      setMessages((prev) => [...prev, { id: msgId(), role: "student", kind: "text", body: trimmed }]);
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
      // Guardrail: the example sentence itself is not a valid pick — she must
      // choose a DIFFERENT sentence. Clicking now selects a whole block, so the
      // guard is block-level: a pick in the example's own block is ignored.
      if (exampleAnchor && span.blockId === exampleAnchor.block_id) {
        return;
      }
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
    [api, projectId, cardInstanceId, cardId, exampleAnchor],
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
    setStatus("active");
  }, []);

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
