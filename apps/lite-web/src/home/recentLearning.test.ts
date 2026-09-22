import { describe, expect, it } from "vitest";
import { recentLearning } from "./recentLearning";
import type { Reading } from "../api/readings";
import type { Writing } from "../api/writings";
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
describe("learning home activity projection", () => {
  it("sorts actual activity across collections and preserves distinct IDs", () => {
    const rows = recentLearning([reading("same")], [writing("same")]);
    expect(rows.map((r) => r.key)).toEqual(["writing:same", "reading:same"]);
    expect(rows.map((r) => r.path)).toEqual([
      "/writings/same",
      "/readings/same",
    ]);
  });
  it("excludes both legacy finish signals", () => {
    expect(
      recentLearning(
        [
          reading("done", { status: "finished" }),
          reading("legacy", { finishedAt: "2026-09-12" }),
        ],
        [writing("done", { status: "finished" })],
      ).map((r) => r.key),
    ).toEqual([]);
    expect(
      recentLearning(
        [],
        [writing("1"), writing("2"), writing("3"), writing("4")],
      ),
    ).toHaveLength(3);
  });
  it("encodes IDs and handles missing dates and titles", () => {
    const row = recentLearning(
      [reading("a/b", { title: "", lastActivityAt: "", updatedAt: "invalid" })],
      [],
    );
    expect(row[0]?.path).toBe("/readings/a%2Fb");
    expect(row[0]?.title).toBe("未命名阅读");
  });
  // 🚨 2026-09-21 项目那一格从底栏藏掉了（产品负责人：还没做完）。
  // 这条钉住「首页也不再请她进去」——底栏没有、首页却还列着项目，
  // 点下去是一个她不该看到的半成品。
  //
  // 钉的是**投影**，不是数据：项目本身一条没动，路由也还在，
  // 手上正做着一个项目的人输网址仍然进得去。
  it("no longer surfaces projects at all", () => {
    const rows = recentLearning([reading("r")], [writing("w")]);
    expect(rows.some((r) => r.kind === "项目")).toBe(false);
    expect(rows.some((r) => r.path.startsWith("/projects"))).toBe(false);
  });
});
