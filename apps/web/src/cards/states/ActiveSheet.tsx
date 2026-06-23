import type { CardSpec } from "@mind-imprint/contracts";
import { pickCardBody } from "../customRenderers";

type Props = {
  card: CardSpec;
  values: Record<string, unknown>;
  onField: (path: string, value: unknown) => void;
  onExpandStep: (stepKey: string) => void;
  onNoteOpen: (stepKey: string) => void;
  onSubmit: () => void;
  onClose: () => void;
};

export function ActiveSheet({ card, values, onField, onExpandStep, onNoteOpen, onSubmit, onClose }: Props) {
  const Body = pickCardBody(card.id);
  return (
    <div className="absolute inset-0 z-40 flex flex-col justify-end">
      <div onClick={onClose} className="absolute inset-0 bg-[rgba(22,28,46,0.40)]" aria-hidden />
      <div className="relative mx-auto flex h-[80%] w-full max-w-[880px] flex-col overflow-hidden rounded-[22px_22px_0_0] bg-white shadow-2xl">
        <div className="h-1 flex-none bg-mk-accent" />
        <div className="flex flex-none items-start gap-3.5 border-b border-[#F0F1F5] px-6 py-4">
          <div className="min-w-0 flex-1">
            <div className="mb-1.5 flex items-center gap-2.5">
              <span className="text-[11px] font-bold tracking-wide text-mk-accent">现在轮到你想</span>
              <span className="rounded-full bg-mk-primary-tint px-2.5 py-0.5 text-[11px] font-semibold text-mk-primary">{card.category}</span>
            </div>
            <div className="text-lg font-bold text-mk-ink">{card.name}</div>
            <div className="mt-1 text-[13px] text-mk-muted-2">{card.purpose}</div>
          </div>
          <div className="flex items-center gap-2">
            <button type="button" aria-label="关闭" onClick={onClose} className="h-8 w-8 rounded-[9px] text-mk-muted-2">✕</button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto bg-[#FAFBFC] px-6 py-5">
          <Body card={card} values={values} onField={onField} onExpandStep={onExpandStep} onNote={onNoteOpen} />
        </div>

        <div className="flex-none border-t border-[#F0F1F5] px-6 py-3.5">
          <button type="button" onClick={onSubmit} className="rounded-[12px] bg-mk-primary px-6 py-3 text-sm font-bold text-white">提交</button>
        </div>
      </div>
    </div>
  );
}
