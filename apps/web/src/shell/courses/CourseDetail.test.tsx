import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// The server border-validates `introduction` only as "a JSON object" and the
// web client never Zod-parses listCourses()'s raw JSON, so a partial
// introduction like `{ hook: "h" }` (missing takeaways/alignment/keywords)
// reaches CourseDetail exactly as stored — regression test for that shape.
vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return { ...real, api: { ...real.api, listCourses: vi.fn(), getCourseProgress: vi.fn() } };
});

import { api } from "@/api";
import { CourseDetail } from "@/shell/courses/CourseDetail";

const partialIntroCourse = {
  slug: "co1",
  branch: "批判性思维",
  title: "一条网络信息，该不该信",
  blurb: "从一句朋友圈转发出发，学会溯源。",
  time_label: "约 40 分钟",
  card_ids: ["craap"],
  step_count: 3,
  category: null,
  featuredRank: null,
  // Partial introduction: only `hook`, everything else absent. Mirrors the
  // branch's own Go tests, which store `{"hook":"h"}` as a valid document.
  introduction: { hook: "h" },
};

function noop() {}

describe("CourseDetail", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders a partial introduction (only `hook`, no takeaways/alignment/keywords) without throwing", async () => {
    (api.listCourses as any).mockResolvedValue([partialIntroCourse]);
    (api.getCourseProgress as any).mockResolvedValue(null);

    render(<CourseDetail slug="co1" onStart={noop} onBack={noop} />);

    expect(await screen.findByText("h")).toBeInTheDocument();
    expect(screen.getByText("开始学习")).toBeInTheDocument();
    // The undefined sections must not render at all.
    expect(screen.queryByText("带走什么")).not.toBeInTheDocument();
    expect(screen.queryByText("学科对标")).not.toBeInTheDocument();
    expect(screen.queryByText("关键词")).not.toBeInTheDocument();
  });
});
