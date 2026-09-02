import { describe, expect, it } from "vitest";
import { noteKindMeta, type Note } from "./notes";

function note(id: string, cluster: string): Note {
  return {
    id,
    kind: "observation",
    body: id,
    author: "student",
    edited: false,
    cluster,
    x: 0,
    y: 0,
    createdAt: "2026-09-01T00:00:00Z",
  };
}

describe("noteKindMeta", () => {
  it("has a plain-language label for every kind", () => {
    for (const kind of ["observation", "quote", "assumption", "question", "idea"] as const) {
      expect(noteKindMeta(kind).label).not.toBe("");
      expect(noteKindMeta(kind).kind).toBe(kind);
    }
  });
});
