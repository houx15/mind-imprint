import { useState } from "react";
import type { CardInstance, CardSpec, TraceEvent } from "@mind-imprint/contracts";
import { pickCardBody } from "../cards/customRenderers";
import { envelopeReducer, newEnvelope } from "../cards/envelopeReducer";

export type StudioCardSheetProps = {
  spec: CardSpec;
  onSubmit: (finalEnvelope: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

// Lean coach-rail-fit card host: reuses the SAME schema-driven renderer +
// reducer as the retired task-workspace card host used to, but laid out for
// the 388px coach-rail column instead of a full-bleed bottom-sheet. No
// card-specific branching here — pickCardBody(spec.id) resolves any custom
// renderer, CardRenderer otherwise (schema-driven).
export function StudioCardSheet({ spec, onSubmit, onSkip }: StudioCardSheetProps) {
  // task_id is a vestigial local artifact — the project submit endpoint
  // ignores it — so an empty string is fine here.
  const [env, setEnv] = useState<CardInstance>(() => newEnvelope(spec.id, ""));
  const Body = pickCardBody(spec.id);

  function handleField(path: string, value: unknown) {
    setEnv((e) => envelopeReducer(e, { type: "field_change", path, value }));
  }

  function handleExpandStep(step_key: string) {
    setEnv((e) => envelopeReducer(e, { type: "step_expand", step_key }));
  }

  function handleNote(step_key: string) {
    setEnv((e) => envelopeReducer(e, { type: "note_open", step_key }));
  }

  function handleSubmit() {
    const finalEnvelope = envelopeReducer(env, { type: "submit" });
    onSubmit(finalEnvelope);
  }

  function handleSkip() {
    onSkip(env.event_trace);
  }

  return (
    <div style={{ border: "1px solid #F0DACF", borderRadius: 14, overflow: "hidden", boxShadow: "0 3px 14px rgba(217,130,99,.10)" }}>
      <div style={{ height: 4, background: "#D98263" }} />
      <div style={{ padding: "12px 15px 8px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#D98263" }}>工具卡</span>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 8px", borderRadius: 999 }}>
            {spec.category}
          </span>
        </div>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>{spec.name}</div>
        <div style={{ fontSize: 11.5, color: "#8A92A3", lineHeight: 1.6, marginTop: 3 }}>{spec.purpose}</div>
      </div>

      <div style={{ padding: "4px 15px 12px", maxHeight: 360, overflowY: "auto" }}>
        <Body card={spec} values={env.field_values} onField={handleField} onExpandStep={handleExpandStep} onNote={handleNote} />
      </div>

      <div style={{ padding: "10px 15px 14px", borderTop: "1px solid #F3ECE6", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <button
          type="button"
          onClick={handleSkip}
          style={{ background: "none", border: "none", color: "#C2557A", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: "6px 0", fontFamily: "inherit" }}
        >
          跳过这张卡
        </button>
        <button
          type="button"
          onClick={handleSubmit}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 6,
            background: "#2A3B7A",
            color: "#fff",
            border: "none",
            padding: "9px 16px",
            borderRadius: 10,
            fontSize: 12.5,
            fontWeight: 700,
            cursor: "pointer",
            fontFamily: "inherit",
          }}
        >
          提交并钉到过程树
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M5 12h14M13 6l6 6-6 6" />
          </svg>
        </button>
      </div>
    </div>
  );
}
