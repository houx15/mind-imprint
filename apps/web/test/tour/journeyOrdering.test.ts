import { describe, it, expect } from "vitest";
import { journeyStarting } from "@/tour/journey";
import { coursesSegments } from "@/tour/segments/courses";
import { projectsSegments } from "@/tour/segments/projects";
import { settingsSegment } from "@/tour/segments/settings";

describe("journeyStarting", () => {
  it("courses-first: courses, then projects, then settings finale", () => {
    const j = journeyStarting("courses");
    expect(j[0]!.id).toBe(coursesSegments[0]!.id);
    expect(j.some((s) => s.id === projectsSegments[0]!.id)).toBe(true);
    expect(j[j.length - 1]!.id).toBe(settingsSegment.id);
  });
  it("projects-first: projects, then courses, then settings finale", () => {
    const j = journeyStarting("projects");
    expect(j[0]!.id).toBe(projectsSegments[0]!.id);
    expect(j[j.length - 1]!.id).toBe(settingsSegment.id);
    // courses group present after projects
    const ci = j.findIndex((s) => s.id === coursesSegments[0]!.id);
    const pi = j.findIndex((s) => s.id === projectsSegments[0]!.id);
    expect(pi).toBeLessThan(ci);
  });
});
