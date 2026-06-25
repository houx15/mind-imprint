import { z } from "zod";
import { useSyncExternalStore } from "react";
import type { RawStorage } from "../store";
import type { MeUser } from "../api";

export const SESSION_KEY = "mk.session";

export const Session = z.object({
  authed: z.boolean().default(false),
  aiAvatar: z.string().default("#2A3B7A"),
});
export type Session = z.infer<typeof Session>;

const DEFAULT: Session = { authed: false, aiAvatar: "#2A3B7A" };

function load(storage: RawStorage): Session {
  const raw = storage.getItem(SESSION_KEY);
  if (raw == null) return DEFAULT;
  try {
    const parsed = Session.safeParse(JSON.parse(raw));
    return parsed.success ? parsed.data : DEFAULT;
  } catch {
    return DEFAULT;
  }
}

export interface SessionStore {
  getSnapshot(): Session;
  subscribe(listener: () => void): () => void;
  setAuthed(v: boolean): void;
  setAvatar(color: string): void;
  getUser(): MeUser | null;
  setUser(u: MeUser | null): void;
}

export function createSession(opts: { storage: RawStorage }): SessionStore {
  let state: Session = load(opts.storage);
  let user: MeUser | null = null;
  const listeners = new Set<() => void>();

  function commit(next: Session): void {
    const changed = next.authed !== state.authed || next.aiAvatar !== state.aiAvatar;
    if (!changed) return;
    state = next;
    opts.storage.setItem(SESSION_KEY, JSON.stringify(state));
    listeners.forEach((l) => l());
  }

  return {
    getSnapshot: () => state,
    subscribe(l) { listeners.add(l); return () => { listeners.delete(l); }; },
    setAuthed(v) { commit({ ...state, authed: v }); },
    setAvatar(color) { commit({ ...state, aiAvatar: color }); },
    getUser: () => user,
    setUser(u) { user = u; listeners.forEach((l) => l()); },
  };
}

export function useSession(s: SessionStore): Session {
  return useSyncExternalStore(s.subscribe, s.getSnapshot);
}
