import type { FieldProps } from "./types";
import type { z } from "zod";
import type { TextAreaField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function TextAreaField({ field, value, onChange }: FieldProps<F>) {
  return (
    <label className="block">
      <span className="block text-[14px] font-semibold text-[#3A4256]">{field.label}</span>
      <textarea
        aria-label={field.label}
        rows={field.rows ?? 2}
        value={(value as string) ?? ""}
        onChange={(e) => onChange(e.target.value)}
        className="mt-2 w-full resize-y rounded-mk-sm border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body leading-relaxed text-mk-ink placeholder:text-mk-faint outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 focus-visible:border-mk-accent transition-colors"
      />
    </label>
  );
}
