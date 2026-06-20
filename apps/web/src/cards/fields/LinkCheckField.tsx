import type { FieldProps } from "./types";
import type { z } from "zod";
import type { LinkCheckField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function LinkCheckField({ field, value, onChange }: FieldProps<F>) {
  const url = (value as string) ?? "";
  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex flex-wrap items-center gap-2.5">
        <input
          aria-label={field.label}
          value={url}
          onChange={(e) => onChange(e.target.value)}
          placeholder="贴链接"
          className="min-w-[200px] flex-1 rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-[13.5px] text-mk-ink outline-none"
        />
        {url.trim() !== "" && (
          <>
            <span className="text-[13px] text-mk-ink break-all">{url}</span>
            <span className="rounded-[9px] bg-mk-green-tint px-3 py-2 text-[12.5px] font-semibold text-mk-green">已溯源</span>
          </>
        )}
      </div>
    </div>
  );
}
