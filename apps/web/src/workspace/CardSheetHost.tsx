import type { CardInstance, CardSpec } from "@mind-imprint/contracts";
import { useRef, useState } from "react";
import { pickCardBody } from "../cards/customRenderers";
import { envelopeReducer } from "../cards/envelopeReducer";
import { pickTeaching } from "../cards/teaching/teachingRegistry";
import { TeachingModal } from "../cards/teaching/TeachingModal";

type Props = {
  cardInstance: CardInstance;
  spec: CardSpec;
  onSubmit: (cardInstanceId: string, finalInstance: CardInstance) => void;
  onClose: (cardInstanceId: string) => void;
  onSkip: (cardInstanceId: string) => void;
};

export function CardSheetHost({ cardInstance, spec, onSubmit, onClose, onSkip }: Props) {
  const [env, setEnv] = useState<CardInstance>(cardInstance);
  const Body = pickCardBody(spec.id);
  const teaching = pickTeaching(spec.id);
  const [showTeaching, setShowTeaching] = useState(false);
  // The teaching entry records note_open at most once per sheet session (过程即数据),
  // mirroring MethodologyPanel's first-expand-only behavior. Re-opening the modal
  // must NOT inflate the note_open signal the evaluator reads.
  const notedTeaching = useRef(false);

  function handleField(path: string, value: unknown) {
    setEnv((e) => envelopeReducer(e, { type: "field_change", path, value }));
  }

  function handleExpandStep(step_key: string) {
    setEnv((e) => envelopeReducer(e, { type: "step_expand", step_key }));
  }

  // Consulting a step's methodology is recorded process data (过程即数据).
  function handleNote(step_key: string) {
    setEnv((e) => envelopeReducer(e, { type: "note_open", step_key }));
  }

  function handleSubmit() {
    const submitted = envelopeReducer(env, { type: "submit" });
    onSubmit(cardInstance.id, submitted);
  }

  function handleClose() {
    // Just close — does NOT record a skip (仅关闭，不跳过)
    onClose(cardInstance.id);
  }

  function handleSkip() {
    // Deliberate skip — records the signal (过程即数据)
    onSkip(cardInstance.id);
  }

  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        zIndex: 40,
        display: "flex",
        flexDirection: "column",
        justifyContent: "flex-end",
      }}
    >
      {/* Scrim */}
      <div
        onClick={handleClose}
        aria-hidden
        style={{
          position: "absolute",
          inset: 0,
          background: "rgba(22,28,46,.40)",
          animation: "mkScrim .22s ease",
        }}
      />

      {/* Sheet */}
      <div
        style={{
          position: "relative",
          margin: "0 auto",
          width: "100%",
          maxWidth: "880px",
          height: "80%",
          background: "#fff",
          borderRadius: "22px 22px 0 0",
          boxShadow: "0 -16px 50px rgba(20,30,60,.22)",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
          animation: "mkSheetUp .34s cubic-bezier(.22,.9,.3,1)",
        }}
      >
        {/* Top accent bar */}
        <div style={{ height: "4px", flex: "none", background: "#D98263" }} />

        {/* Header */}
        <div
          style={{
            flex: "none",
            padding: "18px 26px 16px",
            borderBottom: "1px solid #F0F1F5",
            display: "flex",
            alignItems: "flex-start",
            gap: "14px",
          }}
        >
          <div style={{ flex: 1, minWidth: 0 }}>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "9px",
                marginBottom: "6px",
              }}
            >
              <span
                style={{
                  fontSize: "11px",
                  fontWeight: 700,
                  color: "#D98263",
                  letterSpacing: ".06em",
                }}
              >
                现在轮到你想
              </span>
              <span
                style={{
                  fontSize: "11px",
                  fontWeight: 600,
                  color: "#2A3B7A",
                  background: "#EDEFF9",
                  padding: "2px 9px",
                  borderRadius: "999px",
                }}
              >
                {spec.category}
              </span>
            </div>
            <div style={{ fontSize: "18px", fontWeight: 700, color: "#1C2333" }}>
              {spec.name}
            </div>
            <div style={{ fontSize: "13px", color: "#9AA1B0", marginTop: "3px" }}>
              {spec.purpose}
            </div>
            {teaching && (
              <button
                type="button"
                onClick={() => {
                  if (!notedTeaching.current) { handleNote(spec.steps[0]!.key); notedTeaching.current = true; }
                  setShowTeaching(true);
                }}
                style={{
                  marginTop: "10px",
                  display: "inline-flex",
                  alignItems: "center",
                  gap: "7px",
                  background: "#EBF0FF",
                  color: "#2A3B7A",
                  border: "none",
                  borderRadius: "10px",
                  padding: "8px 14px",
                  fontSize: "13px",
                  fontWeight: 700,
                  cursor: "pointer",
                  fontFamily: "inherit",
                }}
              >
                给我讲讲这个
              </button>
            )}
          </div>

          <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
            <button
              type="button"
              aria-label="关闭"
              onClick={handleClose}
              style={{
                flex: "none",
                width: "34px",
                height: "34px",
                borderRadius: "9px",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                cursor: "pointer",
                color: "#9AA1B0",
                background: "none",
                border: "none",
              }}
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Scrollable body */}
        <div
          style={{
            flex: 1,
            minHeight: 0,
            overflowY: "auto",
            padding: "22px 26px 8px",
            background: "#FAFBFC",
          }}
        >
          <Body
            card={spec}
            values={env.field_values}
            onField={handleField}
            onExpandStep={handleExpandStep}
            onNote={handleNote}
            hideMethodology={!!teaching}
          />
        </div>

        {/* Footer */}
        <div
          style={{
            flex: "none",
            padding: "14px 26px",
            borderTop: "1px solid #F0F1F5",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            background: "#fff",
          }}
        >
          <button
            type="button"
            onClick={handleSkip}
            style={{
              background: "none",
              border: "none",
              color: "#C2557A",
              fontSize: "13px",
              fontWeight: 600,
              cursor: "pointer",
              padding: "10px 0",
              fontFamily: "inherit",
            }}
          >
            跳过这张卡
          </button>
          <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
            <button
              type="button"
              onClick={handleClose}
              style={{
                background: "none",
                border: "none",
                color: "#6B7384",
                fontSize: "14px",
                fontWeight: 600,
                cursor: "pointer",
                padding: "10px 14px",
                fontFamily: "inherit",
              }}
            >
              取消
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: "8px",
                background: "#2A3B7A",
                color: "#fff",
                border: "none",
                padding: "12px 22px",
                borderRadius: "11px",
                fontSize: "14px",
                fontWeight: 700,
                cursor: "pointer",
                fontFamily: "inherit",
              }}
            >
              提交并钉到过程树
              <svg
                width="15"
                height="15"
                viewBox="0 0 24 24"
                fill="none"
                stroke="#fff"
                strokeWidth="2.2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M5 12h14M13 6l6 6-6 6" />
              </svg>
            </button>
          </div>
        </div>
      </div>
      {showTeaching && teaching && <TeachingModal module={teaching} onClose={() => setShowTeaching(false)} />}
    </div>
  );
}
