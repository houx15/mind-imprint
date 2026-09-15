// writings/gradingsChanged.ts — tells an open FinishedWritingPage to fetch
// its 老师批改 again.
//
// The inbox opens a grading by navigating to the writing's page. When that
// page is already on screen, navigate() does nothing (same path), so the
// inbox also fires this event and the page refetches.

export const GRADINGS_CHANGED_EVENT = "lite:gradings-changed";

export function notifyGradingsChanged(atomId: string): void {
  window.dispatchEvent(new CustomEvent(GRADINGS_CHANGED_EVENT, { detail: { atomId } }));
}

/** The writing an event names, or `null` for an event without one. */
export function gradingsChangedAtom(e: Event): string | null {
  const detail: unknown = (e as CustomEvent<unknown>).detail;
  if (typeof detail !== "object" || detail === null) return null;
  const atomId = (detail as { atomId?: unknown }).atomId;
  return typeof atomId === "string" ? atomId : null;
}
