import { describe, expect, it } from "vitest";
import { toggleAudienceRole, type AudienceDocument } from "./audienceBoard";

describe("audience selection preserves work", () => {
  it("restores the same board after deselection and a saved-document round trip", () => {
    const original: AudienceDocument = { step: "roles", activeBoardId: "teacher", boards: [{ id: "teacher", role: "teacher", person: "art teacher", ageRange: "unknown", hobbies: ["animation"], interests: ["process"], offerings: ["sketchbook"] }] };
    const removed = toggleAudienceRole(original, "teacher", () => "unused");
    expect(removed.boards).toEqual([]);
    expect(removed.activeBoardId).toBe("");
    const restored = toggleAudienceRole(JSON.parse(JSON.stringify(removed)), "teacher", () => "wrong-new-id");
    expect(restored.boards).toEqual(original.boards);
    expect(restored.archivedBoards).toEqual([]);
    expect(original.boards).toHaveLength(1);
  });
  it("does not discard old drafts when capacity is reached", () => {
    const board = (id: string) => ({ id, role: id, person: "", ageRange: "", interests: [], offerings: [] });
    const doc: AudienceDocument = { step: "roles", activeBoardId: "active", boards: [board("active")], archivedBoards: Array.from({ length: 12 }, (_, i) => board(String(i))) };
    expect(toggleAudienceRole(doc, "active", () => "unused")).toBe(doc);
  });
});
