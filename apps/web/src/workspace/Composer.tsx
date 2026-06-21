import { useState, useRef } from "react";

type Props = {
  onSend: (text: string) => void;
  disabled?: boolean;
};

export function Composer({ onSend, disabled = false }: Props) {
  const [text, setText] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  function handleSend() {
    const trimmed = text.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setText("");
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey && !disabled) {
      e.preventDefault();
      handleSend();
    }
  }

  function handleInput(e: React.ChangeEvent<HTMLTextAreaElement>) {
    setText(e.target.value);
    // auto-grow
    const el = e.target;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 120)}px`;
  }

  return (
    <div style={{ flex: "none", padding: "0 36px 24px", background: "#F3F4F8" }}>
      <div style={{ maxWidth: "720px", margin: "0 auto" }}>
        <div
          style={{
            background: "#fff",
            border: "1px solid #E2E5EE",
            borderRadius: "16px",
            padding: "12px 14px 12px 18px",
            display: "flex",
            alignItems: "flex-end",
            gap: "12px",
            boxShadow: "0 2px 12px rgba(20,30,60,.05)",
          }}
        >
          <textarea
            ref={textareaRef}
            value={text}
            onChange={handleInput}
            onKeyDown={handleKeyDown}
            rows={1}
            placeholder="把你的想法发给陪练……"
            disabled={disabled}
            style={{
              flex: 1,
              border: "none",
              outline: "none",
              resize: "none",
              fontSize: "14.5px",
              lineHeight: "1.6",
              color: "#1C2333",
              background: "transparent",
              maxHeight: "120px",
              padding: "6px 0",
              fontFamily: "inherit",
              opacity: disabled ? 0.5 : 1,
            }}
          />
          <button
            type="button"
            onClick={handleSend}
            disabled={disabled}
            aria-label="发送"
            style={{
              flex: "none",
              width: "40px",
              height: "40px",
              borderRadius: "11px",
              background: disabled ? "#9AA1B0" : "#2A3B7A",
              border: "none",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: disabled ? "not-allowed" : "pointer",
              opacity: disabled ? 0.6 : 1,
            }}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="#fff"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
