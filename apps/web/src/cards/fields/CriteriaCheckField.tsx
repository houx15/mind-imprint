import type { FieldProps } from "./types";
import type { z } from "zod";
import type { CriteriaCheckField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function CriteriaCheckField({ field, value, onChange }: FieldProps<F>) {
  const current: number[] = Array.isArray(value) ? (value as number[]) : [];
  const levelAt = (ci: number) => (typeof current[ci] === "number" ? current[ci]! : -1);

  function setLevel(ci: number, li: number) {
    const next = field.criteria.map((_, i) => (i === ci ? li : levelAt(i)));
    onChange(next);
  }

  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-3 space-y-2">
        {field.criteria.map((crit, ci) => (
          <div key={ci} className="flex items-center justify-between gap-3 rounded-[10px] border border-mk-border-2 px-3 py-2">
            <span className="text-[13px] text-mk-ink">{crit}</span>
            <div role="radiogroup" aria-label={crit} className="flex gap-1.5">
              {field.levels.map((lv, li) => (
                <button
                  key={li}
                  type="button"
                  role="radio"
                  aria-checked={levelAt(ci) === li}
                  aria-label={`${crit} ${lv}`}
                  onClick={() => setLevel(ci, li)}
                  className={`rounded-[8px] px-2.5 py-1 text-[12px] font-semibold ${levelAt(ci) === li ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#9AA1B0]"}`}
                >
                  {lv}
                </button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
