import { useEffect, useState } from "react";
import type { CardInstance, CardSpec, TraceEvent } from "@mind-imprint/contracts";
import { pickCardBody } from "../cards/customRenderers";
import { envelopeReducer, newEnvelope } from "../cards/envelopeReducer";

export type StudioCardSheetProps = {
  spec: CardSpec;
  onSubmit: (finalEnvelope: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
  // When set, the in-progress envelope is mirrored to localStorage under this
  // key, so a mid-fill refresh / tab-close doesn't silently wipe what the
  // student typed (过程即数据 — an unfinished card is still her work; the field
  // values otherwise live only in this component's state). Cleared on submit
  // or skip. Per-viewer convenience only; the server stays the source of truth
  // for *submitted* cards.
  persistKey?: string;
};

// Lean coach-rail-fit card host: reuses the SAME schema-driven renderer +
// reducer as the retired task-workspace card host used to, but laid out for
// the 388px coach-rail column instead of a full-bleed bottom-sheet. No
// card-specific branching here — pickCardBody(spec.id) resolves any custom
// renderer, CardRenderer otherwise (schema-driven).
export function StudioCardSheet({ spec, onSubmit, onSkip, persistKey }: StudioCardSheetProps) {
  // task_id is a vestigial local artifact — the project submit endpoint
  // ignores it — so an empty string is fine here.
  const [env, setEnv] = useState<CardInstance>(() => {
    if (persistKey) {
      try {
        const raw = localStorage.getItem(persistKey);
        if (raw) {
          const saved = JSON.parse(raw) as CardInstance;
          // Only restore a draft that belongs to THIS card spec.
          if (saved && saved.card_id === spec.id) return saved;
        }
      } catch {
        // corrupt / blocked storage — fall through to a fresh envelope
      }
    }
    return newEnvelope(spec.id, "");
  });
  const Body = pickCardBody(spec.id);

  // Mirror the in-progress envelope so a refresh / tab-close mid-fill keeps
  // her work; guarded because some contexts throw on storage access.
  useEffect(() => {
    if (!persistKey) return;
    try {
      localStorage.setItem(persistKey, JSON.stringify(env));
    } catch {
      // ignore — persistence is best-effort
    }
  }, [persistKey, env]);

  function clearDraft() {
    if (!persistKey) return;
    try {
      localStorage.removeItem(persistKey);
    } catch {
      // ignore
    }
  }

  function handleField(path: string, value: unknown) {
    setEnv((e) => envelopeReducer(e, { type: "field_change", path, value }));
  }

  function handleExpandStep(step_key: string) {
    setEnv((e) => envelopeReducer(e, { type: "step_expand", step_key }));
  }

  function handleNote(step_key: string) {
    setEnv((e) => envelopeReducer(e, { type: "note_open", step_key }));
  }

  function handleSubmit() {
    clearDraft();
    const finalEnvelope = envelopeReducer(env, { type: "submit" });
    onSubmit(finalEnvelope);
  }

  function handleSkip() {
    clearDraft();
    onSkip(env.event_trace);
  }

  return (
    <div className="overflow-hidden rounded-mk-lg border border-mk-border shadow-mk-sm">
      <div className="h-1 bg-mk-accent" />
      <div className="px-[15px] pb-2 pt-3">
        <div className="mb-1 flex items-center gap-2">
          <span className="text-[12px] font-bold text-mk-accent">工具卡</span>
          <span className="rounded-mk-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">
            {spec.category}
          </span>
        </div>
        <div className="text-[14px] font-extrabold text-mk-ink">{spec.name}</div>
        <div className="mt-[3px] text-[12px] leading-relaxed text-mk-faint">{spec.purpose}</div>
      </div>

      <div className="mk-scroll max-h-[360px] overflow-y-auto px-[15px] pb-3 pt-1">
        <Body card={spec} values={env.field_values} onField={handleField} onExpandStep={handleExpandStep} onNote={handleNote} />
      </div>

      <div className="flex items-center justify-between border-t border-mk-border px-[15px] pb-3.5 pt-2.5">
        <button
          type="button"
          onClick={handleSkip}
          className="cursor-pointer border-0 bg-transparent px-0 py-1.5 font-sans text-[12px] font-semibold text-mk-faint transition hover:text-mk-muted"
        >
          跳过这张卡
        </button>
        <button
          type="button"
          onClick={handleSubmit}
          className="inline-flex items-center gap-1.5 rounded-mk-md bg-mk-accent px-4 py-2.5 font-sans text-[14px] font-bold text-white transition hover:bg-mk-accent-600"
        >
          提交并钉到过程树
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M5 12h14M13 6l6 6-6 6" />
          </svg>
        </button>
      </div>
    </div>
  );
}
