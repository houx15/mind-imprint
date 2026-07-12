import { useState } from "react";
import type { Anchor, CardInstance, CardSpec, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

export type StudioAnnotateCardProps = {
  spec: CardSpec;
  anchors: Anchor[];
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

const RISK_NOTE_QUESTION = "这条来源在你的论证里起什么作用？有什么风险 / 局限？";

// Authorship-agnostic annotate answer host (design s3CoachCard, ~L1282-1320).
// It renders whatever anchors it is handed — chip = dimension, question,
// answer textarea, a checkmark once answered — and never assumes the AI
// authored them. Guidance levels L2/L3 (student finds spans / elicits the
// questions themselves) change only WHERE the anchors come from upstream;
// this renderer stays exactly the same either way. Do not hardcode the five
// CRAAP dimensions or specific question copy here.
export function StudioAnnotateCard({ spec, anchors, onSubmit, onSkip }: StudioAnnotateCardProps) {
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [riskNote, setRiskNote] = useState("");

  // Completable only once every anchor has a non-empty answer AND the
  // risk-note is non-empty — matches the design's per-dimension ✓ + lock
  // intent. Locking before that leaves the backend's card `active` with no
  // mint, and the frontend has no rehydration path for that state, so we
  // gate the button rather than let the student lose the card in-session.
  const allAnchorsAnswered = anchors.every((a) => !!(answers[a.id] ?? a.answer)?.trim());
  const canLock = allAnchorsAnswered && !!riskNote.trim();

  function handleAnswerChange(id: string, value: string) {
    setAnswers((prev) => ({ ...prev, [id]: value }));
  }

  function handleLock() {
    const filled: Anchor[] = anchors.map((a) => ({ ...a, answer: answers[a.id] ?? a.answer }));
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
    const env: CardInstance = { ...newEnvelope(spec.id, ""), anchors: filled };
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
              <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#2B3346", fontWeight: 500 }}>{a.question}</div>
              <textarea
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
