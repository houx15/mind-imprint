import { useState } from "react";
import { Bean } from "./Bean";
import { DispositionCard } from "./DispositionCard";
import { EquipmentBar } from "./EquipmentBar";
import type { CoachMessage, EquipCard, StationView, StudioCallbacks } from "./state";

export type CoachRailProps = {
  anchor: string;
  messages: CoachMessage[];
  equipment: EquipCard[];
  activeView: StationView;
  onDisposition: StudioCallbacks["onDisposition"];
  onOpenMethodology: (id: string) => void;
  onSend: (t: string) => void;
};

// Right-side AI 陪练 rail: header + thread + contextual tool-card slot +
// 装备栏 + composer. Design: docs/design/思维印记_工作区.dc.html ~L1230-1405
// (header ~1232-1243, thread ~1245-1280, tool-card slot ~1281-1343, 装备栏
// ~1344-1364, composer ~1366-1404).
//
// NOTE: MethodologyModal is rendered at the StudioShell level, not here —
// CoachRail is only 388px wide (a `position:relative` column), so a
// full-bleed `position:absolute; inset:0` modal rendered inside it gets
// clipped to the rail instead of covering the whole workspace (design
// intent: full-screen scrim, ~L1409). CoachRail only owns the 装备栏
// popover's open/closed state; opening a card just calls onOpenMethodology
// and lets the parent own the modal.
const FONT = "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif";

function ToolboxIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M14.7 6.3a4 4 0 00-5.4 5.4l-6.6 6.6a1.5 1.5 0 002.1 2.1l6.6-6.6a4 4 0 005.4-5.4l-2.1 2.1-2.1-2.1z" />
    </svg>
  );
}

function AttachIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21.44 11.05l-9.19 9.19a5 5 0 01-7.07-7.07l9.19-9.19a3 3 0 014.24 4.24l-9.2 9.19a1 1 0 01-1.41-1.41l8.49-8.49" />
    </svg>
  );
}

function MicIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="9" y="2" width="6" height="12" rx="3" />
      <path d="M5 10a7 7 0 0014 0M12 17v4" />
    </svg>
  );
}

function SendIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
    </svg>
  );
}

function FlagIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#C96F4F" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
      <path d="M12 9v4M12 17h.01" />
    </svg>
  );
}

function CraapPlaceholder() {
  return (
    <div style={{ border: "1px solid #F0DACF", borderRadius: 14, overflow: "hidden", boxShadow: "0 3px 14px rgba(217,130,99,.10)" }}>
      <div style={{ height: 4, background: "#D98263" }} />
      <div style={{ padding: "12px 15px 8px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#D98263" }}>工具卡 · CRAAP</span>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 8px", borderRadius: 999 }}>信息素养</span>
        </div>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>评估这条来源</div>
      </div>
      <div style={{ padding: "0 15px 14px", fontSize: 11.5, color: "#8A92A3", lineHeight: 1.6 }}>
        按 CRAAP 五维核对这条来源（互动卡片见「素材」环节）。
      </div>
    </div>
  );
}

