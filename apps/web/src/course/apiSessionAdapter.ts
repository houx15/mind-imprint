import type {
  CourseRuntimeEvent,
  CourseSession,
  RuntimeSceneResult,
  SliceSessionState,
} from "@mind-imprint/course-contract";
import type { CreateSessionInput, SceneSlot, SessionAdapter } from "@mind-imprint/course-runtime";
import { createCourseSession, saveCourseSession } from "@/api/courseDefinition";

// apiSessionAdapter.ts — the API-backed SessionAdapter (Course Runtime §16,
// Slice 8, implementing the Slice 2 interface). The runtime holds authoritative
// session state client-side; the server persists a full snapshot per
// (user, course). So this adapter keeps the CourseSession IN MEMORY after
// create/load and, on every mutating call, updates that local copy and
// DEBOUNCE-snapshots the WHOLE session via PUT — one coalesced write instead of
// a chatty per-event round trip. `create`/`load` both go through the get-or-
// create POST (there is no per-id GET; get-or-create returns the resumed session
// when one exists). A terminal `completed` status flushes immediately.
//
// Determinism: no Date.now / Math.random / id minting here — session identity
// and timestamps come from the server. The one real timer (the debounce) is the
// host boundary, exactly where side effects are permitted.

/** Extends the runtime's SessionAdapter with a manual snapshot flush. */
export interface ApiSessionAdapter extends SessionAdapter {
  /** Immediately persist any pending snapshot (e.g. on unmount / page hide). */
  flush(): Promise<void>;
}

export interface ApiSessionAdapterOptions {
  /** Debounce window before a coalesced snapshot PUT. Default 400ms. */
  debounceMs?: number;
  /** Injectable client seams (tests pass fakes; prod uses the real client). */
  createSession?: (slug: string) => Promise<CourseSession>;
  saveSession?: (slug: string, session: CourseSession) => Promise<void>;
}

export function makeApiSessionAdapter(slug: string, opts: ApiSessionAdapterOptions = {}): ApiSessionAdapter {
  const debounceMs = opts.debounceMs ?? 400;
  const create = opts.createSession ?? createCourseSession;
  const save = opts.saveSession ?? saveCourseSession;

  let current: CourseSession | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let pending = false;

  async function flush(): Promise<void> {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
    if (!pending || !current) return;
    pending = false;
    await save(slug, current);
  }

  function scheduleSnapshot(): void {
    pending = true;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      void flush();
    }, debounceMs);
  }

  function requireLoaded(sessionId: string): CourseSession {
    if (!current) throw new Error(`ApiSessionAdapter: no session loaded (asked for '${sessionId}')`);
    return current;
  }

  return {
    async load(sessionId: string): Promise<CourseSession | null> {
      // Held copy wins (the runtime just created/mutated it); otherwise resume
      // via the get-or-create POST (the server returns the persisted session).
      if (current && current.id === sessionId) return current;
      current = await create(slug);
      return current;
    },

    async create(_input: CreateSessionInput): Promise<CourseSession> {
      // Identity/authorship are the server's; the input courseId/studentId are
      // advisory. Returns the server's authoritative session.
      current = await create(slug);
      return current;
    },

    async appendEvent(sessionId: string, event: CourseRuntimeEvent): Promise<void> {
      const session = requireLoaded(sessionId);
      session.events.push(event);
      scheduleSnapshot();
    },

    async saveSliceState(sessionId: string, sliceId: string, state: SliceSessionState): Promise<void> {
      const session = requireLoaded(sessionId);
      session.sliceStates[sliceId] = state;
      scheduleSnapshot();
    },

    async saveScene(sessionId: string, which: SceneSlot, result: RuntimeSceneResult): Promise<void> {
      const session = requireLoaded(sessionId);
      session[which] = result;
      scheduleSnapshot();
    },

    async setStatus(sessionId: string, status: CourseSession["status"]): Promise<void> {
      const session = requireLoaded(sessionId);
      session.status = status;
      scheduleSnapshot();
      // The student may leave right after the course completes — persist now.
      if (status === "completed") await flush();
    },

    flush,
  };
}
