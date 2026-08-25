import { describe, it, expect, vi, beforeEach } from "vitest";
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

  // 2026-08-25 edge-case findings: a mid-fill refresh used to wipe the card
  // (field values lived only in local state). With persistKey the in-progress
  // envelope is mirrored to localStorage so it survives, and is cleared once
  // the card leaves the screen (submit/skip).
  describe("persistKey (mid-fill draft survival)", () => {
    const KEY = "mi:carddraft:proj-1:craap";
    beforeEach(() => localStorage.clear());

    it("mirrors the in-progress envelope to localStorage under persistKey", () => {
      render(<StudioCardSheet spec={spec} persistKey={KEY} onSubmit={() => {}} onSkip={() => {}} />);
      const raw = localStorage.getItem(KEY);
      expect(raw).toBeTruthy();
      expect(JSON.parse(raw!).card_id).toBe("craap");
    });

    it("restores a saved draft that belongs to the same card", () => {
      localStorage.setItem(
        KEY,
        JSON.stringify({
          id: "ci_seed", card_id: "craap", task_id: "", parent_node_id: null,
          status: "active", field_values: { restored: "yes" }, event_trace: [],
          rubric_tags: [], anchors: [], created_at: "2026-08-25T00:00:00.000Z", completed_at: null,
        }),
      );
      const onSubmit = vi.fn();
      render(<StudioCardSheet spec={spec} persistKey={KEY} onSubmit={onSubmit} onSkip={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: /提交/ }));
      expect(onSubmit.mock.calls[0][0].field_values.restored).toBe("yes");
    });

    it("ignores a saved draft from a DIFFERENT card (no cross-card bleed)", () => {
      localStorage.setItem(
        KEY,
        JSON.stringify({
          id: "ci_other", card_id: "concession", task_id: "", parent_node_id: null,
          status: "active", field_values: { restored: "should-not-load" }, event_trace: [],
          rubric_tags: [], anchors: [], created_at: "2026-08-25T00:00:00.000Z", completed_at: null,
        }),
      );
      const onSubmit = vi.fn();
      render(<StudioCardSheet spec={spec} persistKey={KEY} onSubmit={onSubmit} onSkip={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: /提交/ }));
      expect(onSubmit.mock.calls[0][0].field_values.restored).toBeUndefined();
    });

    it("clears the persisted draft on submit", () => {
      render(<StudioCardSheet spec={spec} persistKey={KEY} onSubmit={() => {}} onSkip={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: /提交/ }));
      expect(localStorage.getItem(KEY)).toBeNull();
    });

    it("clears the persisted draft on skip", () => {
      render(<StudioCardSheet spec={spec} persistKey={KEY} onSubmit={() => {}} onSkip={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
      expect(localStorage.getItem(KEY)).toBeNull();
    });
  });
});
