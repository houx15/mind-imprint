import type { CardInstance, CardSpec } from "@mind-imprint/contracts";
import { useState } from "react";
import { CardRenderer } from "../cards/CardRenderer";
import { envelopeReducer } from "../cards/envelopeReducer";

type Props = {
  cardInstance: CardInstance;
  spec: CardSpec;
  onSubmit: (cardInstanceId: string, finalInstance: CardInstance) => void;
  onClose: (cardInstanceId: string) => void;
};

export function CardSheetHost({ cardInstance, spec, onSubmit, onClose }: Props) {
  const [env, setEnv] = useState<CardInstance>(cardInstance);

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
    // Close as skip — call parent's onClose which triggers skipCard
    onClose(cardInstance.id);
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
          <CardRenderer
            card={spec}
            values={env.field_values}
            onField={handleField}
            onExpandStep={handleExpandStep}
            onNote={handleNote}
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
          <div style={{ fontSize: "12px", color: "#AEB4C2" }}>
            填写过程会被采集，提交后序列化为标准信封
          </div>
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
    </div>
  );
}
