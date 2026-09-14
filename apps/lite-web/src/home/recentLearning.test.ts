import { describe, expect, it } from "vitest";
import { recentLearning } from "./recentLearning";
import type { Reading } from "../api/readings";
import type { Writing } from "../api/writings";
import type { Project } from "../api/projects";
const reading = (id: string, extra = {}): Reading => ({
  id,
  title: id,
  lang: "zh",
  status: "active",
  hasSource: true,
  createdAt: "2026-09-01",
  updatedAt: "2026-09-14",
  lastActivityAt: "2026-09-10",
  finishedAt: null,
  ...extra,
});
const writing = (id: string, extra = {}): Writing => ({
  id,
  title: id,
  lang: "zh",
  status: "active",
  stage: "draft",
  targetWords: null,
  structureKey: "",
  setupAt: null,
  createdAt: "2026-09-01",
  updatedAt: "2026-09-14",
  lastActivityAt: "2026-09-11",
  finishedAt: null,
  ...extra,
});
const project = (id: string, extra = {}): Project => ({
  id,
  name: id,
  idea: "",
  kind: "",
  boardAxes: false,
  coverGround: "",
  coverGlyph: "",
  status: "running",
  createdAt: "2026-09-01",
  lastActivityAt: "2026-09-12",
  currentStep: "",
  stepsDone: 0,
  stepsTotal: 0,
  ...extra,
});
describe("learning home activity projection", () => {
  it("sorts actual activity across collections and preserves distinct IDs", () => {
    const rows = recentLearning(
      [reading("same")],
      [writing("same")],
      [project("same")],
    );
    expect(rows.map((r) => r.key)).toEqual([
      "project:same",
      "writing:same",
      "reading:same",
    ]);
    expect(rows.map((r) => r.path)).toEqual([
      "/projects/same",
      "/writings/same",
      "/readings/same",
    ]);
  });
  it("excludes both legacy finish signals and archived projects, retains upkeep", () => {
    expect(
      recentLearning(
        [
          reading("done", { status: "finished" }),
          reading("legacy", { finishedAt: "2026-09-12" }),
        ],
        [writing("done", { status: "finished" })],
        [
          project("archived", { status: "archived" }),
          project("upkeep", { status: "keeping" }),
        ],
      ).map((r) => r.key),
    ).toEqual(["project:upkeep"]);
    expect(
      recentLearning(
        [],
        [],
        [project("1"), project("2"), project("3"), project("4")],
      ),
    ).toHaveLength(3);
  });
  it("encodes IDs and handles missing dates and titles", () => {
    const row = recentLearning(
      [reading("a/b", { title: "", lastActivityAt: "", updatedAt: "invalid" })],
      [],
      [project("p")],
    );
    expect(row[1]?.path).toBe("/readings/a%2Fb");
    expect(row[1]?.title).toBe("未命名阅读");
  });
});
