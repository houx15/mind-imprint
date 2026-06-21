import type { FieldProps } from "./types";
import type { z } from "zod";
import type { RepeatableGroupField as Schema, ItemField } from "@mind-imprint/contracts";
import { TextField } from "./TextField";
import { TextAreaField } from "./TextAreaField";
import { SingleChoiceField } from "./SingleChoiceField";
import { MultiChoiceField } from "./MultiChoiceField";
import { RatingField } from "./RatingField";
import { LinkCheckField } from "./LinkCheckField";
import { SpectrumField } from "./SpectrumField";
import type { ComponentType } from "react";

type F = z.infer<typeof Schema>;
type Row = Record<string, unknown>;

const ITEM_COMPONENTS: Record<ItemField["type"], ComponentType<FieldProps<any>>> = {
  text: TextField, textarea: TextAreaField, single_choice: SingleChoiceField,
  multi_choice: MultiChoiceField, rating: RatingField, link_check: LinkCheckField,
  spectrum: SpectrumField,
};

export function RepeatableGroupField({ field, value, onChange }: FieldProps<F>) {
  const rows: Row[] = Array.isArray(value) ? (value as Row[]) : [];
  const update = (next: Row[]) => onChange(next);
  const setCell = (i: number, key: string, v: unknown) =>
    update(rows.map((r, idx) => (idx === i ? { ...r, [key]: v } : r)));

  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 space-y-2.5">
        {rows.map((row, i) => (
          <div key={i} className="space-y-3 rounded-[11px] border border-mk-border-2 bg-white p-3">
            {field.item_fields.map((f) => {
              const Cmp = ITEM_COMPONENTS[f.type];
              return <Cmp key={f.key} field={f} value={row[f.key]} onChange={(v) => setCell(i, f.key, v)} />;
            })}
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={() => update([...rows, {}])}
        className="mt-2.5 w-full rounded-[10px] border border-dashed border-[#CFD4E0] py-2.5 text-[13px] font-semibold text-[#6B7384]"
      >
        + 添加来源
      </button>
    </div>
  );
}
