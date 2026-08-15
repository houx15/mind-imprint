import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

import { ApiError } from "@/api/client";
import { getCourseDefinition } from "@/api/courseDefinition";

// The runtime renderer is stubbed: we test RuntimeCoursePlayer's WIRING (fetch
// the definition, build the four adapters, inject idFactory/clock, surface
// completion as onFinish), not the renderer's internals (covered in the
// course-renderer package). The stub captures the props it was mounted with and
// exposes a button that drives the session to `completed`.
let lastPlayerProps: any = null;
vi.mock("@mind-imprint/course-renderer", () => ({
  CoursePlayer: (props: any) => {
    lastPlayerProps = props;
    return (
      <div data-testid="runtime-player">
        <span>Opening</span>
        <button type="button" onClick={() => void props.adapters.sessionAdapter.setStatus("sid", "completed")}>
          drive-to-completed
        </button>
      </div>
    );
  },
}));

// Session adapter stubbed to a recording fake so RuntimeCoursePlayer's setStatus
// WRAPPER (the onFinish-on-completed seam) is what's under test.
const baseSetStatus = vi.fn().mockResolvedValue(undefined);
vi.mock("@/course/apiSessionAdapter", () => ({
  makeApiSessionAdapter: vi.fn(() => ({
    load: vi.fn(),
    create: vi.fn(),
    appendEvent: vi.fn(),
    saveSliceState: vi.fn(),
    saveScene: vi.fn(),
    setStatus: baseSetStatus,
    flush: vi.fn(),
  })),
}));

// Legacy player stubbed so the 404-routing test can assert which player mounts
// without dragging in the legacy player's own data fetches.
vi.mock("@/shell/courses/CoursePlayer", () => ({
  CoursePlayer: (props: any) => <div data-testid="legacy-player">legacy:{props.courseId}</div>,
}));

vi.mock("@/api/courseDefinition", () => ({
  getCourseDefinition: vi.fn(),
  createCourseSession: vi.fn(),
  saveCourseSession: vi.fn(),
}));
const getDefMock = vi.mocked(getCourseDefinition);

import { RuntimeCoursePlayer } from "@/shell/courses/RuntimeCoursePlayer";
import { CoursesContainer } from "@/shell/courses/CoursesContainer";

const SLUG = "evidence-comparability";
const golden = {
  schemaVersion: "2.0",
  course: { id: "evidence-comparability", title: "Can These Two Claims Be Compared?" },
};

beforeEach(() => {
  vi.clearAllMocks();
  lastPlayerProps = null;
  baseSetStatus.mockResolvedValue(undefined);
});

describe("RuntimeCoursePlayer", () => {
  it("fetches the definition, mounts the runtime player with the four adapters + injected idFactory/clock, and surfaces completion as onFinish", async () => {
    getDefMock.mockResolvedValue(golden);
    const onFinish = vi.fn();
    const onExit = vi.fn();

    render(<RuntimeCoursePlayer slug={SLUG} studentId="student-42" onExit={onExit} onFinish={onFinish} />);

    // Renders the Opening (the runtime player) once the definition resolves.
    await screen.findByTestId("runtime-player");
    expect(screen.getByText("Opening")).toBeInTheDocument();
    expect(getDefMock).toHaveBeenCalledWith(SLUG);

    // Wiring: the fetched document + the four adapters + host boundary factories.
    expect(lastPlayerProps.document).toEqual(golden);
    expect(lastPlayerProps.studentId).toBe("student-42");
    expect(typeof lastPlayerProps.idFactory).toBe("function");
    expect(typeof lastPlayerProps.clock).toBe("function");
    expect(Object.keys(lastPlayerProps.adapters).sort()).toEqual(
      ["assetResolver", "closingGenerator", "openingGenerator", "sessionAdapter"].sort(),
    );
    // clock returns an ISO string; idFactory a string.
    expect(typeof lastPlayerProps.clock()).toBe("string");
    expect(new Date(lastPlayerProps.clock()).toString()).not.toBe("Invalid Date");

    // Completion: driving the session to `completed` fires onFinish (the
    // renderer has no completion callback of its own).
    fireEvent.click(screen.getByText("drive-to-completed"));
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(baseSetStatus).toHaveBeenCalledWith("sid", "completed");
  });
});

describe("CoursesContainer per-course routing", () => {
  it("mounts the runtime player for a course WITH a 2.0 definition", async () => {
    getDefMock.mockResolvedValue(golden);
    render(<CoursesContainer initialCourseId={SLUG} studentId="student-42" />);
    await screen.findByTestId("runtime-player");
    expect(screen.queryByTestId("legacy-player")).toBeNull();
  });

  it("falls back to the legacy player when the definition 404s (no 2.0 definition)", async () => {
    getDefMock.mockRejectedValue(new ApiError("not_found", "该课程没有 2.0 定义", 404));
    render(<CoursesContainer initialCourseId={SLUG} studentId="student-42" />);
    const legacy = await screen.findByTestId("legacy-player");
    expect(legacy).toHaveTextContent(`legacy:${SLUG}`);
    expect(screen.queryByTestId("runtime-player")).toBeNull();
  });
});
