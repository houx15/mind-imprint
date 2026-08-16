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
// P2-07 durability: a rejected save must NEVER look like a successful one.
// `pending` is only cleared once `save()` resolves; a rejection keeps the
// dirty snapshot and arms a bounded-backoff retry. `mutationSeq` guards the
// (rarer) case of a mutation arriving while a save is in flight — that
// mutation re-dirties independently rather than being silently swept up by
// the in-flight save's success.
//
// Determinism: no Date.now / Math.random / id minting here — session identity
// and timestamps come from the server. The one real timer (the debounce) is the
// host boundary, exactly where side effects are permitted.

/** Persistence status a host UI can surface (e.g. "保存中…" / "保存失败，重试中"). */
export type SaveStatus = "idle" | "saving" | "saved" | "error";

/** Extends the runtime's SessionAdapter with a manual snapshot flush + status. */
export interface ApiSessionAdapter extends SessionAdapter {
  /** Immediately persist any pending snapshot (e.g. on unmount / page hide). */
  flush(): Promise<void>;
  /** Current persistence status of the held session. */
  getSaveStatus(): SaveStatus;
}

export interface ApiSessionAdapterOptions {
  /** Debounce window before a coalesced snapshot PUT. Default 400ms. */
  debounceMs?: number;
  /** Cap for the retry backoff (ms). Default 10s. */
  maxRetryMs?: number;
  /** Injectable client seams (tests pass fakes; prod uses the real client). */
  createSession?: (slug: string) => Promise<CourseSession>;
  saveSession?: (slug: string, session: CourseSession) => Promise<void>;
}

export function makeApiSessionAdapter(slug: string, opts: ApiSessionAdapterOptions = {}): ApiSessionAdapter {
  const debounceMs = opts.debounceMs ?? 400;
  const maxRetryMs = opts.maxRetryMs ?? 10_000;
  const create = opts.createSession ?? createCourseSession;
  const save = opts.saveSession ?? saveCourseSession;

  let current: CourseSession | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let pending = false;
  let mutationSeq = 0;
  let retryDelayMs = debounceMs;
  let saveStatus: SaveStatus = "idle";
  let inFlight: Promise<void> | null = null;

  function scheduleRetry(): void {
    if (retryTimer) clearTimeout(retryTimer);
    retryTimer = setTimeout(() => {
      retryTimer = null;
      // Unattended retry: doFlush() already records saveStatus and re-arms
      // the next retry on failure, so there's nothing further to do here —
      // just avoid an unhandled rejection.
      void flush().catch(() => {});
    }, retryDelayMs);
    retryDelayMs = Math.min(retryDelayMs * 2, maxRetryMs);
  }

  async function doFlush(): Promise<void> {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
    if (retryTimer) {
      clearTimeout(retryTimer);
      retryTimer = null;
    }
    if (!pending || !current) return;
    const seqAtStart = mutationSeq;
    saveStatus = "saving";
    try {
      await save(slug, current);
      // Only clear the dirty flag if nothing mutated the session while this
      // save was in flight — a concurrent mutation re-dirties on its own.
      if (mutationSeq === seqAtStart) pending = false;
      saveStatus = "saved";
      retryDelayMs = debounceMs;
    } catch (err) {
      saveStatus = "error";
      scheduleRetry();
      throw err;
    }
  }

  /** De-duped: overlapping callers (e.g. pagehide + unmount) share one save. */
  function flush(): Promise<void> {
    if (!inFlight) {
      inFlight = doFlush().finally(() => {
        inFlight = null;
      });
    }
    return inFlight;
  }

  function scheduleSnapshot(): void {
    pending = true;
    mutationSeq += 1;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      // Debounced auto-save: handle the rejection here instead of a bare
      // `void flush()` (P2-07) — doFlush() already records saveStatus="error"
      // and arms a retry, so this just prevents an unhandled rejection.
      void flush().catch(() => {});
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

    async setCurrent(sessionId: string, next: CourseSession["current"]): Promise<void> {
      const session = requireLoaded(sessionId);
      session.current = next;
      scheduleSnapshot();
    },

    flush,
    getSaveStatus: () => saveStatus,
  };
}
