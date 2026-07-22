import { useState } from "react";
import type { Anchor, CardInstance, CardSpec, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

// Merged onto the anchor at lock time once the student has located the
// sentence herself (N3c §8 — the container writes this after she selects
// text in the article pane). Shape mirrors `CreatedSpan` (Task 7) plus
// `quote`, matching what the anchor itself carries.
type LocatedSpan = { block_id: string; start: number; end: number; quote: string };

export type StudioAnnotateCardProps = {
  spec: CardSpec;
  anchors: Anchor[];
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
  // L2/L3 only (spec §5, §8). All three are OPTIONAL and go together: an
  // older host that cannot switch panes to the article passes none of them,
  // and this renderer degrades to answer-only with no dead controls — see
  // the dead-control comment at the locate/escape block below.
  locatedSpans?: Record<string, LocatedSpan>;
  onRequestLocate?: (anchorId: string, dimension: string) => void;
  onSpanNotFound?: (anchorId: string, dimension: string) => void;
};

const RISK_NOTE_QUESTION = "这条来源在你的论证里起什么作用？有什么风险 / 局限？";

// The guidance level (L1/L2/L3) is NEVER a stored field, never a prop, never
// sent over the wire (spec §2) — it is derived from how each anchor arrived:
//   - author "ai"                        → "answer": AI wrote the question
//     AND circled the sentence; she only answers. Today's behavior, unchanged.
//   - author "student" + a question      → "locate": AI wrote the question;
//     she finds the sentence herself.
//   - author "student" + blank/whitespace question → "elicit": she writes
//     the question herself too, on top of locating.
// This function IS the level — do not add a parallel "level" field anywhere.
export type AnchorMode = "answer" | "locate" | "elicit";

export function anchorMode(a: Anchor): AnchorMode {
  if (a.author === "ai") return "answer";
  return a.question.trim() === "" ? "elicit" : "locate";
}

// Authorship-agnostic annotate answer host (design s3CoachCard, ~L1282-1320).
// It renders whatever anchors it is handed — chip = dimension, question,
// answer textarea, a checkmark once answered — and never assumes the AI
// authored them. Guidance levels L2/L3 (student finds spans / elicits the
// questions themselves) change only WHERE the anchors come from upstream;
// this renderer stays exactly the same either way. Do not hardcode the five
// CRAAP dimensions or specific question copy here.
export function StudioAnnotateCard({
  spec,
  anchors,
  onSubmit,
  onSkip,
  locatedSpans,
  onRequestLocate,
  onSpanNotFound,
}: StudioAnnotateCardProps) {
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [riskNote, setRiskNote] = useState("");
  // L3 only: the question she writes herself. Keyed by anchor id, same
  // pattern as `answers`.
  const [questions, setQuestions] = useState<Record<string, string>>({});
  // Per-anchor "找不到合适的句子" escapes she has taken (locate/elicit modes
  // only). Local because the container (Task 9) only needs the one-shot
  // onSpanNotFound callback; the card itself must still track it to unblock
  // its own lock button.
  const [escapes, setEscapes] = useState<Record<string, boolean>>({});

  // Completable only once every anchor has a non-empty answer AND the
  // risk-note is non-empty — matches the design's per-dimension ✓ + lock
  // intent. Locking before that leaves the backend's card `active` with no
  // mint, and the frontend has no rehydration path for that state, so we
  // gate the button rather than let the student lose the card in-session.
  //
  // FIX 2 (whole-branch review CRITICAL): `[].every()` is vacuously true —
  // when `anchors` is empty (studioturn.go's surfaceAnchors degrades to no
  // anchors on ANY generator failure: a parse error, a provider hiccup, an
  // empty generation), `allAnchorsAnswered` was true with nothing answered,
  // so typing only the risk note made a zero-anchor card lockable. The
  // server's craap.json completion predicate (every_tag_present over 5
  // fixed tags) can never be satisfied by zero submitted anchors, so that
  // submit could only ever leave the card_instance "active" with nothing
  // minted — exactly the FIX 1 "stuck active, card discarded anyway"
  // scenario this file's own comment above warns about. Requiring at least
  // one anchor closes the hole at its narrowest: a card this renderer was
  // handed nothing to ask about must never present a live lock button.
  const hasAnchors = anchors.length > 0;
  const allAnchorsAnswered = hasAnchors && anchors.every((a) => !!(answers[a.id] ?? a.answer)?.trim());

  // Locating is client-side encouragement ONLY (spec §5) — the server's
  // completion predicate never requires a located span, so this gate must
  // never be able to strand her: every non-"answer" anchor needs a located
  // span OR a taken escape, and the escape is always available. DEAD-CONTROL
  // RULE: when the host gave us no onRequestLocate, we render no locate/
  // escape controls at all (below), so we must not gate on them either — a
  // gate with no way to satisfy it is a wall.
  const nonAnswerAnchors = anchors.filter((a) => anchorMode(a) !== "answer");
  const allLocatedOrEscaped =
    !onRequestLocate || nonAnswerAnchors.every((a) => !!locatedSpans?.[a.id] || !!escapes[a.id]);

  // "elicit" anchors additionally need a written question. No escape covers
  // this one on purpose (spec §5, brief): writing your own question cannot
  // fail the way searching for a sentence can.
  const elicitAnchors = anchors.filter((a) => anchorMode(a) === "elicit");
  const allQuestionsWritten = elicitAnchors.every((a) => !!(questions[a.id] ?? "").trim());

  const canLock =
    hasAnchors && allAnchorsAnswered && !!riskNote.trim() && allLocatedOrEscaped && allQuestionsWritten;

  function handleAnswerChange(id: string, value: string) {
    setAnswers((prev) => ({ ...prev, [id]: value }));
  }

  function handleQuestionChange(id: string, value: string) {
    setQuestions((prev) => ({ ...prev, [id]: value }));
  }

  function handleNotFound(anchorId: string, dimension: string) {
    setEscapes((prev) => ({ ...prev, [anchorId]: true }));
    onSpanNotFound?.(anchorId, dimension);
  }

  function handleLock() {
    const filled: Anchor[] = anchors.map((a) => {
      const mode = anchorMode(a);
      const located = locatedSpans?.[a.id];
      return {
        ...a,
        answer: answers[a.id] ?? a.answer,
        question: mode === "elicit" ? (questions[a.id] ?? a.question) : a.question,
        ...(located ? { block_id: located.block_id, start: located.start, end: located.end, quote: located.quote } : {}),
      };
    });
    const materialId = anchors[0]?.material_id ?? "";
    filled.push({
      id: "risk_note",
      material_id: materialId,
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      dimension: "risk_note",
      author: "student",
      question: RISK_NOTE_QUESTION,
      answer: riskNote.trim(),
    });

    // 铁律 4 · 过程即数据: a dimension she looked for and could not find is
    // DATA, not an error. Record `span_not_found` for every escape she took
    // that never got superseded by an actual located span.
    const notFoundEvents: TraceEvent[] = nonAnswerAnchors
      .filter((a) => escapes[a.id] && !locatedSpans?.[a.id])
      .map((a) => ({ kind: "span_not_found", dimension: a.dimension, at: new Date().toISOString() }));

    const env: CardInstance = { ...newEnvelope(spec.id, ""), anchors: filled, event_trace: notFoundEvents };
    onSubmit(env);
  }

  function handleSkip() {
    const scaffold = newEnvelope(spec.id, "");
    onSkip(scaffold.event_trace);
  }

  return (
    <div style={{ border: "1px solid #F0DACF", borderRadius: 14, overflow: "hidden", boxShadow: "0 3px 14px rgba(217,130,99,.10)" }}>
      <div style={{ height: 4, background: "#D98263" }} />
      <div style={{ padding: "12px 15px 8px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#D98263" }}>工具卡 · CRAAP</span>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 8px", borderRadius: 999 }}>
            {spec.category}
          </span>
        </div>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>评估这条来源</div>
      </div>

      <div style={{ padding: "0 15px 14px" }}>
        <div style={{ fontSize: 11.5, color: "#8A92A3", lineHeight: 1.6, marginBottom: 10 }}>
          左边点亮的句子对应下面每个维度——挑一个作答，理由自己写：
        </div>

        {anchors.map((a) => {
          const answered = !!(answers[a.id] ?? a.answer)?.trim();
          const mode = anchorMode(a);
          const located = locatedSpans?.[a.id];
          const escaped = !!escapes[a.id];
          return (
            <div
              key={a.id}
              style={{
                border: "1px solid #ECEEF3",
                borderRadius: 12,
                padding: "13px 15px",
                marginBottom: 10,
                background: "#fff",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 7 }}>
                <span
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 6,
                    fontSize: 11,
                    fontWeight: 700,
                    padding: "3px 10px",
                    borderRadius: 999,
                    color: "#fff",
                    background: "#2A3B7A",
                  }}
                >
                  {a.dimension}
                </span>
                {answered && (
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                )}
              </div>

              {/* "elicit" (L3): she writes the question herself — no AI text
                  to show. Every other mode ("answer"/"locate") keeps today's
                  read-only question line unchanged. */}
              {mode === "elicit" ? (
                <input
                  type="text"
                  aria-label={`${a.dimension}-question`}
                  value={questions[a.id] ?? ""}
                  onChange={(e) => handleQuestionChange(a.id, e.target.value)}
                  placeholder="写下你想问的问题——针对这条来源，你自己想核什么？"
                  style={{
                    width: "100%",
                    border: "1px solid #E1E4ED",
                    borderRadius: 9,
                    padding: "7px 10px",
                    fontSize: 12.5,
                    lineHeight: 1.6,
                    color: "#1C2333",
                    background: "#fff",
                    outline: "none",
                    fontFamily: "inherit",
                  }}
                />
              ) : (
                <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#2B3346", fontWeight: 500 }}>{a.question}</div>
              )}

              <textarea
                aria-label={`${a.dimension}-answer`}
                value={answers[a.id] ?? a.answer}
                onChange={(e) => handleAnswerChange(a.id, e.target.value)}
                rows={2}
                style={{
                  width: "100%",
                  marginTop: 8,
                  border: "1px solid #E1E4ED",
                  borderRadius: 9,
                  padding: "8px 10px",
                  fontSize: 12,
                  lineHeight: 1.55,
                  color: "#1C2333",
                  background: "#fff",
                  outline: "none",
                  resize: "vertical",
                  fontFamily: "inherit",
                }}
              />

              {/* Locating is never a wall (铁律 2, spec §5): the escape must
                  always be able to unblock the lock. DEAD-CONTROL RULE: an
                  older host with no onRequestLocate cannot switch panes, so
                  neither control renders — a control that cannot work must
                  not be shown (mirrors the !hasAnchors precedent above). */}
              {mode !== "answer" && onRequestLocate && (
                <div style={{ marginTop: 8 }}>
                  {located ? (
                    <div style={{ fontSize: 11.5, color: "#4C9A82" }}>
                      已在文章里定位：「{located.quote}」
                    </div>
                  ) : escaped ? (
                    <div style={{ fontSize: 11.5, color: "#9AA1B0" }}>
                      已记录：这条没能在文章里找到合适的句子。
                    </div>
                  ) : (
                    <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                      <button
                        type="button"
                        onClick={() => onRequestLocate(a.id, a.dimension)}
                        style={{
                          background: "none",
                          border: "1px solid #2A3B7A",
                          color: "#2A3B7A",
                          fontSize: 11.5,
                          fontWeight: 600,
                          cursor: "pointer",
                          padding: "5px 10px",
                          borderRadius: 8,
                          fontFamily: "inherit",
                        }}
                      >
                        去文章里选出这句
                      </button>
                      <button
                        type="button"
                        onClick={() => handleNotFound(a.id, a.dimension)}
                        style={{
                          background: "none",
                          border: "none",
                          color: "#9AA1B0",
                          fontSize: 11.5,
                          fontWeight: 500,
                          cursor: "pointer",
                          padding: "5px 0",
                          textDecoration: "underline",
                          fontFamily: "inherit",
                        }}
                      >
                        找不到合适的句子
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}

        <div style={{ fontSize: 11, fontWeight: 700, color: "#9AA1B0", margin: "12px 0 6px" }}>作用与风险（自己写）</div>
        <textarea
          value={riskNote}
          onChange={(e) => setRiskNote(e.target.value)}
          rows={3}
          placeholder="这条来源在你的论证里起什么作用？有什么风险 / 局限？"
          style={{
            width: "100%",
            border: "1px solid #E1E4ED",
            borderRadius: 10,
            padding: "9px 11px",
            fontSize: 12.5,
            lineHeight: 1.6,
            color: "#1C2333",
            background: "#fff",
            outline: "none",
            resize: "vertical",
            fontFamily: "inherit",
          }}
        />

        {/* A card that arrived with no anchors can never satisfy its completion
            predicate, so its lock button is legitimately dead (hasAnchors,
            above). Say WHY, and point at the exit she does have — a disabled
            control with no explanation reads as a broken product. */}
        {!hasAnchors && (
          <div style={{ marginTop: 12, fontSize: 12, lineHeight: 1.6, color: "#C96F4F" }}>
            这张卡没能取到要核对的句子，暂时锁不了。先跳过，稍后再核这条来源。
          </div>
        )}

        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 14 }}>
          <button
            type="button"
            onClick={handleSkip}
            style={{ background: "none", border: "none", color: "#C2557A", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: "6px 0", fontFamily: "inherit" }}
          >
            跳过这张卡
          </button>
          <button
            type="button"
            onClick={handleLock}
            disabled={!canLock}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              fontSize: 13,
              fontWeight: 700,
              cursor: canLock ? "pointer" : "not-allowed",
              opacity: canLock ? 1 : 0.5,
              padding: "9px 15px",
              borderRadius: 10,
              color: "#fff",
              background: "#2A3B7A",
              border: "1px solid #2A3B7A",
              fontFamily: "inherit",
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <rect x="5" y="11" width="14" height="10" rx="2" />
              <path d="M8 11V7a4 4 0 018 0v4" />
            </svg>
            锁定，进下一条
          </button>
        </div>
      </div>
    </div>
  );
}
