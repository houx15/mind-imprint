import { useState } from "react";
import { createStore } from "../store";
import { createSession, useSession, type SessionStore } from "./session";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import { AuthScreen } from "./auth/AuthScreen";
import { LeftRail } from "./LeftRail";
import { DirectoryView } from "./directory/DirectoryView";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { RecordsView } from "./records/RecordsView";
import { SettingsView } from "./settings/SettingsView";

// Module-level singletons — used when no props are injected (production entry)
const defaultStore = createStore({});
const defaultSession = createSession({ storage: window.localStorage });

type Tab = "tasks" | "records" | "settings";
type TaskView = "directory" | "workspace";

export function AppShell({
  store = defaultStore,
  session = defaultSession,
}: {
  store?: Store;
  session?: SessionStore;
}) {
  const sess = useSession(session);
  const screen = sess.authed ? "app" : "auth";

  const [tab, setTab] = useState<Tab>("tasks");
  const [taskView, setTaskView] = useState<TaskView>("directory");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [openingMessage, setOpeningMessage] = useState<string | undefined>(undefined);

  if (screen === "auth") {
    return <AuthScreen onEnterApp={() => session.setAuthed(true)} />;
  }

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
          <SettingsView session={session} onLogout={() => session.setAuthed(false)} />
        )}
      </div>
    </div>
  );
}
