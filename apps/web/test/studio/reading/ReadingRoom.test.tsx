import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";

const SOURCE: MaterialSource = {
  id: "m1", title: "NASA 气候报告", sourceUrl: "", kind: "article", origin: "nasa.gov",
  blocks: [{ id: "b0", text: "全球平均气温持续上升。" }, { id: "b1", text: "因此这项政策必然失败。" }],
  locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0,
  lateralRead: false, isLateralInstrument: false, siftSkipped: false, lateralRelation: "", lateralJudgment: "",
};

// Task 10: ReadingRoom now owns the read-together loop internally
// (`useReadingLoop`), so it needs `projectId` + an `api` slice — this test
// never triggers the loop (no coach send, no card), so a bare object with
// no calls actually made is enough; readTurn is present only to satisfy the
// type, never invoked.
const NOOP_API = {
  readTurn: async function* () {},
  summonCard: async function* () {},
  activateProjectCard: async () => {},
  evaluateCardSelection: async () => {
    throw new Error("not used in this test");
  },
  submitProjectCard: async function* () {},
  skipProjectCard: async () => {},
};

describe("ReadingRoom", () => {
  it("renders the article and returns via back", () => {
    const onBack = vi.fn();
    render(<ReadingRoom projectId="p1" source={SOURCE} onBack={onBack} api={NOOP_API} />);
    // The title shows in both the topbar brand and the article header.
    expect(screen.getAllByText("NASA 气候报告").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/全球平均气温持续上升/)).toBeInTheDocument();
    // The chat starts with the coach greeting and the reading-deck starters.
    expect(screen.getByText(/这条来源可信吗/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/返回工作区/));
    expect(onBack).toHaveBeenCalled();
  });
});
