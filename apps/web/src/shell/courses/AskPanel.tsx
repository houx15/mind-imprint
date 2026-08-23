import { useCallback, useEffect, useRef, useState } from "react";
import { AsrStream } from "../../api/voice";
import { MicCapture } from "../../audio/capture";
import { Bean } from "../../studio/Bean";

// The 问印记 ask panel — binding design docs/design/思维印记_工作区.dc.html
// lines 349-397. Copy is verbatim: 问印记 · 随时打断我，问任何问题 ·
// 正在看： · 你可能想问 · 输入你的问题…… · 按住说话，问老师.
//
// Task 10: this is now a free helper — no card offers, no phase runtime. The
// course player is linear (SegmentTimeline pages through the render cache);
// the coach here only ever answers questions, it never proposes a tool card.

// v1: push-to-talk (ASR) is disabled in the course view — students already have
// plenty of speech-to-text tools of their own, so we don't ship our own mic
// here yet. The AsrStream/MicCapture wiring below is kept intact and referenced
// so re-enabling for a later version is a one-line flip.
const VOICE_INPUT_ENABLED = false;

export type AskMessage = {
  id: string;
  role: "student" | "assistant";
  text: string;
};

export type AskPanelProps = {
  expanded: boolean;
  onToggle: () => void;
  branchColor: string;
  context: string;
  chips: string[];
  messages: AskMessage[];
  pending: boolean;
  onSend: (text: string) => void;
};

function ChevronRightIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 18l6-6-6-6" />
    </svg>
  );
}

function ChevronLeftIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--mk-secondary)" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" />
    </svg>
  );
}

function SendIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--mk-surface)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
    </svg>
  );
}

function MicIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="var(--mk-peach)" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 2a3 3 0 013 3v6a3 3 0 01-6 0V5a3 3 0 013-3z" />
      <path d="M19 10v1a7 7 0 01-14 0v-1M12 18v4" />
    </svg>
  );
}

