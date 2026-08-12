import { apiFetch } from "./client";

// revision.ts — recording the student's revision history. `scheduleCardRevision`
// is called when a FINISHED card is edited (editFilledPart) — the student went
// back and reworked a part after AI comment / after 我写好了. It debounces per
// project (longer than the 700ms snippet-save debounce, so the server reads the
// SAVED snippet state) and posts a best-effort checkpoint; the server dedups by
// content hash, so idle fires never pile up identical rows.

const timers = new Map<string, ReturnType<typeof setTimeout>>();
const DEBOUNCE_MS = 1500;

export function scheduleCardRevision(projectId: string): void {
  const existing = timers.get(projectId);
  if (existing) clearTimeout(existing);
  timers.set(
    projectId,
    setTimeout(() => {
      timers.delete(projectId);
      void apiFetch(`/api/v1/projects/${projectId}/revision/card-edit`, { method: "POST" }).catch(() => {
        /* best-effort: a missed revision checkpoint never disturbs writing */
      });
    }, DEBOUNCE_MS),
  );
}
