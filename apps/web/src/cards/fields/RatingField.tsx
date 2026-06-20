import type { FieldProps } from "./types";
import type { z } from "zod";
import type { RatingField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function RatingField({ field, value, onChange }: FieldProps<F>) {
  const current = typeof value === "number" ? value : 0;
  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex gap-1.5" role="radiogroup" aria-label={field.label}>
        {Array.from({ length: field.scale }, (_, i) => i + 1).map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={current === n}
            aria-label={`${field.label} ${n}`}
            onClick={() => onChange(n)}
            className={`h-8 w-8 rounded-[8px] text-sm font-semibold ${current >= n ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#9AA1B0]"}`}
          >
            {n}
          </button>
        ))}
      </div>
    </div>
  );
}
