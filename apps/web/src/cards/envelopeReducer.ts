import type { CardInstance, TraceEvent } from "@mind-imprint/contracts";

export type ReducerAction =
  | { type: "activate" }
  | { type: "field_change"; path: string; value: unknown }
  | { type: "step_expand"; step_key: string }
  | { type: "note_open"; step_key: string }
  | { type: "skip" }
  | { type: "submit" };

type Now = () => string;
const defaultNow: Now = () => new Date().toISOString();

let seq = 0;
export function newEnvelope(card_id: string, task_id: string, now: Now = defaultNow): CardInstance {
  return {
    id: `ci_${++seq}`,
    card_id, task_id, parent_node_id: null,
    status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
    created_at: now(), completed_at: null,
  };
}

function setPath(obj: Record<string, unknown>, path: string, value: unknown): Record<string, unknown> {
  const tokens = path.replace(/\[(\d+)\]/g, ".$1").split(".");
  const root: any = Array.isArray(obj) ? [...obj] : { ...obj };
  let cur: any = root;
  for (let i = 0; i < tokens.length - 1; i++) {
    const t = tokens[i]!;
    const existing = cur[t];
    cur[t] = Array.isArray(existing) ? [...existing] : { ...(existing ?? {}) };
    cur = cur[t];
  }
  cur[tokens[tokens.length - 1]!] = value;
  return root;
}

function append(env: CardInstance, event: TraceEvent): CardInstance {
  return { ...env, event_trace: [...env.event_trace, event] };
}

export function envelopeReducer(env: CardInstance, action: ReducerAction, now: Now = defaultNow): CardInstance {
  switch (action.type) {
    case "activate":
      return { ...env, status: "active" };
    case "field_change":
      return append(
        { ...env, field_values: setPath(env.field_values, action.path, action.value) },
        { kind: "field_change", path: action.path, at: now() },
      );
    case "step_expand":
      return append(env, { kind: "step_expand", step_key: action.step_key, at: now() });
    case "note_open":
      return append(env, { kind: "note_open", step_key: action.step_key, at: now() });
    case "skip":
      return append({ ...env, status: "skipped" }, { kind: "skip", at: now() });
    case "submit":
      return append({ ...env, status: "completed", completed_at: now() }, { kind: "submit", at: now() });
  }
}
