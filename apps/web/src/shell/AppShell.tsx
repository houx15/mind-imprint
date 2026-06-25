import { useEffect, useState } from "react";
import { createStore } from "../store";
import { createSession, useSession, type SessionStore } from "./session";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import { api as defaultApi, type ApiClient, type MeUser } from "../api";
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
  client = defaultApi,
}: {
  store?: Store;
  session?: SessionStore;
  client?: Pick<ApiClient, "getMe" | "signout">;
}) {
  const sess = useSession(session);
  const [booted, setBooted] = useState(false);

  useEffect(() => {
    let cancelled = false;
    client.getMe()
      .then((u: MeUser) => { if (!cancelled) { session.setUser(u); session.setAuthed(true); } })
      .catch(() => { if (!cancelled) session.setAuthed(false); })
      .finally(() => { if (!cancelled) setBooted(true); });
    return () => { cancelled = true; };
  }, [client, session]);

  const screen = sess.authed ? "app" : "auth";

  const [tab, setTab] = useState<Tab>("tasks");
  const [taskView, setTaskView] = useState<TaskView>("directory");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [openingMessage, setOpeningMessage] = useState<string | undefined>(undefined);

  if (!booted) {
    return <div style={{ width: "100%", height: "100%", background: "#F3F4F8" }} />;
  }
  if (screen === "auth") {
    return <AuthScreen onAuthed={(u) => { session.setUser(u); session.setAuthed(true); }} />;
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
          <SettingsView
            session={session}
            user={session.getUser()}
            onLogout={() => { void client.signout().finally(() => { session.setUser(null); session.setAuthed(false); }); }}
          />
        )}
      </div>
    </div>
  );
}
