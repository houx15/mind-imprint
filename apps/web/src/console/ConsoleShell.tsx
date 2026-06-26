import { useState } from "react";
import type { ApiClient } from "../api";
import type { SessionStore } from "../shell/session";
import { SettingsView } from "../shell/settings/SettingsView";
import { ConsoleRail, type ConsoleTab } from "./ConsoleRail";
import { ClassesView } from "./ClassesView";
import { ClassDetailView } from "./ClassDetailView";

export type ConsoleClient = Pick<
  ApiClient,
  "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
>;

export function ConsoleShell({
  session,
  client,
  onLogout,
}: {
  session: SessionStore;
  client: ConsoleClient;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<ConsoleTab>("classes");
  const [openClassId, setOpenClassId] = useState<string | null>(null);
  const user = session.getUser();
  const role = user?.role ?? "admin";

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <ConsoleRail tab={tab} onTab={(t) => { setTab(t); if (t === "classes") setOpenClassId(null); }} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative", display: "flex" }}>
        {tab === "classes" && openClassId == null && (
          <ClassesView client={client} role={role} onOpenClass={setOpenClassId} />
        )}
        {tab === "classes" && openClassId != null && (
          <ClassDetailView client={client} classId={openClassId} onBack={() => setOpenClassId(null)} />
        )}
        {tab === "settings" && (
          <SettingsView session={session} user={user} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
