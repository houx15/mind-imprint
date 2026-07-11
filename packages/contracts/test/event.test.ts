import { describe, it, expect } from "vitest";
import { StudioEvent, Surface, EVENT_TYPES } from "../src/event";

describe("event stream (C4)", () => {
  it("surface is studio|course|chat", () => {
    expect(Surface.safeParse("studio").success).toBe(true);
    expect(Surface.safeParse("nope").success).toBe(false);
  });
  it("card_clicked carries the unprompted flag", () => {
    const ev = StudioEvent.safeParse({
      type: "card_clicked", surface: "studio", card_id: "craap", unprompted: true,
    });
    expect(ev.success).toBe(true);
  });
  it("suggestion_disposition carries action + reason", () => {
    expect(StudioEvent.safeParse({
      type: "suggestion_disposition", surface: "studio", action: "reject", reason: "source is a blog",
    }).success).toBe(true);
  });
  it("enumerates all event types", () => {
    expect(EVENT_TYPES).toEqual(expect.arrayContaining([
      "prompt_sent","card_clicked","gate_attempt","suggestion_disposition","verbalization_submitted",
      "source_opened","citation_added","version_saved","rescue_triggered","stance_change_logged","chat_message",
    ]));
  });
});
