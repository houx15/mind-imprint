import { z } from "zod";

export const ShowIf = z.object({ key: z.string().min(1), equals: z.string() });

const base = { key: z.string().min(1), label: z.string().min(1), show_if: ShowIf.optional() };

export const TextField = z.object({ type: z.literal("text"), ...base });
export const TextAreaField = z.object({ type: z.literal("textarea"), ...base, rows: z.number().int().positive().optional() });
export const SingleChoiceField = z.object({ type: z.literal("single_choice"), ...base, options: z.array(z.string()).min(1) });
export const MultiChoiceField = z.object({ type: z.literal("multi_choice"), ...base, options: z.array(z.string()).min(1) });
export const RatingField = z.object({ type: z.literal("rating"), ...base, scale: z.number().int().positive() });
export const LinkCheckField = z.object({ type: z.literal("link_check"), ...base });
export const SpectrumField = z.object({ type: z.literal("spectrum"), ...base, stops: z.array(z.string()).min(2) });
export const CriteriaCheckField = z.object({
  type: z.literal("criteria_check"),
  ...base,
  criteria: z.array(z.string()).min(2),
  levels: z.array(z.string()).min(2),
});

// item_fields cannot themselves be repeatable (no nesting)
export const ItemField = z.discriminatedUnion("type", [
  TextField, TextAreaField, SingleChoiceField, MultiChoiceField, RatingField, LinkCheckField, SpectrumField,
]);

export const RepeatableGroupField = z.object({
  type: z.literal("repeatable_group"),
  ...base,
  item_fields: z.array(ItemField).min(1),
});

export const FieldPrimitive = z.discriminatedUnion("type", [
  TextField, TextAreaField, SingleChoiceField, MultiChoiceField, RatingField, LinkCheckField, SpectrumField, CriteriaCheckField, RepeatableGroupField,
]);

export type FieldPrimitive = z.infer<typeof FieldPrimitive>;
export type ItemField = z.infer<typeof ItemField>;
export type FieldType = FieldPrimitive["type"];
