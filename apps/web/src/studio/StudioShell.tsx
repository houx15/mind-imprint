import { StationRail } from "./StationRail";
import { ViewFrame } from "./ViewFrame";
import { CoachRail } from "./CoachRail";
import type { StudioCallbacks, StudioState } from "./state";

export type StudioShellProps = {
  state: StudioState;
  callbacks: StudioCallbacks;
};

// Top bar + 3-column body + focus mode. Design binding:
// docs/design/思维印记_工作区.dc.html ~L757-780 (top bar + focus-exit button),
// body row ~L780 (station rail / center view frame / coach rail).
const FONT = "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif";

function BackIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" />
    </svg>
  );
}

function FocusIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7" />
    </svg>
  );
}

function ExitFocusIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 3H5a2 2 0 00-2 2v4M15 3h4a2 2 0 012 2v4M9 21H5a2 2 0 01-2-2v-4M15 21h4a2 2 0 002-2v-4" />
    </svg>
  );
}

export function StudioShell({ state, callbacks }: StudioShellProps) {
  const activeView = state.stations.find((s) => s.code === state.activeStation)?.view ?? "结构";

  return (
    <div style={{ height: "100%", minHeight: "100vh", display: "flex", flexDirection: "column", position: "relative", fontFamily: FONT }}>
      {!state.focusMode && (
        <div style={{ height: 56, flex: "none", background: "#fff", borderBottom: "1px solid #EAECF2", display: "flex", alignItems: "center", padding: "0 22px", gap: 14 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 7, color: "#6B7384", fontSize: 13, fontWeight: 600, cursor: "pointer", padding: "7px 11px", borderRadius: 9 }}>
            <BackIcon />
            写作工作室
          </div>
          <div style={{ width: 1, height: 22, background: "#EAECF2" }} />
          <span style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
            {state.project.title}
          </span>
          <span style={{ flex: "none", fontSize: 11.5, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "3px 10px", borderRadius: 999 }}>
            {state.project.qualLabel}
          </span>
          <button
            type="button"
            onClick={callbacks.onToggleFocus}
            title="专注模式：收起顶栏与侧栏，只留写作区与 AI 陪练"
            style={{
              marginLeft: "auto",
              flex: "none",
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              fontSize: 13,
              fontWeight: 700,
              color: state.focusMode ? "#2A3B7A" : "#6B7384",
              background: state.focusMode ? "#EDEFF9" : "transparent",
              border: `1px solid ${state.focusMode ? "#DfE3F4" : "#E7E9F0"}`,
              padding: "7px 13px",
              borderRadius: 10,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            <FocusIcon />
            {state.focusMode ? "退出专注" : "专注模式"}
          </button>
        </div>
      )}

      {state.focusMode && (
        <button
          type="button"
          onClick={callbacks.onToggleFocus}
          title="退出专注"
          style={{
            position: "absolute",
            top: 12,
            right: 18,
            zIndex: 40,
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            fontSize: 12.5,
            fontWeight: 700,
            color: "#2A3B7A",
            background: "#fff",
            border: "1px solid #DfE3F4",
            boxShadow: "0 4px 14px rgba(20,30,60,.14)",
            padding: "8px 13px",
            borderRadius: 10,
            cursor: "pointer",
            fontFamily: "inherit",
          }}
        >
          <ExitFocusIcon />
          退出专注
        </button>
      )}

      <div style={{ flex: 1, minHeight: 0, display: "flex", overflowX: "auto" }}>
        <StationRail stations={state.stations} active={state.activeStation} focus={state.focusMode} onSelect={callbacks.onSelectStation} />
        <ViewFrame state={state} />
        <CoachRail
          anchor={state.coach.anchor}
          messages={state.coach.messages}
          equipment={state.coach.equipment}
          activeView={activeView}
          onDisposition={callbacks.onDisposition}
          onOpenMethodology={callbacks.onOpenMethodology}
          onSend={callbacks.onComposerSend}
        />
      </div>
    </div>
  );
}
