import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { StudioCardSheet } from "@/studio/StudioCardSheet";

const spec = CARD_REGISTRY["craap"]!;

describe("StudioCardSheet", () => {
  it("renders the schema-driven card body (reusing pickCardBody/CardRenderer, not a craap special-case)", () => {
    render(<StudioCardSheet spec={spec} onSubmit={() => {}} onSkip={() => {}} />);
    expect(screen.getByText(spec.name)).toBeInTheDocument();
    expect(screen.getAllByText(/权威性|来源/).length).toBeGreaterThan(0);
  });

  it("submit fires onSubmit with a completed envelope (via envelopeReducer)", () => {
    const onSubmit = vi.fn();
    render(<StudioCardSheet spec={spec} onSubmit={onSubmit} onSkip={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: /提交/ }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
    const finalEnvelope = onSubmit.mock.calls[0][0];
    expect(finalEnvelope.status).toBe("completed");
    expect(finalEnvelope.card_id).toBe("craap");
  });

  it("skip fires onSkip with the event trace accumulated so far", () => {
    const onSkip = vi.fn();
    render(<StudioCardSheet spec={spec} onSubmit={() => {}} onSkip={onSkip} />);
    fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
    expect(onSkip).toHaveBeenCalledTimes(1);
    expect(Array.isArray(onSkip.mock.calls[0][0])).toBe(true);
  });
});
