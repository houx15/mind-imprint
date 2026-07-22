import { useCallback, useEffect, useRef, useState } from "react";
import type { Anchor, CardInstance, CardSpec, MaterialSource, TraceEvent } from "@mind-imprint/contracts";
import { AsrStream } from "../api/voice";
import { MicCapture } from "../audio/capture";
import { Bean } from "./Bean";
import { DispositionCard } from "./DispositionCard";
import { EquipmentBar } from "./EquipmentBar";
import { StudioAnnotateCard, type LocatedSpan } from "./StudioAnnotateCard";
import { StudioCompareCard } from "./StudioCompareCard";
import { StudioSortCard } from "./StudioSortCard";
import { StudioScaleCard } from "./StudioScaleCard";
import { StudioMatrixCard } from "./StudioMatrixCard";
import { StudioCardSheet } from "./StudioCardSheet";
import type { CoachMessage, EquipCard, StationView, StudioCallbacks } from "./state";

export type LiveCard = {
  cardInstanceId: string;
  cardId: string;
  spec: CardSpec;
  status: "proposed" | "active";
  anchors: Anchor[];
  // The material this card is ABOUT (server's card_instance--evaluates-->
  // material edge target, whole-branch review finding [5]). Optional/plumbing
  // only here — consuming it to replace a client-side guess (e.g.
  // StudioCompareCard's materials[0] default) is a separate follow-up.
  materialId?: string;
};

export type CoachRailProps = {
  anchor: string;
  messages: CoachMessage[];
  equipment: EquipCard[];
  activeView: StationView;
  onDisposition: StudioCallbacks["onDisposition"];
  onOpenMethodology: (id: string) => void;
  onSend: (t: string) => void;
  sending?: boolean;
  // Live tool-card slot (Task 9): when set, this replaces the disposition/
  // placeholder card with either a proposal bubble (status "proposed") or
  // the schema-driven card sheet (status "active").
  card?: LiveCard | null;
  onOpenCard?: (cardInstanceId: string) => void;
  onSubmitCard?: (finalEnvelope: CardInstance) => void;
  onSkipCard?: (eventTrace: TraceEvent[]) => void;
  // Task 11 (SIFT): the project's material list, so StudioCompareCard can
  // offer lateral-source candidates (everything except the card's own
  // materialId).
  materials?: MaterialSource[];
  // The student's in-progress lateral-source pick for the active compare
  // card, LIFTED (whole-branch review finding [3]) so the center pane's
  // Compare primitive can render the same choice she just made here — see
  // StudioContainer, which owns this state.
  lateralMaterialId?: string;
  onLateralMaterialChange?: (materialId: string) => void;
  // N3c task 9 (spec §8): threaded straight through to StudioAnnotateCard —
  // see that file's own prop doc for the dead-control convention these four
  // go together under, and StudioContainer for where they're sourced.
  locatedSpans?: Record<string, LocatedSpan>;
  onRequestLocate?: (anchorId: string, dimension: string) => void;
  onSpanNotFound?: (anchorId: string, dimension: string) => void;
  pendingTrace?: TraceEvent[];
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

function CardProposalBubble({ spec, onOpen }: { spec: CardSpec; onOpen: () => void }) {
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
      <div style={{ padding: "0 15px 14px" }}>
        <button
          type="button"
          onClick={onOpen}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 6,
            background: "#2A3B7A",
            color: "#fff",
            border: "none",
            padding: "8px 16px",
            borderRadius: 10,
            fontSize: 12.5,
            fontWeight: 700,
            cursor: "pointer",
            fontFamily: "inherit",
          }}
        >
          打开
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M5 12h14M13 6l6 6-6 6" />
          </svg>
        </button>
      </div>
    </div>
  );
}

