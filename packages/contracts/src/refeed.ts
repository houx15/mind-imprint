import type { CardSpec } from "./cardSpec";
import type { CardInstance } from "./envelope";

export interface RefeedAnswer { label: string; value: unknown }
export interface RefeedStep { title: string; answers: RefeedAnswer[] }
export interface RefeedPayload {
  card_id: string;
  card_name: string;
  status: "completed" | "skipped";
  steps?: RefeedStep[];
}

function isEmpty(v: unknown): boolean {
  return v === undefined || v === null || v === "" || (Array.isArray(v) && v.length === 0);
}

export function serializeCardForRefeed(spec: CardSpec, instance: CardInstance): RefeedPayload {
  const card_name = spec.name;
  if (instance.status === "skipped") {
    return { card_id: spec.id, card_name, status: "skipped" };
  }
  const steps: RefeedStep[] = [];
  for (const step of spec.steps) {
    const stepValues = (instance.field_values[step.key] ?? {}) as Record<string, unknown>;
    const answers: RefeedAnswer[] = [];
    for (const field of step.fields) {
      const raw = stepValues[field.key];
      if (isEmpty(raw)) continue;
      if (field.type === "repeatable_group") {
        // remap each row's keys to the item-field labels
        const rows = (raw as Array<Record<string, unknown>>).map((row) => {
          const out: Record<string, unknown> = {};
          for (const item of field.item_fields) {
            if (!isEmpty(row[item.key])) out[item.label] = row[item.key];
          }
          return out;
        }).filter((r) => Object.keys(r).length > 0);
        if (rows.length > 0) answers.push({ label: field.label, value: rows });
      } else {
        answers.push({ label: field.label, value: raw });
      }
    }
    if (answers.length > 0) steps.push({ title: step.title, answers });
  }
  return { card_id: spec.id, card_name, status: "completed", steps };
}
