import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TourProvider, useTour } from "@/tour/TourProvider";
import type { TourNavContext, TourSegment } from "@/tour/types";

const nav: TourNavContext = {
  setTab: vi.fn(),
  openCourse: vi.fn(),
  setCoursesSub: vi.fn(),
  openDemoProject: vi.fn(),
  setStudioRoom: vi.fn(),
  openDemoReport: vi.fn(),
  setReadingView: vi.fn(),
  openDemoReadingRoom: vi.fn(),
  setWritingView: vi.fn(),
};

const seg = (id: string, n: number): TourSegment => ({
  id, name: id,
  steps: Array.from({ length: n }, (_, i) => ({ id: `${id}-${i}`, text: `${id} ${i}`, advance: "next" as const })),
});

function Probe() {
  const t = useTour();
  return (
    <div>
      <span data-testid="state">{t.running ? `${t.segment?.id}:${t.stepIndex}` : "idle"}</span>
      <button onClick={() => t.play([seg("a", 2), seg("b", 1)])}>play</button>
      <button onClick={t.next}>next</button>
      <button onClick={t.prev}>prev</button>
      <button onClick={t.skipSegment}>skip</button>
      <button onClick={t.stop}>stop</button>
    </div>
  );
}

describe("TourProvider", () => {
  it("plays, advances across segments, and completes", () => {
    const onComplete = vi.fn();
    render(<TourProvider nav={nav} onComplete={onComplete}><Probe /></TourProvider>);
    expect(screen.getByTestId("state").textContent).toBe("idle");

    fireEvent.click(screen.getByText("play"));
    expect(screen.getByTestId("state").textContent).toBe("a:0");
    fireEvent.click(screen.getByText("next"));  // a:1
    expect(screen.getByTestId("state").textContent).toBe("a:1");
    fireEvent.click(screen.getByText("next"));  // rolls into segment b
    expect(screen.getByTestId("state").textContent).toBe("b:0");
    fireEvent.click(screen.getByText("next"));  // past last step → complete
    expect(screen.getByTestId("state").textContent).toBe("idle");
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("calls onEnter with the nav context when a step becomes active", () => {
    const onEnter = vi.fn();
    const s: TourSegment = { id: "s", name: "s", steps: [{ id: "s0", text: "hi", advance: "next", onEnter }] };
    function P() { const t = useTour(); return <button onClick={() => t.play(s)}>go</button>; }
    render(<TourProvider nav={nav}><P /></TourProvider>);
    fireEvent.click(screen.getByText("go"));
    expect(onEnter).toHaveBeenCalledWith(nav);
  });

  it("stop() ends the tour and fires onComplete once", () => {
    const onComplete = vi.fn();
    render(<TourProvider nav={nav} onComplete={onComplete}><Probe /></TourProvider>);
    fireEvent.click(screen.getByText("play"));
    fireEvent.click(screen.getByText("stop"));
    expect(screen.getByTestId("state").textContent).toBe("idle");
    expect(onComplete).toHaveBeenCalledTimes(1);
  });
});
