import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

vi.mock("@/shell/courses/CoursesContainer", () => ({
  CoursesContainer: ({ initialOpen }: { initialOpen?: { slug: string } | null }) => (
    <div data-testid="courses-container" data-open={initialOpen?.slug ?? ""} />
  ),
}));
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

describe("CoursesTab pendingCourseId", () => {
  it("opens the deep-linked course given at mount", () => {
    render(
      <CoursesTab
        pendingCourseId="course-12"
        onPendingCourseConsumed={() => {}}
        onGoPortal={() => {}}
        onImmersiveChange={() => {}}
      />,
    );
    expect(screen.getByTestId("courses-container")).toHaveAttribute("data-open", "course-12");
  });

  it("reacts to a pendingCourseId that arrives AFTER mount (browser Back reopens a course)", async () => {
    const onPendingCourseConsumed = vi.fn();
    const props = {
      onPendingCourseConsumed,
      onGoPortal: () => {},
      onImmersiveChange: () => {},
    };
    // Mount on the 图鉴 sub with no course deep-link — as when the courses tab is
    // already open and the user is browsing, not inside a course.
    const { rerender } = render(<CoursesTab {...props} pendingCourseId={null} pendingSub="gallery" onPendingSubConsumed={() => {}} />);
    expect(screen.getByTestId("toolkit-cards")).toBeInTheDocument();

    // The shell's popstate handler sets pendingCourseId when Back lands on
    // /courses/:slug → the already-mounted tab must switch to 课程 and open it.
    rerender(<CoursesTab {...props} pendingCourseId="course-12" pendingSub={null} onPendingSubConsumed={() => {}} />);
    await waitFor(() => expect(screen.getByTestId("courses-container")).toHaveAttribute("data-open", "course-12"));
    expect(onPendingCourseConsumed).toHaveBeenCalled();
  });
});
