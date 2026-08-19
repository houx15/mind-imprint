import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

/**
 * Task 8, step 3: the `line-clamp-2` project-title element on the home
 * snapshot tile and the directory card both carry a native `title` attr with
 * the FULL title, so a desktop hover reveals what the clamp cuts off — a
 * safety net alongside the top-bar's tap-to-reveal (RoomSwitcherAndTitle
 * test).
 */

const LONG_TITLE =
  "中国是否让地球变得更可持续？一个关于碳排放、可再生能源投资与全球气候治理格局的深入探究与论证分析";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listProjects: vi.fn(async () => [
        { id: "p1", title: LONG_TITLE, qualLabel: "拓展论文 EE", activeStation: "forming", status: "working" as const },
      ]),
      listCourses: vi.fn(async () => []),
      createProject: vi.fn(),
    },
  };
});

import { Directory } from "@/workspace/Directory";
import { HomePage } from "@/shell/home/HomePage";

describe("Directory card title safety net", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("carries the full project title as a native title attr on the clamped title element", async () => {
    render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);

    const titleEl = await screen.findByText(LONG_TITLE);
    expect(titleEl).toHaveAttribute("title", LONG_TITLE);
    expect(titleEl.className).toContain("line-clamp-2");
  });
});

describe("HomePage card title safety net", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("carries the full project title as a native title attr on the clamped title element", async () => {
    render(
      <HomePage
        user={{ display_name: "Phoebe" } as any}
        onOpenProject={vi.fn()}
        onOpenCourse={vi.fn()}
        onCreateProject={vi.fn()}
        onGoProjects={vi.fn()}
      />,
    );

    const titleEl = await screen.findByText(LONG_TITLE);
    expect(titleEl).toHaveAttribute("title", LONG_TITLE);
    expect(titleEl.className).toContain("line-clamp-2");
  });
});
