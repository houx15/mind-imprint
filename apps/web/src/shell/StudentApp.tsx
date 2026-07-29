import { useState } from "react";
import type { SessionStore } from "./session";
import { LeftRail } from "./LeftRail";
import { CoursesContainer } from "./courses/CoursesContainer";
import { WorkspaceContainer } from "../workspace/WorkspaceContainer";
import { GrowthReport } from "./growth/GrowthReport";
import { SettingsView } from "./settings/SettingsView";
import { ChatContainer } from "./chat/ChatContainer";

type Tab = "chat" | "courses" | "studio" | "growth" | "settings";

export function StudentApp({
  session,
  onLogout,
}: {
  session: SessionStore;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<Tab>("studio");
  // When the student opens a specific project's evaluation report (from the
  // Directory "查看评估报告" action), we deep-link the 成长报告 to that entry.
  // Cleared when the growth tab is opened directly from the left rail.
  const [growthFocus, setGrowthFocus] = useState<string | null>(null);

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <LeftRail tab={tab} onTab={(t) => { if (t === "growth") setGrowthFocus(null); setTab(t); }} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "chat" && <ChatContainer />}
        {tab === "courses" && <CoursesContainer onGoPortal={() => setTab("studio")} />}
        {tab === "studio" && (
          <WorkspaceContainer
            onFinished={(projectId?: string) => {
              setGrowthFocus(projectId ?? null);
              setTab("growth");
            }}
          />
        )}
        {tab === "growth" && <GrowthReport initialScopeId={growthFocus} />}
        {tab === "settings" && (
          <SettingsView session={session} user={session.getUser()} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
