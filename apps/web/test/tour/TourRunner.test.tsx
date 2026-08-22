import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TourProvider, useTour } from "@/tour/TourProvider";
import { TourRunner } from "@/tour/TourRunner";
import type { TourNavContext, TourSegment } from "@/tour/types";

const nav: TourNavContext = {
  setTab: vi.fn(),
  openCourse: vi.fn(),
  setCoursesSub: vi.fn(),
  openDemoProject: vi.fn(),
  setStudioRoom: vi.fn(),
};

function Harness({ seg }: { seg: TourSegment }) {
  const t = useTour();
  return <button onClick={() => t.play(seg)}>play</button>;
}

function renderTour(seg: TourSegment) {
  return render(
    <TourProvider nav={nav}>
      <Harness seg={seg} />
      <TourRunner />
    </TourProvider>,
  );
}

describe("TourRunner", () => {
  it("renders the 印记 bubble text and advances on 下一步", () => {
    const seg: TourSegment = { id: "s", name: "s", steps: [
      { id: "s0", text: "第一步说明", advance: "next", placement: "center" },
      { id: "s1", text: "第二步说明", advance: "next", placement: "center" },
    ]};
    renderTour(seg);
    fireEvent.click(screen.getByText("play"));
    expect(screen.getByText("第一步说明")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "下一步" }));
    expect(screen.getByText("第二步说明")).toBeInTheDocument();
  });

  it("ends when 结束 is clicked", () => {
    const seg: TourSegment = { id: "s", name: "s", steps: [{ id: "s0", text: "内容", advance: "next", placement: "center" }] };
    renderTour(seg);
    fireEvent.click(screen.getByText("play"));
    fireEvent.click(screen.getByRole("button", { name: "结束" }));
    expect(screen.queryByText("内容")).not.toBeInTheDocument();
  });

  it("advance:action shows a hint and advances when the target is clicked", () => {
    const seg: TourSegment = { id: "s", name: "s", steps: [
      { id: "s0", text: "点它", advance: "action", actionEvent: { selector: "#target", type: "click" }, placement: "center" },
      { id: "s1", text: "完成", advance: "next", placement: "center" },
    ]};
    renderTour(seg);
    // an out-of-tour element to click
    const target = document.createElement("button"); target.id = "target"; document.body.appendChild(target);
    fireEvent.click(screen.getByText("play"));
    expect(screen.getByText("点它")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "下一步" })).not.toBeInTheDocument(); // action steps have no 下一步
    fireEvent.click(target);
    expect(screen.getByText("完成")).toBeInTheDocument();
    document.body.removeChild(target);
  });
});
