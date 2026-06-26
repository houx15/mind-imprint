import { useEffect, useState } from "react";
import { createStore } from "../store";
import { createSession, useSession, type SessionStore } from "./session";
import type { Store } from "../store/createStore";
import { api as defaultApi, type ApiClient, type MeUser } from "../api";
import { AuthScreen } from "./auth/AuthScreen";
import { StudentApp } from "./StudentApp";
import { ConsoleShell } from "../console/ConsoleShell";

const defaultStore = createStore({});
const defaultSession = createSession({ storage: window.localStorage });

type ShellClient = Pick<
  ApiClient,
  | "getMe" | "signout"
  | "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
  | "getOverview" | "listTeacherInvites" | "createTeacherInvite" | "adminImport"
  | "listTeachers" | "assignTeacher" | "removeTeacher"
>;

export function AppShell({
  store = defaultStore,
  session = defaultSession,
  client = defaultApi,
}: {
  store?: Store;
  session?: SessionStore;
  client?: ShellClient;
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

  if (!booted) {
    return <div style={{ width: "100%", height: "100%", background: "#F3F4F8" }} />;
  }
  if (!sess.authed) {
    return <AuthScreen onAuthed={(u) => { session.setUser(u); session.setAuthed(true); }} />;
  }

  const onLogout = () => { void client.signout().finally(() => { session.setUser(null); session.setAuthed(false); }); };
  const role = session.getUser()?.role;

  if (role === "teacher" || role === "admin") {
    return <ConsoleShell session={session} client={client} onLogout={onLogout} />;
  }
  return <StudentApp store={store} session={session} onLogout={onLogout} />;
}
