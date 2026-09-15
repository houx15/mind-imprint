import { describe, expect, it } from "vitest";
import { cardShowsProse } from "./classWeeklyLogic";

// Three students: u1 has both cards, u2 watch only, u3 praise only.
const praise = [{ userId: "u1" }, { userId: "u3" }];
const watch = [{ userId: "u1" }, { userId: "u2" }];
const watchIds = new Set(watch.map((c) => c.userId));

const shownOn = (kind: string, cards: { userId: string }[]) =>
  cards.filter((c) => cardShowsProse(kind, c.userId, watchIds)).map((c) => c.userId);

describe("cardShowsProse", () => {
  it("shows a student with both cards only under 需要建议", () => {
    expect(shownOn("watch", watch)).toContain("u1");
    expect(shownOn("praise", praise)).not.toContain("u1");
  });

  it("shows a watch-only student under 需要建议", () => expect(shownOn("watch", watch)).toContain("u2"));

  it("shows a praise-only student under 值得表扬", () => expect(shownOn("praise", praise)).toEqual(["u3"]));

  it("shows each student's lead and action exactly once", () => {
    const all = [...shownOn("praise", praise), ...shownOn("watch", watch)].sort();
    expect(all).toEqual(["u1", "u2", "u3"]);
  });
});
