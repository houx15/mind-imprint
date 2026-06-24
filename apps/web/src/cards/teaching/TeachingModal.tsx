import { useState } from "react";
import type { TeachingModule } from "./types";

interface TeachingModalProps {
  module: TeachingModule;
  onClose: () => void;
}

export function TeachingModal({ module, onClose }: TeachingModalProps) {
  const [idx, setIdx] = useState(0);
  const chapters = module.chapters;
  const isFirst = idx === 0;
  const isLast = idx === chapters.length - 1;
  const currentChapter = chapters[idx];
  if (!currentChapter) return null;
  const ChapterContent = currentChapter.Component;

  function handleNext() {
    if (isLast) {
      onClose();
    } else {
      setIdx(idx + 1);
    }
  }

  function handlePrev() {
    if (!isFirst) setIdx(idx - 1);
  }

  return (
    /* Scrim */
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(22,28,46,.34)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
      onClick={onClose}
    >
      {/* Modal — stop click propagation so clicking inside doesn't close */}
      <div
        style={{
          background: "#fff",
          borderRadius: 18,
          boxShadow: "0 24px 70px rgba(15,20,45,.4)",
          width: "100%",
          maxWidth: 430,
          overflow: "hidden",
          display: "flex",
          flexDirection: "column",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "13px 16px",
            borderBottom: "1px solid #EAECF2",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              fontSize: 13,
              fontWeight: 700,
              color: "#1C2333",
            }}
          >
            <span
              style={{
                fontSize: 10,
                fontWeight: 700,
                padding: "3px 9px",
                borderRadius: 999,
                color: "#4C9A82",
                background: "#E7F3EE",
              }}
            >
              {module.category}
            </span>
            {module.title}
          </div>
          <button
            aria-label="关闭"
            onClick={onClose}
            style={{
              width: 24,
              height: 24,
              borderRadius: 7,
              background: "#F3F4F8",
              color: "#9AA1B0",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 13,
              border: "none",
              cursor: "pointer",
            }}
          >
            ✕
          </button>
        </div>

        {/* Horizontal stepper */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            padding: "14px 18px 4px",
            justifyContent: "center",
          }}
        >
          {chapters.map((ch, i) => {
            const isDone = i < idx;
            const isOn = i === idx;
            return (
              <div
                key={ch.key}
                style={{ display: "flex", alignItems: "center", gap: 6 }}
              >
                {i > 0 && (
                  <span
                    style={{
                      width: 16,
                      height: 2,
                      background: "#EAECF2",
                      borderRadius: 2,
                      display: "inline-block",
                    }}
                  />
                )}
                <div
                  style={{ display: "flex", alignItems: "center", gap: 6 }}
                >
                  <span
                    style={{
                      width: 24,
                      height: 24,
                      borderRadius: 999,
                      fontSize: 11,
                      fontWeight: 800,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      background: isDone
                        ? "#E7F3EE"
                        : isOn
                        ? "#2A3B7A"
                        : "#F3F4F8",
                      color: isDone
                        ? "#4C9A82"
                        : isOn
                        ? "#fff"
                        : "#9AA1B0",
                    }}
                  >
                    {isDone ? "✓" : i + 1}
                  </span>
                  <span
                    style={{
                      fontSize: 10.5,
                      color: isOn ? "#2A3B7A" : "#9AA1B0",
                      fontWeight: isOn ? 700 : 400,
                    }}
                  >
                    {ch.label}
                  </span>
                </div>
              </div>
            );
          })}
        </div>

        {/* Stage — renders current chapter */}
        <div style={{ padding: "18px 22px 16px", flex: 1 }}>
          <ChapterContent />
        </div>

        {/* Footer navigation */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "12px 18px",
            borderTop: "1px solid #EAECF2",
          }}
        >
          <button
            onClick={handlePrev}
            disabled={isFirst}
            style={{
              fontSize: 11.5,
              fontWeight: 700,
              borderRadius: 8,
              padding: "7px 14px",
              border: "1px solid #EAECF2",
              background: "#fff",
              color: isFirst ? "#9AA1B0" : "#6B7384",
              cursor: isFirst ? "not-allowed" : "pointer",
              opacity: isFirst ? 0.5 : 1,
            }}
          >
            ← 上一步
          </button>

          {/* Dots */}
          <div style={{ display: "flex", gap: 5 }}>
            {chapters.map((ch, i) => (
              <span
                key={ch.key}
                style={{
                  width: i === idx ? 16 : 6,
                  height: 6,
                  borderRadius: 999,
                  background: i === idx ? "#2A3B7A" : "#D4D9E6",
                  transition: "width .2s",
                  display: "inline-block",
                }}
              />
            ))}
          </div>

          <button
            onClick={handleNext}
            style={{
              fontSize: 11.5,
              fontWeight: 700,
              borderRadius: 8,
              padding: "7px 14px",
              border: "none",
              background: "#2A3B7A",
              color: "#fff",
              cursor: "pointer",
            }}
          >
            {isLast ? "完成" : "下一步"}
          </button>
        </div>
      </div>
    </div>
  );
}
