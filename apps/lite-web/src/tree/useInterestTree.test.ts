import { describe, expect, it } from "vitest";
import { outputCount } from "./useInterestTree";
import type { Keyword, KeywordSource } from "./types";

function source(kind: KeywordSource["kind"], id: string): KeywordSource {
  return { kind, id, label: kind, date: "2026-09-18" };
}

function keyword(id: string, sources: KeywordSource[]): Keyword {
  return {
    id,
    interestId: id,
    text: id,
    en: id,
    field: "self",
    strength: 1,
    bornAt: 0,
    firstSeenAt: "2026-09-18T00:00:00Z",
    note: "",
    sources,
    disciplineIds: [],
    at: { t: 0.5, spread: 40 },
  };
}

describe("outputCount", () => {
  it("counts unique finished readings, writings and projects", () => {
    expect(
      outputCount([
        keyword("a", [source("reading", "r1"), source("project", "p1")]),
        keyword("b", [source("reading", "r1"), source("writing", "w1")]),
      ]),
    ).toBe(3);
  });

  it("does not count quiz or news sources as finished outputs", () => {
    expect(
      outputCount([
        keyword("a", [source("course", "quiz-1")]),
        keyword("b", [source("news", "news-1")]),
      ]),
    ).toBe(0);
  });

  it("ignores a finished-kind source without a real id", () => {
    expect(outputCount([keyword("a", [source("reading", "")])])).toBe(0);
  });
});
