import { useState, useRef, useCallback, useEffect } from "react";
import { AsrStream } from "../api/voice";
import { MicCapture } from "../audio/capture";

type Props = {
  onSend: (text: string, source?: "voice") => void;
  disabled?: boolean;
};

export function Composer({ onSend, disabled = false }: Props) {
  const [text, setText] = useState("");
  const [recording, setRecording] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const micRef = useRef<MicCapture | null>(null);
  const asrRef = useRef<AsrStream | null>(null);
  // True once a voice transcript (partial or final) has filled the textarea, until the
  // student edits it by hand or sends — i.e. "was this turn's text produced by speaking?"
  const wasVoiceRef = useRef(false);

  function handleSend() {
    const trimmed = text.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed, wasVoiceRef.current ? "voice" : undefined);
    wasVoiceRef.current = false;
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
    wasVoiceRef.current = false; // manual edit reverts this turn to typed
    // auto-grow
    const el = e.target;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 120)}px`;
  }

  // Stable across renders (reads only refs + the setState setter, both of
  // which are stable), so the unmount-cleanup effect below always tears
  // down whatever the current mic/ASR connection is. Safe to call more than
  // once: once torn down, the refs are null and further calls are no-ops.
  const stopRecording = useCallback(() => {
    micRef.current?.stop();
    asrRef.current?.stop();
    micRef.current = null;
    asrRef.current = null;
    setRecording(false);
  }, []);

  // If the component unmounts mid-recording, release the mic + ASR socket
  // instead of leaving them running with nothing left to stop them.
  useEffect(() => {
    return () => {
      stopRecording();
    };
  }, [stopRecording]);

  async function handleMicDown() {
    if (disabled || recording) return;
    setRecording(true);
    try {
      const asr = new AsrStream();
      asr.onPartial((t) => { wasVoiceRef.current = true; setText(t); });
      asr.onFinal((t) => { wasVoiceRef.current = true; setText(t); });
      asr.onError((message) => {
        console.warn("ASR error:", message);
        stopRecording();
      });
      asrRef.current = asr;

      const mic = new MicCapture();
      micRef.current = mic;
      await mic.start((pcm) => asrRef.current?.sendPCM(pcm));
    } catch (err) {
      console.warn("Mic capture failed:", err);
      stopRecording();
    }
  }

  function handleMicUp() {
    if (!recording) return;
    stopRecording();
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
            onPointerDown={handleMicDown}
            onPointerUp={handleMicUp}
            onPointerLeave={handleMicUp}
            disabled={disabled}
            aria-label={recording ? "正在录音，松开结束" : "按住说话"}
            aria-pressed={recording}
            style={{
              flex: "none",
              width: "40px",
              height: "40px",
              borderRadius: "11px",
              background: recording ? "#D6455D" : "#EEF0F6",
              border: "none",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: disabled ? "not-allowed" : "pointer",
              opacity: disabled ? 0.6 : 1,
              transition: "background .15s ease",
            }}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke={recording ? "#fff" : "#5A6178"}
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z" />
              <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
              <line x1="12" y1="19" x2="12" y2="23" />
              <line x1="8" y1="23" x2="16" y2="23" />
            </svg>
          </button>
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
