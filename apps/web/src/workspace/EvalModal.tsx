import type { Evaluation } from "@mind-imprint/contracts";
import { DEMO_RUBRIC } from "@mind-imprint/contracts";
import { evalView } from "./evalView";

type Props = {
  evaluation: Evaluation;
  onClose: () => void;
};

export function EvalModal({ evaluation, onClose }: Props) {
  const { dims } = evalView(evaluation, DEMO_RUBRIC);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="你的思维印记"
      style={{
        position: "absolute",
        inset: 0,
        zIndex: 50,
        background: "rgba(22,28,46,.42)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "32px",
        animation: "mkScrim .22s ease",
      }}
    >
      <div
        style={{
          width: "100%",
          maxWidth: "600px",
          maxHeight: "90%",
          overflowY: "auto",
          background: "#fff",
          borderRadius: "20px",
          boxShadow: "0 24px 64px rgba(20,30,60,.28)",
          animation: "mkPop .3s cubic-bezier(.22,.9,.3,1)",
        }}
      >
        {/* Header — deep-blue gradient */}
        <div
          style={{
            padding: "26px 30px 20px",
            background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)",
            borderRadius: "20px 20px 0 0",
            position: "relative",
          }}
        >
          {/* Close X */}
          <button
            type="button"
            aria-label="关闭"
            onClick={onClose}
            style={{
              position: "absolute",
              top: "18px",
              right: "18px",
              width: "32px",
              height: "32px",
              borderRadius: "8px",
              background: "rgba(255,255,255,.14)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              color: "#fff",
              border: "none",
            }}
          >
            <svg
              width="16"
              height="16"
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

          <div
            style={{
              fontSize: "11px",
              fontWeight: 700,
              letterSpacing: ".1em",
              color: "#AEB8E4",
              marginBottom: "8px",
            }}
          >
            过程小结 · 仅你可见
          </div>
          <div style={{ fontSize: "24px", fontWeight: 800, color: "#fff" }}>你的思维印记</div>
          <div
            style={{
              fontSize: "13px",
              color: "#C3CBEC",
              marginTop: "6px",
              lineHeight: "1.6",
            }}
          >
            基于完整对话 + 标准信封，按 SOLO 四级评级 · L1 单点 · L2 多点 · L3 关联 · L4 拓展
          </div>
        </div>

        {/* Body */}
        <div style={{ padding: "24px 30px" }}>
          {/* Dim rows */}
          {dims.map((d, i) => (
            <div
              key={d.dim}
              style={{
                padding: "13px 0",
                borderBottom: i < dims.length - 1 ? "1px solid #F2F3F7" : "none",
              }}
            >
              {/* Dim name + levelLabel pill */}
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: "12px",
                  marginBottom: "8px",
                }}
              >
                <span style={{ fontSize: "14px", fontWeight: 700, color: "#1C2333" }}>
                  {d.dim}
                </span>
                <span
                  style={{
                    fontSize: "12.5px",
                    fontWeight: 700,
                    color: "#D98263",
                    background: "#FBEEE7",
                    padding: "2px 10px",
                    borderRadius: "999px",
                    flexShrink: 0,
                  }}
                >
                  {d.levelLabel}
                </span>
              </div>

              {/* Seg bars */}
              <div style={{ display: "flex", gap: "5px", marginBottom: "7px" }}>
                {d.segs.map((seg, idx) => (
                  <span
                    key={idx}
                    aria-hidden="true"
                    style={{
                      flex: "1",
                      height: "6px",
                      borderRadius: "3px",
                      background: seg.filled ? "#D98263" : "#ECEEF4",
                    }}
                  />
                ))}
              </div>

              {/* Note */}
              <div style={{ fontSize: "12.5px", color: "#8A92A3" }}>{d.note}</div>
            </div>
          ))}

          {/* Narrative card */}
          <div
            style={{
              marginTop: "20px",
              background: "#F7F8FB",
              border: "1px solid #EDEEF4",
              borderRadius: "14px",
              padding: "17px 19px",
            }}
          >
            <div
              style={{
                fontSize: "12px",
                fontWeight: 700,
                color: "#2A3B7A",
                letterSpacing: ".04em",
                marginBottom: "9px",
              }}
            >
              过程叙述
            </div>
            <div style={{ fontSize: "14px", lineHeight: "1.78", color: "#3A4256" }}>
              {evaluation.narrative}
            </div>
          </div>

          {/* 回到任务 button */}
          <button
            type="button"
            onClick={onClose}
            style={{
              width: "100%",
              marginTop: "18px",
              background: "#2A3B7A",
              color: "#fff",
              border: "none",
              padding: "13px",
              borderRadius: "12px",
              fontSize: "14.5px",
              fontWeight: 700,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            回到任务
          </button>
        </div>
      </div>
    </div>
  );
}
