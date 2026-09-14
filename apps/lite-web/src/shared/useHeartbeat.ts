import { useEffect } from "react";
import { sendHeartbeat, type HeartbeatKind } from "../api/reports";

/**
 * useHeartbeat — the client half of the end-of-session report's minute
 * count (server half: `POST /api/v1/{readings,writings}/{id}/heartbeat` and
 * `POST /api/v1/pbl/projects/{id}/heartbeat`, clamped to 120s per call and
 * ignored once the atom is finished).
 *
 * Posts `{seconds: 60}` once every 60 seconds — comfortably under the
 * server's clamp, so a well-behaved tick never gets truncated — and ONLY
 * while both of these hold:
 *
 *   - `document.visibilityState === "visible"`. This is the single most
 *     important line here: a student who leaves the tab open overnight, or
 *     switches away to something else, must not accrue hours of "focus
 *     time" she never spent. The number on her report has to be one she'd
 *     recognize as true.
 *   - `enabled`, which the hosts set to "the atom is loaded and NOT
 *     finished" — re-reading or re-opening a finished piece is not time
 *     spent on the work, so it must not add minutes to a report that has
 *     already been generated.
 *
 * The interval is torn down on unmount and whenever `enabled`/`atomId`
 * changes, so nothing leaks across a route change.
 *
 * Every failure is swallowed. A missed heartbeat is bookkeeping, not a
 * feature: it must never surface an error to her, never retry-storm, and
 * never interrupt whatever she is doing in the room.
 */
export function useHeartbeat(kind: HeartbeatKind, atomId: string, enabled: boolean): void {
  useEffect(() => {
    if (!enabled) return;
    const id = window.setInterval(() => {
      if (document.visibilityState !== "visible") return;
      sendHeartbeat(kind, atomId, 60).catch(() => {
        // Swallowed on purpose — see the module doc above.
      });
    }, 60_000);
    return () => window.clearInterval(id);
  }, [kind, atomId, enabled]);
}
