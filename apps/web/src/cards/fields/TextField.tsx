import type { FieldProps } from "./types";
import type { z } from "zod";
import type { TextField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function TextField({ field, value, onChange }: FieldProps<F>) {
  return (
    <label className="block">
      <span className="block text-[14px] font-semibold text-[#3A4256]">{field.label}</span>
      <input
        aria-label={field.label}
        value={(value as string) ?? ""}
        onChange={(e) => onChange(e.target.value)}
        className="mt-2 w-full rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-sm text-mk-ink outline-none"
      />
    </label>
  );
}
