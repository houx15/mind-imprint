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

// The minimal structural slice of the real ApiClient the loop needs — a Pick
// so tests can inject a plain object literal (`as any`) exercising only
// these five calls, mirroring StudioContainer's own StudioApi pattern.
export type ReadingLoopApi = {
  readTurn(
    projectId: string,
    materialId: string,
    body: { student_text: string; focused_spans: { block_id: string; quote: string }[] },
  ): AsyncGenerator<StudioTurnEvent>;
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
  coachLines: string[];
  sendTurn: (text: string) => Promise<void>;
  startPick: () => Promise<void>;
  pickSentence: (span: CreatedSpan) => Promise<void>;
  confirm: () => Promise<void>;
  repick: () => void;
  skip: () => Promise<void>;
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
  const [coachLines, setCoachLines] = useState<string[]>([]);

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

  const sendTurn = useCallback(
    async (text: string) => {
      for await (const ev of api.readTurn(projectId, source.id, { student_text: text, focused_spans: [] })) {
        if (ev.type === "card") {
          const anchor = ev.anchors[0] ?? null;
          setCardInstanceId(ev.cardInstanceId);
          setCardId(ev.cardId);
          setExampleAnchor(anchor);
          setExampleWhy(anchor?.question || ev.nudgeText);
          setStudentSpan(null);
          setEvalResult(null);
          setStatus("proposed");
        } else if (ev.type === "intervention") {
          // A coach hint — appended, never replacing the thread; the loop
          // stays idle (no card offered this turn).
          setCoachLines((lines) => [...lines, ev.body]);
        }
        // "done" / "respond" / "gate" / "review" / "error" — nothing to do,
        // stay idle.
      }
    },
    [api, projectId, source.id],
  );

  const startPick = useCallback(async () => {
    if (!cardInstanceId) return;
    await api.activateProjectCard(projectId, cardInstanceId);
    setStatus("active");
  }, [api, projectId, cardInstanceId]);

  const pickSentence = useCallback(
    async (span: CreatedSpan) => {
      if (!cardInstanceId || !cardId) return;
      // Guardrail: the example sentence itself is not a valid pick — she
      // must choose a DIFFERENT one. Same block + same range as the example
      // ⇒ silently ignore (no evaluate call, no state change).
      if (
        exampleAnchor &&
        span.blockId === exampleAnchor.block_id &&
        span.start === exampleAnchor.start &&
        span.end === exampleAnchor.end
      ) {
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
    if (!cardInstanceId || !cardId || !studentSpan) return;
    const anchor = studentSpanToAnchor(source, studentSpan, cardId);
    for await (const ev of api.submitProjectCard(projectId, cardInstanceId, {
      field_values: {},
      event_trace: [],
      anchors: [anchor],
    })) {
      if (ev.type === "done" && ev.cardStatus === "completed") {
        clearCard();
      }
    }
  }, [api, projectId, cardInstanceId, cardId, studentSpan, source, clearCard]);

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
    coachLines,
    sendTurn,
    startPick,
    pickSentence,
    confirm,
    repick,
    skip,
  };
}
