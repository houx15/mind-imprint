import { describe, expect, it } from "vitest";
import {
  groupByStatus,
  projectTitle,
  PROJECT_STATUSES,
  PROJECT_STATUS_LABELS,
  PROJECT_KINDS,
  PROJECT_KIND_LABELS,
  type Project,
} from "./projects";

function p(id: string, over: Partial<Project> = {}): Project {
  return {
    id,
    idea: "我想弄明白我们学校的剩饭到底去哪了",
    kind: "research",
    name: "",
    coverGround: "",
    coverGlyph: "",
    status: "talking",
    createdAt: "2026-09-01T00:00:00Z",
    lastActivityAt: "2026-09-01T00:00:00Z",
    ...over,
  };
}

describe("groupByStatus", () => {
  it("returns every column even when empty, so the board keeps its shape", () => {
    const got = groupByStatus([]);
    expect(Object.keys(got)).toEqual([...PROJECT_STATUSES]);
    for (const s of PROJECT_STATUSES) expect(got[s]).toEqual([]);
  });

  it("files each project under its own status and preserves input order", () => {
    const got = groupByStatus([
      p("a", { status: "running" }),
      p("b", { status: "talking" }),
      p("c", { status: "running" }),
    ]);
    expect(got.running.map((x) => x.id)).toEqual(["a", "c"]);
    expect(got.talking.map((x) => x.id)).toEqual(["b"]);
    expect(got.archived).toEqual([]);
  });

  it("drops an unknown status rather than throwing — a new server status must not blank the board", () => {
    const rogue = p("z", { status: "done" as Project["status"] });
    expect(() => groupByStatus([rogue])).not.toThrow();
    expect(Object.values(groupByStatus([rogue])).flat()).toEqual([]);
  });
});

describe("projectTitle", () => {
  it("uses her name once she has given one", () => {
    expect(projectTitle(p("a", { name: "校园植物图鉴" }))).toBe("校园植物图鉴");
  });

  it("falls back to her own sentence, never to a placeholder", () => {
    expect(projectTitle(p("a"))).toBe("我想弄明白我们学校的剩饭到底去哪了");
    expect(projectTitle(p("a", { name: "   " }))).toBe("我想弄明白我们学校的剩饭到底去哪了");
  });
});

// These two guard the same invariant from the frontend side: a status or kind
// the server can send with no label here renders as blank chrome.
describe("labels cover the closed sets", () => {
  it("labels every status", () => {
    for (const s of PROJECT_STATUSES) expect(PROJECT_STATUS_LABELS[s]).toBeTruthy();
    expect(Object.keys(PROJECT_STATUS_LABELS).sort()).toEqual([...PROJECT_STATUSES].sort());
  });

  it("labels every kind", () => {
    for (const k of PROJECT_KINDS) expect(PROJECT_KIND_LABELS[k]).toBeTruthy();
    expect(Object.keys(PROJECT_KIND_LABELS).sort()).toEqual([...PROJECT_KINDS].sort());
  });
});
