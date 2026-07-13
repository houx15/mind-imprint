import { SourceDossier } from "../studio/material/SourceDossier";
import { MATERIAL_FIXTURE } from "../studio/fixtures";

export function MaterialPanel() {
  return (
    <div className="min-h-screen bg-mk-bg p-8 font-sans text-mk-ink">
      <div className="mx-auto max-w-[760px] space-y-5">
        <div className="rounded-mk border border-mk-border bg-white p-5">
          <SourceDossier sources={MATERIAL_FIXTURE} />
        </div>
      </div>
    </div>
  );
}
