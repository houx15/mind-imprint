import type { FieldProps } from "./types";
import type { z } from "zod";
import type { SingleChoiceField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function SingleChoiceField({ field, value, onChange }: FieldProps<F>) {
  return (
    <div>
      <span className="block text-[14px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex flex-wrap gap-2">
        {field.options.map((opt) => {
          const active = value === opt;
          return (
            <button
              key={opt}
              type="button"
              onClick={() => onChange(opt)}
              aria-pressed={active}
              className={`rounded-full px-3 py-1.5 text-[14px] font-semibold ${active ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#6B7384]"}`}
            >
              {opt}
            </button>
          );
        })}
      </div>
    </div>
  );
}