// Active graph (Toulmin) card: its interactive 5-slot builder lives in the
// 结构 CENTER pane (unlike annotate/compare, whose fill controls are the rail
// card itself). Rendering a second sheet here would double-render the same
// card, so the rail shows only a short handoff nudge pointing at that pane —
// the design's S3→S4 handoff tone.
function StructureHandoffNudge({ spec }: { spec: CardSpec }) {
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
      </div>
      <div style={{ padding: "0 15px 14px", fontSize: 11.5, color: "#8A92A3", lineHeight: 1.6 }}>
        论证结构在「结构」环节里搭——去中间那一栏，逐张卡片选素材、写句子。我在这儿盯着，一步卡住随时问我。
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
  sending = false,
  card = null,
  onOpenCard,
  onSubmitCard,
  onSkipCard,
  materials = [],
  lateralMaterialId = "",
  onLateralMaterialChange,
  locatedSpans,
  onRequestLocate,
  onSpanNotFound,
  pendingTrace,
}: CoachRailProps) {
  const [equipOpen, setEquipOpen] = useState(false);
  const [composerText, setComposerText] = useState("");
  const [recording, setRecording] = useState(false);
  const [voiceError, setVoiceError] = useState<string | null>(null);
  const micRef = useRef<MicCapture | null>(null);
  const asrRef = useRef<AsrStream | null>(null);

  const lastAi = [...messages].reverse().find((m) => m.kind === "ai");

  // Stable across renders (reads only refs + the setState setter, both
  // stable) so the unmount-cleanup effect below always tears down whatever
  // mic/ASR connection is live. Safe to call more than once: once torn
  // down, the refs are null and further calls are no-ops.
  const stopRecording = useCallback(() => {
    micRef.current?.stop();
    asrRef.current?.stop();
    micRef.current = null;
    asrRef.current = null;
    setRecording(false);
  }, []);

  // If the rail unmounts mid-recording (e.g. the student navigates away),
  // release the mic + ASR socket instead of leaving them running.
  useEffect(() => {
    return () => {
      stopRecording();
    };
  }, [stopRecording]);

  async function handleMicClick() {
    if (sending) return;
    if (recording) {
      stopRecording();
      return;
    }
    setVoiceError(null);
    setRecording(true);
    try {
      const asr = new AsrStream();
      // Transcript (partial or final) lands in the composer for the student
      // to see and edit — it is never auto-sent.
      asr.onPartial((t) => setComposerText(t));
      asr.onFinal((t) => setComposerText(t));
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

  function handleSend() {
    if (sending) return;
    const text = composerText.trim();
    if (!text) return;
    if (recording) stopRecording();
    onSend(text);
    setComposerText("");
  }

  return (
    <div style={{ width: 388, flex: "none", background: "#fff", borderLeft: "1px solid #EAECF2", display: "flex", flexDirection: "column", fontFamily: FONT, position: "relative" }}>
      <style>{`@keyframes coachRailPulse { 0%,100% { opacity: 1; } 50% { opacity: .35; } }
@keyframes coachRailRecPulse { 0%,100% { opacity: 1; } 50% { opacity: .3; } }`}</style>

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
                      {m.anchor && (
                        <span style={{ fontSize: 10.5, color: "#AEB4C2", fontWeight: 600 }}>锚定 {m.anchor}</span>
                      )}
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

        {card && card.status === "proposed" ? (
          <CardProposalBubble spec={card.spec} onOpen={() => onOpenCard?.(card.cardInstanceId)} />
        ) : card && card.status === "active" ? (
          card.spec.primitive === "annotate" ? (
            <StudioAnnotateCard
              spec={card.spec}
              anchors={card.anchors}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
              locatedSpans={locatedSpans}
              onRequestLocate={onRequestLocate}
              onSpanNotFound={onSpanNotFound}
              pendingTrace={pendingTrace}
            />
          ) : card.spec.primitive === "compare" ? (
            <StudioCompareCard
              spec={card.spec}
              anchors={card.anchors}
              materialId={card.materialId ?? ""}
              materials={materials}
              lateralMaterialId={lateralMaterialId}
              onLateralMaterialChange={(id) => onLateralMaterialChange?.(id)}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
          ) : card.spec.primitive === "sort" ? (
            <StudioSortCard
              spec={card.spec}
              cardInstanceId={card.cardInstanceId}
              anchors={card.anchors}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
          ) : card.spec.primitive === "scale" ? (
            <StudioScaleCard
              spec={card.spec}
              cardInstanceId={card.cardInstanceId}
              anchors={card.anchors}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
          ) : card.spec.primitive === "matrix" ? (
            <StudioMatrixCard
              spec={card.spec}
              cardInstanceId={card.cardInstanceId}
              anchors={card.anchors}
              onSubmit={(env) => onSubmitCard?.(env)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
          ) : card.spec.primitive === "graph" ? (
            <StructureHandoffNudge spec={card.spec} />
          ) : (
            <StudioCardSheet
              spec={card.spec}
              onSubmit={(finalEnvelope) => onSubmitCard?.(finalEnvelope)}
              onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
            />
          )
        ) : activeView === "素材" ? (
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
        {recording && (
          <div style={{ display: "flex", alignItems: "center", gap: 9, background: "#FBEEE7", border: "1px solid #F1D6C8", borderRadius: 11, padding: "9px 13px", marginBottom: 9 }}>
            <span style={{ width: 9, height: 9, borderRadius: "50%", background: "#D9534F", animation: "coachRailRecPulse 1.2s infinite" }} />
            <span style={{ fontSize: 12.5, fontWeight: 600, color: "#A8543A" }}>正在录音… 说完点麦克风结束</span>
            <span style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 2 }}>
              <span style={{ width: 3, height: 11, background: "#D9853A", borderRadius: 2 }} />
              <span style={{ width: 3, height: 16, background: "#D9853A", borderRadius: 2 }} />
              <span style={{ width: 3, height: 8, background: "#D9853A", borderRadius: 2 }} />
              <span style={{ width: 3, height: 14, background: "#D9853A", borderRadius: 2 }} />
            </span>
          </div>
        )}
        {voiceError && !recording && (
          <div
            role="alert"
            onClick={() => setVoiceError(null)}
            title="点击关闭"
            style={{ display: "flex", alignItems: "center", gap: 8, background: "#FDEEEC", border: "1px solid #F3C9C0", borderRadius: 11, padding: "9px 13px", marginBottom: 9, cursor: "pointer" }}
          >
            <FlagIcon />
            <span style={{ fontSize: 12.5, fontWeight: 600, color: "#C0392B" }}>{voiceError}</span>
          </div>
        )}
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
            disabled={sending}
            style={{ flex: 1, border: "none", outline: "none", resize: "none", fontSize: 14, lineHeight: 1.6, color: "#1C2333", background: "transparent", maxHeight: 100, padding: "6px 0", fontFamily: "inherit" }}
          />
          <div
            onClick={handleMicClick}
            title="语音输入"
            role="button"
            aria-pressed={recording}
            aria-label={recording ? "正在录音，点击结束" : "语音输入"}
            style={{
              flex: "none",
              width: 32,
              height: 32,
              borderRadius: 9,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: sending ? "not-allowed" : "pointer",
              color: recording ? "#D9534F" : "#6B7384",
              background: recording ? "#FBEEE7" : "transparent",
              opacity: sending ? 0.5 : 1,
            }}
          >
            <MicIcon />
          </div>
          <button
            type="button"
            aria-label="发送"
            onClick={handleSend}
            disabled={sending || !composerText.trim()}
            style={{
              flex: "none",
              width: 36,
              height: 36,
              borderRadius: 10,
              background: "#2A3B7A",
              border: "none",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: sending || !composerText.trim() ? "not-allowed" : "pointer",
              opacity: sending || !composerText.trim() ? 0.5 : 1,
            }}
          >
            <SendIcon />
          </button>
        </div>
      </div>
    </div>
  );
}
