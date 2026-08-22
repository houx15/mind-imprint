import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div data-testid="courses-container" />, }));
vi.mock("@/shell/courses/LearningHistory", () => ({ LearningHistory: () => <div data-testid="learning-history" />, }));
vi.mock("@/shell/growth/ToolkitCards", () => ({ ToolkitCards: () => <div data-testid="toolkit-cards" />, }));

import { CoursesTab } from "@/shell/CoursesTab";

describe("CoursesTab pendingSub", () => {
  it("opens on the requested sub-tab when pendingSub is given at mount", () => {
    render(
      <CoursesTab
        pendingCourseId={null}
        onPendingCourseConsumed={() => {}}
        pendingSub="gallery"
        onPendingSubConsumed={() => {}}
        onGoPortal={() => {}}
        onImmersiveChange={() => {}}
      />,
    );
    expect(screen.getByTestId("toolkit-cards")).toBeInTheDocument();
  });

  it("reacts to a pendingSub change after mount (tour switches sub-tab live)", async () => {
    const onPendingSubConsumed = vi.fn();
    const props = {
      pendingCourseId: null,
      onPendingCourseConsumed: () => {},
      onPendingSubConsumed,
      onGoPortal: () => {},
      onImmersiveChange: () => {},
    };
    // Mount on the default 课程 sub (no pendingSub), as happens on the first
    // setTab("courses") before any setCoursesSub.
    const { rerender } = render(<CoursesTab {...props} pendingSub={null} />);
    expect(screen.getByTestId("courses-container")).toBeInTheDocument();

    // A later setCoursesSub flips the prop → the mounted tab must switch subs.
    rerender(<CoursesTab {...props} pendingSub="gallery" />);
    await waitFor(() => expect(screen.getByTestId("toolkit-cards")).toBeInTheDocument());
    expect(screen.queryByTestId("courses-container")).not.toBeInTheDocument();
    expect(onPendingSubConsumed).toHaveBeenCalled();
  });
});
