import type { FieldProps } from "./types";
import type { z } from "zod";
import type { MultiChoiceField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function MultiChoiceField({ field, value, onChange }: FieldProps<F>) {
  const selected = Array.isArray(value) ? (value as string[]) : [];
  const toggle = (opt: string) =>
    onChange(selected.includes(opt) ? selected.filter((o) => o !== opt) : [...selected, opt]);
  return (
    <div>
      <span className="block text-[14px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex flex-wrap gap-2">
        {field.options.map((opt) => {
          const active = selected.includes(opt);
          return (
            <button
              key={opt}
              type="button"
              aria-pressed={active}
              onClick={() => toggle(opt)}
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
