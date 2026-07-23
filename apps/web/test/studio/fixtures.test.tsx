import { describe, it, expect } from "vitest";
import { STUDIO_FIXTURE } from "@/studio/fixtures";

describe("STUDIO_FIXTURE", () => {
  it("has 7 stations S0-S6 in order with exactly one current", () => {
    const codes = STUDIO_FIXTURE.stations.map((s) => s.code);
    expect(codes).toEqual(["S0", "S1", "S2", "S3", "S4", "S5", "S6"]);
    expect(STUDIO_FIXTURE.stations.filter((s) => s.state === "current")).toHaveLength(1);
  });
  it("the active station resolves to a station with a view", () => {
    const active = STUDIO_FIXTURE.stations.find((s) => s.code === STUDIO_FIXTURE.activeStation);
    expect(active).toBeTruthy();
    expect(active!.view).toBeTruthy();
  });
});
