import { describe, it, expect } from "vitest";
import { Skill } from "../src/skill";
import course from "../skills/info-literacy-course.json";

describe("info-literacy-course challenge phase", () => {
  it("parses with a challenge phase carrying an inline challenge block", () => {
    const parsed = Skill.parse(course);
    const ch = parsed.contracts["challenge"];
    expect(ch).toBeDefined();
    expect(ch!.requires).toEqual(["reflect"]);
    expect(ch!.floor ?? []).toEqual([]);
    expect(ch!.challenge?.anchors.length).toBe(3);
  });
});
