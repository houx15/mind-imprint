import type { FieldProps } from "./types";
import type { z } from "zod";
import type { SpectrumField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function SpectrumField({ field, value, onChange }: FieldProps<F>) {
  const max = field.stops.length - 1;
  const index = typeof value === "number" ? value : -1;
  const clamp = (n: number) => Math.max(0, Math.min(max, n));

  function onKeyDown(e: React.KeyboardEvent) {
    const from = index < 0 ? 0 : index;
    if (e.key === "ArrowRight" || e.key === "ArrowUp") { e.preventDefault(); onChange(clamp(from + 1)); }
    else if (e.key === "ArrowLeft" || e.key === "ArrowDown") { e.preventDefault(); onChange(clamp(from - 1)); }
    else if (e.key === "Home") { e.preventDefault(); onChange(0); }
    else if (e.key === "End") { e.preventDefault(); onChange(max); }
  }

  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div
        role="radiogroup"
        aria-label={field.label}
        tabIndex={0}
        onKeyDown={onKeyDown}
        className="relative mt-3 flex items-start outline-none"
      >
        {/* connecting track behind the dots */}
        <div className="pointer-events-none absolute left-[10%] right-[10%] top-[7px] h-[2px] bg-[#EEF0F4]" aria-hidden="true" />
        {field.stops.map((stop, i) => (
          <button
            key={i}
            type="button"
            role="radio"
            aria-checked={index === i}
            aria-label={stop}
            tabIndex={-1}
            onClick={() => onChange(i)}
            className="relative z-[1] flex flex-1 flex-col items-center gap-1.5 bg-transparent"
          >
            <span className={`h-3.5 w-3.5 rounded-full ${index === i ? "bg-mk-primary" : "bg-[#D9DDE7]"}`} />
            <span className={`text-center text-[11.5px] leading-tight ${index === i ? "font-semibold text-mk-ink" : "text-[#9AA1B0]"}`}>{stop}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
