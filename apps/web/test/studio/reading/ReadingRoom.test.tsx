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

describe("ReadingRoom", () => {
  it("renders the article and returns via back", () => {
    const onBack = vi.fn();
    render(<ReadingRoom source={SOURCE} onBack={onBack} />);
    expect(screen.getByText("NASA 气候报告")).toBeInTheDocument();
    expect(screen.getByText(/全球平均气温持续上升/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/返回工作区/));
    expect(onBack).toHaveBeenCalled();
  });
});
