import { useMemo, useState } from "react";
import { CARD_REGISTRY, type CardInstance } from "@mind-imprint/contracts";
import { envelopeReducer, newEnvelope, type ReducerAction } from "../cards/envelopeReducer";
import { ProposalBubble } from "../cards/states/ProposalBubble";
import { ActiveSheet } from "../cards/states/ActiveSheet";
import { CompletedCard } from "../cards/states/CompletedCard";
import { PHOEBE_VALUES } from "./fixtures";

const reg = CARD_REGISTRY;
const cardIds = Object.keys(reg);

// Group cards by category for <optgroup> rendering
const cardsByCategory: Record<string, { id: string; name: string }[]> = {};
for (const spec of Object.values(reg)) {
  const bucket = cardsByCategory[spec.category] ?? [];
  bucket.push({ id: spec.id, name: spec.name });
  cardsByCategory[spec.category] = bucket;
}

export function Harness() {
  const [cardId, setCardId] = useState(cardIds[0]!);
  const card = reg[cardId]!;
  const [env, setEnv] = useState<CardInstance>(() => ({
    ...newEnvelope(cardId, "t_demo"),
    field_values: PHOEBE_VALUES[cardId] ?? {},
  }));

  const dispatch = (a: ReducerAction) => setEnv((e) => envelopeReducer(e, a));
  const reset = (id: string) => {
    setCardId(id);
    setEnv({ ...newEnvelope(id, "t_demo"), field_values: PHOEBE_VALUES[id] ?? {} });
  };
  const filledCount = useMemo(() => Object.keys(env.field_values).length, [env]);

  return (
    <div className="relative min-h-screen bg-mk-bg p-8 font-sans text-mk-ink">
      <div className="mx-auto max-w-[760px] space-y-5">
        <div className="flex flex-wrap items-center gap-2">
          <label htmlFor="card-picker" className="text-[13px] font-semibold text-mk-muted-2 sr-only">
            选择工具卡
          </label>
          <select
            id="card-picker"
            aria-label="选择工具卡"
            value={cardId}
            onChange={(e) => reset(e.target.value)}
            className="rounded-full border border-mk-border bg-white px-3 py-1.5 text-[13px] font-semibold text-mk-ink"
          >
            {Object.entries(cardsByCategory).map(([category, cards]) => (
              <optgroup key={category} label={category}>
                {cards.map(({ id, name }) => (
                  <option key={id} value={id}>
                    {name}
                  </option>
                ))}
              </optgroup>
            ))}
          </select>

          {cardIds.map((id) => (
            <button
              key={id}
              onClick={() => reset(id)}
              className={`rounded-full px-3 py-1.5 text-[13px] font-semibold ${id === cardId ? "bg-mk-primary text-white" : "bg-white text-mk-muted-2"}`}
            >
              {reg[id]!.name}
            </button>
          ))}
        </div>

        {card.body_status === "stub" && (
          <p className="text-[12px] text-mk-muted-2">
            占位 · 富交互待上线 · {card.interaction_type}
          </p>
        )}

        {env.status === "proposed" && (
          <ProposalBubble
            status="proposed"
            category={card.category}
            name={card.name}
            nudge="这儿先别急着写，我们用这张卡先想一步？"
            onOpen={() => dispatch({ type: "activate" })}
            onSkip={() => dispatch({ type: "skip" })}
          />
        )}
        {env.status === "skipped" && (
          <ProposalBubble status="skipped" category={card.category} name={card.name} nudge=""
            onOpen={() => dispatch({ type: "activate" })} onSkip={() => {}} />
        )}
        {env.status === "completed" && <CompletedCard card={card} filledCount={filledCount} />}

        <pre data-testid="envelope-json" className="overflow-auto rounded-mk border border-mk-border bg-white p-4 text-[12px] text-mk-ink">
          {JSON.stringify(env, null, 2)}
        </pre>
      </div>

      {env.status === "active" && (
        <ActiveSheet
          card={card}
          values={env.field_values}
          onField={(path, value) => dispatch({ type: "field_change", path, value })}
          onExpandStep={(step_key) => dispatch({ type: "step_expand", step_key })}
          onNoteOpen={(step_key) => dispatch({ type: "note_open", step_key })}
          onSubmit={() => dispatch({ type: "submit" })}
          // S1 has no persistence/draft state, so closing the sheet finalizes the envelope.
          onClose={() => dispatch({ type: "submit" })}
        />
      )}
    </div>
  );
}
