import { useState } from "react";
import type { SessionStore } from "./session";
import { LeftRail } from "./LeftRail";
import { CoursesContainer } from "./courses/CoursesContainer";
import { StudioContainer } from "../studio/StudioContainer";
import { GrowthPlaceholder } from "./growth/GrowthPlaceholder";
import { SettingsView } from "./settings/SettingsView";

type Tab = "courses" | "studio" | "growth" | "settings";

export function StudentApp({
  session,
  onLogout,
}: {
  session: SessionStore;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<Tab>("studio");

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <LeftRail tab={tab} onTab={setTab} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "courses" && <CoursesContainer onGoPortal={() => setTab("studio")} />}
        {tab === "studio" && <StudioContainer />}
        {tab === "growth" && <GrowthPlaceholder />}
        {tab === "settings" && (
          <SettingsView session={session} user={session.getUser()} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
