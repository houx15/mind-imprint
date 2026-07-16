import { describe, it, expect } from "vitest";
import { Skill, Contract, Gate, FloorItem } from "../src/skill";
import courseSkill from "../skills/info-literacy-course.json";
import projectSkill from "../skills/writing-project.json";

describe("skill format (C5)", () => {
  it("a writing-project fixture with kind, contracts, and gate parses", () => {
    const skill = {
      id: "writing-project",
      kind: "project",
      contracts: {
        outline: {
          requires: [],
          produces: ["outline"],
          view: "OutlineView",
          repertoire: ["tree", "order"],
          gate: {
            machine: ["grammar"],
            student_written: ["thesis"],
            human: ["peer_review"],
          },
        },
      },
      intake: { topic: "string" },
      vocabulary: "academic-voice",
      cards: ["sift_craap", "toulmin"],
    };
    expect(Skill.safeParse(skill).success).toBe(true);
  });
  it("a contract missing gate fails", () => {
    const badContract = {
      requires: [],
      produces: [],
      view: "View",
      repertoire: [],
      // gate missing
    };
    expect(Contract.safeParse(badContract).success).toBe(false);
  });
  it("gate provides defaults for empty arrays", () => {
    const gate = Gate.safeParse({ machine: [], student_written: [], human: [] });
    expect(gate.success).toBe(true);
  });
  it("gate allows omitted fields with defaults", () => {
    const gate = Gate.safeParse({});
    expect(gate.success).toBe(true);
    if (gate.success) {
      expect(gate.data.machine).toEqual([]);
      expect(gate.data.student_written).toEqual([]);
      expect(gate.data.human).toEqual([]);
    }
  });
});

describe("course skill", () => {
  it("parses the authored course skill", () => {
    const r = Skill.safeParse(courseSkill);
    expect(r.success).toBe(true);
  });

  it("still parses the project skill unchanged", () => {
    expect(Skill.safeParse(projectSkill).success).toBe(true);
  });

  it("rejects a floor kind outside the closed set", () => {
    expect(FloorItem.safeParse({ kind: "vibes_ok" }).success).toBe(false);
  });
});
