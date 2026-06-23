import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { loadRegistry, type CardInstance, type CardSpec } from "@mind-imprint/contracts";
import { CardSheetHost } from "./CardSheetHost";

// Guardrail: a custom (escape-hatch) renderer must still produce the standard
// envelope. We drive belief-spectrum (which uses BeliefSpectrumRenderer) through
// CardSheetHost and assert the emitted CardInstance is indistinguishable in
// shape from a schema-rendered card's.
const spec = loadRegistry()["belief-spectrum"] as CardSpec;
const specFieldKeys = spec.steps.flatMap((s) => s.fields.map((f) => f.key));
const selfField = spec.steps[0]!.fields.find((f) => f.key === "self") as { stops: string[] };

const instance: CardInstance = {
  id: "ci-bs", card_id: "belief-spectrum", task_id: "task-1", parent_node_id: null, status: "active",
  field_values: {}, event_trace: [], rubric_tags: [], created_at: "2024-01-01T00:00:00.000Z", completed_at: null,
};

describe("CardSheetHost + custom renderer (guardrail)", () => {
  it("emits the standard envelope from BeliefSpectrumRenderer", () => {
    const onSubmit = vi.fn();
    render(<CardSheetHost cardInstance={instance} spec={spec} onSubmit={onSubmit} onClose={vi.fn()} />);

    // Interact through the bespoke shared axis (我 row, no stances yet).
    fireEvent.click(screen.getByRole("radio", { name: selfField.stops[2] }));
    fireEvent.click(screen.getByText("提交并钉到过程树"));

    expect(onSubmit).toHaveBeenCalledOnce();
    const final = onSubmit.mock.calls[0]![1] as CardInstance;

    // Standard envelope: field_change recorded, then submit; status completed.
    expect(final.event_trace.some((e) => e.kind === "field_change" && (e as { path: string }).path === "self")).toBe(true);
    expect(final.event_trace.at(-1)!.kind).toBe("submit");
    expect(final.status).toBe("completed");

    // Standard data: value is the stop index; only spec field keys are written.
    expect(final.field_values.self).toBe(2);
    for (const key of Object.keys(final.field_values)) {
      expect(specFieldKeys, `unexpected field key "${key}"`).toContain(key);
    }
  });
});
