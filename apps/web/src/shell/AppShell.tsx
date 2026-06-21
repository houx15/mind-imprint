import { useState } from "react";
import { createStore } from "../store";
import { createSession, useSession, type SessionStore } from "./session";
import { loadConfig, isVerified } from "../llm";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { LlmConfig } from "../llm/types";
import type { ChatFn } from "../agent/runEvaluation";

import { AuthScreen } from "./auth/AuthScreen";
import { LeftRail } from "./LeftRail";
import { DirectoryView } from "./directory/DirectoryView";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { RecordsView } from "./records/RecordsView";
import { SettingsView } from "./settings/SettingsView";
import { KeyGateModal } from "./KeyGateModal";

// Module-level singletons — used when no props are injected (production entry)
const defaultStore = createStore({ storage: window.localStorage });
const defaultSession = createSession({ storage: window.localStorage });

type Tab = "tasks" | "records" | "settings";
type TaskView = "directory" | "workspace";

export function AppShell({
  store = defaultStore,
  session = defaultSession,
  chat,
  initialConfig,
}: {
  store?: Store;
  session?: SessionStore;
  chat?: ChatFn;
  initialConfig?: Partial<LlmConfig>;
}) {
  const sess = useSession(session);
  const screen = sess.authed ? "app" : "auth";

  const [tab, setTab] = useState<Tab>("tasks");
  const [taskView, setTaskView] = useState<TaskView>("directory");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [verified, setVerified] = useState<boolean>(() => isVerified(loadConfig()));

  if (screen === "auth") {
    return <AuthScreen onEnterApp={() => session.setAuthed(true)} />;
  }

  return (
    <div
      style={{
        display: "flex",
        height: "100%",
        width: "100%",
        background: "#F3F4F8",
        overflow: "hidden",
      }}
    >
      <LeftRail tab={tab} onTab={setTab} />

      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "tasks" && taskView === "directory" && (
          <DirectoryView
            store={store}
            onOpenTask={(id) => {
              setActiveTaskId(id);
              setTaskView("workspace");
            }}
          />
        )}
        {tab === "tasks" && taskView === "workspace" && (
          <WorkspaceContainer
            store={store}
            taskId={activeTaskId!}
            onBack={() => setTaskView("directory")}
            chat={chat}
            config={initialConfig}
          />
        )}
        {tab === "records" && (
          <RecordsView store={store} registry={CARD_REGISTRY} />
        )}
        {tab === "settings" && (
          <SettingsView
            session={session}
            chat={chat}
            onLogout={() => session.setAuthed(false)}
          />
        )}
      </div>

      {!verified && (
        <KeyGateModal chat={chat} onPass={() => setVerified(true)} />
      )}
    </div>
  );
}
