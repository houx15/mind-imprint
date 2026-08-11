import { describe, it, expect } from "vitest";
import { RevisionCheckpointArtifactType, ProposalCheckpointContent } from "./revisionCheckpoint";

describe("revision checkpoint contracts", () => {
  it("accepts a proposal content shape", () => {
    expect(() =>
      ProposalCheckpointContent.parse({
        objective: "x",
        reason: "y",
        activities: "a",
        resources: "s",
        counterpoints: "",
      })
    ).not.toThrow();
  });

  it("rejects an unknown artifact type", () => {
    expect(RevisionCheckpointArtifactType.safeParse("banana").success).toBe(false);
  });
});