export function CoachRail({
  anchor,
  messages,
  equipment,
  activeView,
  onDisposition,
  onOpenMethodology,
  onSend,
}: CoachRailProps) {
  const [equipOpen, setEquipOpen] = useState(false);
  const [composerText, setComposerText] = useState("");

  const lastAi = [...messages].reverse().find((m) => m.kind === "ai");

  function handleSend() {
    const text = composerText.trim();
    if (!text) return;
    onSend(text);
    setComposerText("");
  }

  return (
    <div style={{ width: 388, flex: "none", background: "#fff", borderLeft: "1px solid #EAECF2", display: "flex", flexDirection: "column", fontFamily: FONT, position: "relative" }}>
      <style>{`@keyframes coachRailPulse { 0%,100% { opacity: 1; } 50% { opacity: .35; } }`}</style>

      {/* header */}
      <div style={{ flex: "none", padding: "15px 18px 13px", borderBottom: "1px solid #EFF0F5" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <Bean size={30} />
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 14.5, fontWeight: 700, color: "#1C2333" }}>AI 陪练</div>
          </div>
        </div>
        <div
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 6,
            marginTop: 11,
            fontSize: 11.5,
            fontWeight: 600,
            color: "#2A3B7A",
            background: "#EDEFF9",
            padding: "5px 11px",
            borderRadius: 999,
          }}
        >
          <span style={{ width: 6, height: 6, borderRadius: "50%", background: "#4C9A82", animation: "coachRailPulse 1.6s infinite" }} />
          正在看：{anchor}
        </div>
      </div>

      {/* thread + contextual tool-card slot */}
      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "18px 18px 12px" }}>
        {messages.map((m, i) => (
          <div key={i} style={{ marginBottom: 15 }}>
            {m.kind === "student" && (
              <div style={{ display: "flex", justifyContent: "flex-end" }}>
                <div style={{ maxWidth: 280, background: "#2A3B7A", color: "#fff", padding: "10px 14px", borderRadius: "14px 14px 4px 14px", fontSize: 13.5, lineHeight: 1.6 }}>
                  {m.body}
                </div>
              </div>
            )}
            {m.kind === "ai" && (
              <div style={{ display: "flex", gap: 9, alignItems: "flex-start" }}>
                <Bean size={27} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  {m.tag && (
                    <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 5 }}>
                      <span
                        style={{
                          fontSize: 10.5,
                          fontWeight: 700,
                          color: "#fff",
                          background: "#2A3B7A",
                          padding: "2px 9px",
                          borderRadius: 999,
                        }}
                      >
                        {m.tag}
                      </span>
                    </div>
                  )}
                  <div style={{ background: "#F7F8FB", border: "1px solid #EEF0F5", padding: "10px 14px", borderRadius: "4px 14px 14px 14px", fontSize: 13.5, lineHeight: 1.7, color: "#2B3346" }}>
                    {m.body}
                  </div>
                </div>
              </div>
            )}
            {m.kind === "flag" && (
              <div style={{ display: "flex", gap: 9, alignItems: "flex-start" }}>
                <Bean size={27} />
                <div style={{ flex: 1, minWidth: 0, background: "#fff", border: "1px solid #F1D6C8", borderLeft: "3px solid #D98263", borderRadius: "4px 12px 12px 12px", padding: "11px 14px" }}>
                  <div style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 10.5, fontWeight: 700, color: "#C96F4F", marginBottom: 6 }}>
                    <FlagIcon />
                    {m.label}
                  </div>
                  <div style={{ fontSize: 13.5, lineHeight: 1.7, color: "#2B3346" }}>{m.body}</div>
                </div>
              </div>
            )}
          </div>
        ))}

        {activeView === "素材" ? (
          <CraapPlaceholder />
        ) : (
          <DispositionCard
            tag={lastAi?.tag ?? "AI 建议"}
            anchor={anchor}
            body={lastAi?.body ?? ""}
            onDisposition={onDisposition}
          />
        )}
      </div>

      {/* 装备栏 */}
      <EquipmentBar cards={equipment} open={equipOpen} onToggle={() => setEquipOpen((o) => !o)} onOpen={onOpenMethodology} />

      {/* composer */}
      <div style={{ flex: "none", padding: "11px 16px 15px", borderTop: "1px solid #EFF0F5" }}>
        <div style={{ background: "#F7F8FB", border: "1px solid #E2E5EE", borderRadius: 13, padding: "8px 8px 8px 10px", display: "flex", alignItems: "flex-end", gap: 6 }}>
          <div
            onClick={() => setEquipOpen((o) => !o)}
            title="装备栏 · 工具卡"
            role="button"
            style={{
              flex: "none",
              width: 32,
              height: 32,
              borderRadius: 9,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              color: equipOpen ? "#2A3B7A" : "#6B7384",
              background: equipOpen ? "#EDEFF9" : "transparent",
            }}
          >
            <ToolboxIcon />
          </div>
          <div
            title="上传文件"
            role="button"
            style={{ flex: "none", width: 32, height: 32, borderRadius: 9, display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", color: "#6B7384" }}
          >
            <AttachIcon />
          </div>
          <textarea
            value={composerText}
            onChange={(e) => setComposerText(e.target.value)}
            rows={1}
            placeholder="把你的想法发给印记……"
            style={{ flex: 1, border: "none", outline: "none", resize: "none", fontSize: 14, lineHeight: 1.6, color: "#1C2333", background: "transparent", maxHeight: 100, padding: "6px 0", fontFamily: "inherit" }}
          />
          <div
            title="语音输入"
            role="button"
            style={{ flex: "none", width: 32, height: 32, borderRadius: 9, display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", color: "#6B7384" }}
          >
            <MicIcon />
          </div>
          <button
            type="button"
            aria-label="发送"
            onClick={handleSend}
            style={{ flex: "none", width: 36, height: 36, borderRadius: 10, background: "#2A3B7A", border: "none", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" }}
          >
            <SendIcon />
          </button>
        </div>
      </div>
    </div>
  );
}
