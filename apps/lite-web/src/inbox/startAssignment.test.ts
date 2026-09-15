import { describe, expect, it } from "vitest";
import { roomPathForStart } from "./startAssignment";

describe("roomPathForStart", () => {
  it("reading", () => expect(roomPathForStart({ kind: "reading", atomId: "a1", projectId: null })).toBe("/readings/a1"));
  it("writing", () => expect(roomPathForStart({ kind: "writing", atomId: "a2", projectId: null })).toBe("/writings/a2"));
  it("project uses the project id", () =>
    expect(roomPathForStart({ kind: "project", atomId: "a3", projectId: "p3" })).toBe("/projects/p3"));
});
