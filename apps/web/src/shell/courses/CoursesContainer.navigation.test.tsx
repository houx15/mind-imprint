import { StrictMode } from "react";
import { act, render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
vi.mock("./CoursesView", () => ({ CoursesView: () => null }));
vi.mock("./CourseDetail", () => ({ CourseDetail: () => null }));
vi.mock("./CoursePlayer", () => ({ CoursePlayer: () => null }));
vi.mock("./RuntimeCoursePlayer", () => ({ RuntimeCoursePlayer: () => null }));
vi.mock("./CourseReport", () => ({ CourseReport: () => null }));
import { CoursesContainer } from "./CoursesContainer";

// Navigation side effects cannot announce a fake departure during React's
// effect replay or a callback identity change: Lite turns null into a URL push.
describe("course host navigation notifications", () => {
  it("keeps the deep link through effect replay and notifies the latest host only on departure", async () => {
    const first = vi.fn();
    const next = vi.fn();
    const target = { slug: "evidence", mode: "detail" as const };
    const view = render(<StrictMode><CoursesContainer initialOpen={target} onActiveCourseChange={first} /></StrictMode>);
    await act(async () => {});
    expect(first.mock.calls.every(([slug]) => slug === "evidence")).toBe(true);
    view.rerender(<StrictMode><CoursesContainer initialOpen={target} onActiveCourseChange={next} /></StrictMode>);
    await act(async () => {});
    expect(next).not.toHaveBeenCalled();
    view.unmount();
    await act(async () => {});
    expect(next).toHaveBeenCalledTimes(1);
    expect(next).toHaveBeenCalledWith(null);
  });
});
