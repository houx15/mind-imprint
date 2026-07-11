import { useState } from "react";
import { StudioShell } from "../studio/StudioShell";
import { STUDIO_FIXTURE } from "../studio/fixtures";
import type { StationCode, StudioCallbacks, StudioState } from "../studio/state";

export function StudioPanel() {
  const [state, setState] = useState<StudioState>(STUDIO_FIXTURE);
  const [events, setEvents] = useState<unknown[]>([]);

  const callbacks: StudioCallbacks = {
    onSelectStation: (code: StationCode) => setState((prev) => ({ ...prev, activeStation: code })),
    onToggleFocus: () => setState((prev) => ({ ...prev, focusMode: !prev.focusMode })),
    onDisposition: (choice, reason) => setEvents((prev) => [...prev, { type: "disposition", choice, reason }]),
    onOpenMethodology: (cardId) => setEvents((prev) => [...prev, { type: "open_methodology", cardId }]),
    onComposerSend: (text) => setEvents((prev) => [...prev, { type: "composer_send", text }]),
  };

  return (
    <div className="min-h-screen bg-mk-bg font-sans text-mk-ink">
      <StudioShell state={state} callbacks={callbacks} />

      <div className="mx-auto max-w-[760px] space-y-3 p-8">
        <pre
          data-testid="studio-event-log"
          className="overflow-auto rounded-mk border border-mk-border bg-white p-4 text-[12px] text-mk-ink"
        >
          {JSON.stringify(events, null, 2)}
        </pre>
      </div>
    </div>
  );
}
