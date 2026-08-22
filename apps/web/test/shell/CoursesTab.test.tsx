import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("@/shell/courses/CoursesContainer", () => ({ CoursesContainer: () => <div data-testid="courses-container" />, }));
vi.mock("@/shell/courses/LearningHistory", () => ({ LearningHistory: () => <div data-testid="learning-history" />, }));
vi.mock("@/shell/growth/ToolkitCards", () => ({ ToolkitCards: () => <div data-testid="toolkit-cards" />, }));

import { CoursesTab } from "@/shell/CoursesTab";

describe("CoursesTab pendingSub", () => {
  it("opens on the requested sub-tab when pendingSub is given", () => {
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
});