export function AskPanel({
  expanded,
  onToggle,
  branchColor,
  context,
  chips,
  messages,
  pending,
  onSend,
}: AskPanelProps) {
  const [text, setText] = useState("");
  const [recording, setRecording] = useState(false);
  const [voiceError, setVoiceError] = useState<string | null>(null);
  const micRef = useRef<MicCapture | null>(null);
  const asrRef = useRef<AsrStream | null>(null);

  function handleSend() {
    const value = text.trim();
    if (!value || pending) return;
    onSend(value);
    setText("");
  }

  function handleChip(value: string) {
    if (pending) return;
    onSend(value);
  }

  // Stable across renders (reads only refs + the setState setter, both
  // stable) so the unmount-cleanup effect below always tears down whatever
  // mic/ASR connection is live. Safe to call more than once: once torn down,
  // the refs are null and further calls are no-ops. Mirrors CoachRail's
  // stopRecording exactly.
  const stopRecording = useCallback(() => {
    micRef.current?.stop();
    asrRef.current?.stop();
    micRef.current = null;
    asrRef.current = null;
    setRecording(false);
  }, []);

  // If the panel unmounts mid-hold (e.g. the student navigates away),
  // release the mic + ASR socket instead of leaving them running.
  useEffect(() => {
    return () => {
      stopRecording();
    };
  }, [stopRecording]);

  // Push-to-talk: hold to transcribe into the ask input, release to stop.
  // The transcript only ever fills `text` — never auto-sent (克制/铁律 2):
  // the student reviews it and sends via the existing handleSend button,
  // exactly like a mis-heard word typed by hand.
  async function startRecording() {
    if (pending || recording) return;
    setVoiceError(null);
    setRecording(true);
    try {
      const asr = new AsrStream();
      asr.onPartial((t) => setText(t));
      asr.onFinal((t) => setText(t));
      asr.onError((message) => {
        setVoiceError(message);
        stopRecording();
      });
      asrRef.current = asr;

      const mic = new MicCapture();
      micRef.current = mic;
      await mic.start((pcm) => asrRef.current?.sendPCM(pcm));
    } catch (err) {
      setVoiceError(err instanceof Error ? err.message : "无法访问麦克风");
      stopRecording();
    }
  }

  if (!expanded) {
    return (
      <div
        onClick={onToggle}
        role="button"
        aria-label="展开问印记"
        style={{ width: 46, flex: "none", background: "var(--mk-surface)", borderLeft: "1px solid var(--mk-border)", display: "flex", flexDirection: "column", alignItems: "center", paddingTop: 16, gap: 14, cursor: "pointer" }}
      >
        <ChevronLeftIcon />
        <Bean color={branchColor} size={26} />
        <span style={{ writingMode: "vertical-rl", fontSize: 12.5, fontWeight: 700, color: "var(--mk-secondary)", letterSpacing: ".08em" }}>问印记</span>
      </div>
    );
  }

  return (
    <div style={{ width: 330, flex: "none", background: "var(--mk-surface)", borderLeft: "1px solid var(--mk-border)", display: "flex", flexDirection: "column" }}>
      <style>{`@keyframes mkPulse { 0%,100% { opacity:.5;} 50% { opacity:1;} }`}</style>
      <div style={{ flex: "none", padding: "16px 18px 14px", borderBottom: "1px solid var(--mk-border)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <Bean color={branchColor} size={30} />
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 14.5, fontWeight: 700, color: "var(--mk-ink)" }}>问印记</div>
            <div style={{ fontSize: 11.5, color: "var(--mk-muted)", fontWeight: 500 }}>随时打断我，问任何问题</div>
          </div>
          <div
            onClick={onToggle}
            role="button"
            aria-label="收起问印记"
            style={{ flex: "none", width: 30, height: 30, borderRadius: 8, display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", color: "var(--mk-muted)" }}
          >
            <ChevronRightIcon />
          </div>
        </div>
        <div style={{ display: "inline-flex", alignItems: "center", gap: 6, marginTop: 12, fontSize: 11.5, fontWeight: 600, color: "var(--mk-accent-500)", background: "var(--mk-accent-50)", padding: "5px 11px", borderRadius: 999 }}>
          <span style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--mk-success)", animation: "mkPulse 1.6s infinite" }} />
          正在看：{context}
        </div>
      </div>

      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "18px 20px" }}>
        {messages.length > 0 && (
          <div style={{ display: "flex", flexDirection: "column", gap: 12, marginBottom: 18 }}>
            {messages.map((m) => (
              <div key={m.id} style={{ display: "flex", flexDirection: "column", alignItems: m.role === "assistant" ? "flex-start" : "flex-end" }}>
                {/* Guard against an empty bubble on a frame with no reply
                    text yet (e.g. the assistant message is still streaming
                    its first chunk). */}
                {m.text !== "" && (
                  <div
                    data-testid="ask-bubble"
                    style={
                      m.role === "assistant"
                        ? { background: "var(--mk-surface)", border: "1px solid var(--mk-border)", borderRadius: "4px 12px 12px 12px", padding: "10px 13px", fontSize: 13, lineHeight: 1.65, color: "var(--mk-ink)", maxWidth: "92%" }
                        : { background: "var(--mk-accent-500)", color: "var(--mk-surface)", borderRadius: "12px 12px 4px 12px", padding: "10px 13px", fontSize: 13, lineHeight: 1.55, maxWidth: "92%" }
                    }
                  >
                    {m.text}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
        <div style={{ fontSize: 12, color: "var(--mk-muted)", fontWeight: 600, marginBottom: 11 }}>你可能想问</div>
        {chips.map((c, i) => (
          <div
            key={i}
            onClick={() => handleChip(c)}
            style={{ border: "1px solid var(--mk-border)", borderRadius: 12, padding: "11px 14px", marginBottom: 9, fontSize: 13.5, color: "var(--mk-ink)", cursor: "pointer", lineHeight: 1.5 }}
          >
            {c}
          </div>
        ))}
      </div>

      <div style={{ flex: "none", padding: "14px 18px 18px", borderTop: "1px solid var(--mk-border)" }}>
        {/* Guided tour anchor (§P5 Task 7): the persistent ask box — present from
            mount regardless of message history, unlike `[data-testid="ask-bubble"]`
            which only exists once a reply has streamed in. Behavior-neutral. */}
        <div data-tour="courses-ask-box" style={{ background: "var(--mk-paper)", border: "1px solid var(--mk-input-border)", borderRadius: 14, padding: "8px 8px 8px 14px", display: "flex", alignItems: "center", gap: 10 }}>
          <input
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="输入你的问题……"
            disabled={pending}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSend();
              }
            }}
            style={{ flex: 1, minWidth: 0, border: "none", outline: "none", background: "transparent", fontSize: 14, color: "var(--mk-ink)" }}
          />
          <button
            type="button"
            aria-label="发送"
            onClick={handleSend}
            disabled={pending || text.trim().length === 0}
            style={{
              flex: "none",
              width: 36,
              height: 36,
              borderRadius: 10,
              background: pending || text.trim().length === 0 ? "var(--mk-faint)" : "var(--mk-accent-500)",
              border: "none",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: pending || text.trim().length === 0 ? "not-allowed" : "pointer",
            }}
          >
            <SendIcon />
          </button>
        </div>
        {VOICE_INPUT_ENABLED && voiceError && !recording && (
          <div
            role="alert"
            onClick={() => setVoiceError(null)}
            title="点击关闭"
            style={{ fontSize: 12, fontWeight: 600, color: "var(--mk-danger)", background: "var(--mk-danger-bg)", border: "1px solid var(--mk-danger-bg)", borderRadius: 10, padding: "8px 12px", marginTop: 10, cursor: "pointer" }}
          >
            {voiceError}
          </div>
        )}
        {/* Push-to-talk (reuses the AsrStream/MicCapture pattern from
            CoachRail): hold to transcribe, release to stop. The transcript
            only ever fills the input above — the student sends it herself.
            Disabled in v1 (see VOICE_INPUT_ENABLED above). */}
        {VOICE_INPUT_ENABLED && (
          <button
            type="button"
            onMouseDown={startRecording}
            onMouseUp={stopRecording}
            onMouseLeave={stopRecording}
            onTouchStart={startRecording}
            onTouchEnd={stopRecording}
            disabled={pending}
            aria-pressed={recording}
            style={{
              width: "100%",
              marginTop: 10,
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              gap: 9,
              background: recording ? "var(--mk-peach-bg)" : "var(--mk-surface)",
              border: "1.5px solid var(--mk-peach)",
              color: "var(--mk-peach)",
              fontSize: 14,
              fontWeight: 700,
              padding: 11,
              borderRadius: 12,
              cursor: pending ? "not-allowed" : "pointer",
              opacity: pending ? 0.5 : 1,
              fontFamily: "inherit",
            }}
          >
            <MicIcon />
            按住说话，问老师
          </button>
        )}
      </div>
    </div>
  );
}
