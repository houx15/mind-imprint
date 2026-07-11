import { useState } from "react";
import type { StudioEvent } from "@mind-imprint/contracts";
import { SourceDossier } from "../workspace/material/SourceDossier";
import { SOURCE_FIXTURES } from "../workspace/material/fixtures";

export function MaterialPanel() {
  const [events, setEvents] = useState<StudioEvent[]>([]);

  return (
    <div className="min-h-screen bg-mk-bg p-8 font-sans text-mk-ink">
      <div className="mx-auto max-w-[760px] space-y-5">
        <div className="rounded-mk border border-mk-border bg-white p-5">
          <SourceDossier sources={SOURCE_FIXTURES} onEvent={(e) => setEvents((prev) => [...prev, e])} />
        </div>

        <pre
          data-testid="material-event-log"
          className="overflow-auto rounded-mk border border-mk-border bg-white p-4 text-[12px] text-mk-ink"
        >
          {JSON.stringify(events, null, 2)}
        </pre>
      </div>
    </div>
  );
}
