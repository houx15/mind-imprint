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
  | "getMe" | "signin" | "signout"
  | "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
  | "getOverview" | "listTeacherInvites" | "createTeacherInvite" | "adminImport"
  | "listTeachers" | "assignTeacher" | "removeTeacher"
>;

// The marketing site's "体验 Demo" entrance deep-links here with `?trial=1`.
// With no existing session we sign in as the seeded sample student so the full
// flow is reachable without a class join-code. These are the public demo creds
// from apps/api/internal/store/migrations/0004_seed_password.sql — not a secret.
// (A dedicated, sandboxed backend demo endpoint is the eventual production form.)
const DEMO_EMAIL = "phoebe@demo.mindimprint.local";
const DEMO_PASSWORD = "phoebe-dev-pass";

function wantsTrial(): boolean {
  if (typeof window === "undefined") return false;
  return new URLSearchParams(window.location.search).get("trial") === "1";
}

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
    async function boot() {
      try {
        const u = await client.getMe();
        if (!cancelled) { session.setUser(u); session.setAuthed(true); }
      } catch {
        // No live session. If arriving from the marketing "体验 Demo" entrance,
        // auto-sign-in as the seeded sample student before falling back to auth.
        if (wantsTrial()) {
          try {
            const u = await client.signin({ email: DEMO_EMAIL, password: DEMO_PASSWORD });
            if (!cancelled) {
              session.setUser(u);
              session.setAuthed(true);
              // Tidy the URL so a refresh doesn't re-trigger the trial path.
              window.history.replaceState({}, "", window.location.pathname);
            }
            return;
          } catch { /* fall through to the auth screen */ }
        }
        if (!cancelled) session.setAuthed(false);
      } finally {
        if (!cancelled) setBooted(true);
      }
    }
    void boot();
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
