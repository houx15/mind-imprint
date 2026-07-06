import { useState } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { SessionStore } from "./session";
import { LeftRail } from "./LeftRail";
import { DirectoryView } from "./directory/DirectoryView";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { RecordsView } from "./records/RecordsView";
import { SettingsView } from "./settings/SettingsView";

type Tab = "courses" | "tasks" | "records" | "settings";
type TaskView = "directory" | "workspace";

export function StudentApp({
  store,
  session,
  onLogout,
}: {
  store: Store;
  session: SessionStore;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<Tab>("tasks");
  const [taskView, setTaskView] = useState<TaskView>("directory");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [openingMessage, setOpeningMessage] = useState<string | undefined>(undefined);

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <LeftRail tab={tab} onTab={setTab} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "tasks" && taskView === "directory" && (
          <DirectoryView
            store={store}
            onOpenTask={(id, opening) => {
              setActiveTaskId(id);
              setOpeningMessage(opening);
              setTaskView("workspace");
            }}
          />
        )}
        {tab === "tasks" && taskView === "workspace" && (
          <WorkspaceContainer
            store={store}
            taskId={activeTaskId!}
            openingMessage={openingMessage}
            onBack={() => { setTaskView("directory"); setOpeningMessage(undefined); }}
          />
        )}
        {tab === "records" && <RecordsView store={store} registry={CARD_REGISTRY} />}
        {tab === "settings" && (
          <SettingsView session={session} user={session.getUser()} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
