import { useState } from "react";
import { synthesize } from "../../api/voice";
import { player } from "../../audio/player";

export type TeachingContent = { title: string; subtitle: string; body: string[]; foreground_asset_id: string | null };

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";
// Speaker glyph: body + two sound-wave arcs, matches the file's inline stroke-svg convention.
const SPEAKER_BODY = "M4 9h4l5-4v14l-5-4H4z";
const SPEAKER_WAVE_1 = "M15.5 9.5a3.5 3.5 0 0 1 0 5";
const SPEAKER_WAVE_2 = "M17.5 7.2a6.5 6.5 0 0 1 0 9.6";

type NarrationState = "idle" | "loading" | "playing";

export function TeachingTemplate({ content }: { content: TeachingContent }) {
  const [narration, setNarration] = useState<NarrationState>("idle");

  async function handleNarrate() {
    if (narration === "loading") return;
    setNarration("loading");
    try {
      const text = [content.title, content.subtitle, ...content.body].join("\n");
      const url = await synthesize(text);
      await player.play(url, () => setNarration("idle"));
      setNarration("playing");
    } catch (err) {
      console.warn("narration failed", err);
      setNarration("idle");
    }
  }

  return (
    <div style={{ maxWidth: 700, margin: "0 auto", width: "100%" }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
        <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-0.01em", lineHeight: 1.3 }}>{content.title}</div>
        <button
          type="button"
          onClick={handleNarrate}
          disabled={narration === "loading"}
          aria-label="朗读本节"
          style={{
            flexShrink: 0, marginTop: 4, display: "inline-flex", alignItems: "center", gap: 6,
            fontSize: 12.5, fontWeight: 700, color: narration === "playing" ? "#fff" : "#2A3B7A",
            background: narration === "playing" ? "#2A3B7A" : "#EDEFF9",
            border: "1px solid #DEE1F0", borderRadius: 999, padding: "6px 12px",
            cursor: narration === "loading" ? "default" : "pointer", opacity: narration === "loading" ? 0.7 : 1,
          }}
        >
          {narration === "loading" ? (
            <span style={{ fontSize: 12 }}>…</span>
          ) : (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke={narration === "playing" ? "#fff" : "#2A3B7A"} strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
              <path d={SPEAKER_BODY} />
              <path d={SPEAKER_WAVE_1} />
              <path d={SPEAKER_WAVE_2} />
            </svg>
          )}
          朗读本节
        </button>
      </div>
      <div style={{ marginTop: 16, height: 148, borderRadius: 16, background: "linear-gradient(135deg,#EDEFF9 0%,#F3F0EC 100%)", border: "1px solid #EAECF2", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 11 }}>
        <div style={{ width: 54, height: 54, borderRadius: 16, background: "#fff", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "0 6px 16px rgba(42,59,122,.12)" }}>
          <svg width="27" height="27" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round"><path d={STAR} /></svg>
        </div>
        <div style={{ fontSize: 14, color: "#5B6373", fontWeight: 600, maxWidth: 520, textAlign: "center", padding: "0 20px" }}>{content.subtitle}</div>
      </div>
      <div style={{ marginTop: 20 }}>
        {content.body.map((p, i) => (
          <div key={i} style={{ fontSize: 15.5, lineHeight: 1.85, color: "#2B3346", marginBottom: 15 }}>{p}</div>
        ))}
      </div>
    </div>
  );
}
