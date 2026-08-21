import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { ToolkitCards } from "@/shell/growth/ToolkitCards";
import { api } from "@/api";

afterEach(() => { vi.restoreAllMocks(); });

const CATALOG = {
  theme: "light" as const,
  cards: [
    { cardId: "concession", name: "让步段 · 以退为进", nameEn: "Concession", category: "知识工具",
      purpose: "让步段的用法", stages: ["写作"], example: "", hasAsset: true,
      coverUrl: "https://cdn/concession.webp", courseId: "",
      encountered: true, score: 68, stars: 4, uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" },
    { cardId: "craap", name: "信源辨识卡 CRAAP / CRRAAB", nameEn: "Source Evaluation", category: "信息素养",
      purpose: "对单一来源做纵向体检", stages: ["阅读"], example: "", hasAsset: true,
      coverUrl: "https://cdn/craap.webp", courseId: "00000000-0000-0000-0000-0000000000c1",
      encountered: true, score: 45, stars: 3, uses: 1, surfaces: ["project", "course"], lastUsed: "2026-07-23T00:00:00Z" },
    { cardId: "toulmin", name: "论证构建卡（图尔敏）", nameEn: "Toulmin", category: "论证结构",
      purpose: "把论证拆成部件", stages: [], example: "", hasAsset: false,
      coverUrl: "", courseId: "",
      encountered: false, score: 0, stars: 0, uses: 0, surfaces: [], lastUsed: "" },
  ],
};

// Related courses are derived from listCourses() by card_ids (NOT from the
// catalog entry's courseId), and the jump hands back the course SLUG.
const COURSES = [
  { slug: "co-craap", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句话出发",
    time_label: "约 40 分钟", card_ids: ["craap"], step_count: 3 },
];

function mockCatalog() {
  vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
  vi.spyOn(api, "listCourses").mockResolvedValue(COURSES as never);
}

describe("ToolkitCards gallery", () => {
  it("shows all cards with unlocked count, category groups, and 4 theme swatches", async () => {
    mockCatalog();
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    expect(screen.getByText(/已遇到/)).toBeTruthy();
    // Each represented category appears twice: once as a filter chip, once as
    // the section header above that group's tiles.
    for (const cat of ["知识工具", "信息素养", "论证结构"]) {
      expect(screen.getAllByText(cat).length).toBeGreaterThanOrEqual(2);
    }
    // 4 colorway swatches (by their aria-labels; cyber-warm surfaces as 湖蓝)
    for (const label of ["浅色", "青绿", "石板", "湖蓝"]) {
      expect(screen.getByLabelText(label)).toBeTruthy();
    }
  });

  it("distinguishes encountered (full colour, star rating shown) from not-yet-encountered (dimmed, no stars)", async () => {
    mockCatalog();
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());

    const encountered = screen.getByTitle("让步段 · 以退为进");
    const notEncountered = screen.getByTitle("论证构建卡（图尔敏）");

    // Encountered covers render at full colour; un-encountered ones keep the
    // colorway but are softly dimmed + desaturated, never fully grey.
    expect(encountered.querySelector("img")!.className).toContain("grayscale-0");
    expect(within(encountered).getByLabelText("熟练度 4 星")).toBeInTheDocument();

    expect(notEncountered.innerHTML).toContain("grayscale-[.35]");
    expect(within(notEncountered).getByText("还没遇到")).toBeInTheDocument();
    expect(within(notEncountered).queryByLabelText(/熟练度/)).toBeNull();
  });

  it("does not print the card name under the cover — the artwork already carries it", async () => {
    mockCatalog();
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());

    // A card WITH cover art: the name shows exactly once inside the tile — in
    // the hover overlay. The old bottom name band would make this 2.
    const withArt = screen.getByTitle("让步段 · 以退为进");
    expect(within(withArt).getAllByText("让步段 · 以退为进")).toHaveLength(1);
    // ...but it is still reachable non-visually.
    expect(withArt.querySelector("img")!.getAttribute("alt")).toBe("让步段 · 以退为进");
    expect(withArt.getAttribute("title")).toBe("让步段 · 以退为进");

    // A card with NO cover art falls back to a text face, which prints the name
    // in the middle — so that tile still shows it (face + hover overlay).
    const noArt = screen.getByTitle("论证构建卡（图尔敏）");
    expect(within(noArt).getAllByText("论证构建卡（图尔敏）")).toHaveLength(2);
  });

  it("paints a real scrim behind the status row so it stays legible over the cover art", async () => {
    mockCatalog();
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());

    // Every v3 cover ends in a saturated slogan strip, so the stars need a
    // scrim. It has to be an inline gradient — Tailwind's alpha syntax on the
    // mk-* CSS variables (from-mk-ink/[.82]) compiles to nothing and used to
    // leave this band with background-image: none.
    const tile = screen.getByTitle("让步段 · 以退为进");
    const band = tile.querySelector<HTMLElement>("div.absolute.inset-x-0.bottom-0")!;
    expect(band.style.background).toContain("linear-gradient");
    expect(band.className).toContain("h-[20%]");
  });

  it("opens a two-tab detail modal whose 我的练习历史 tab holds the usage record", async () => {
    mockCatalog();
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByTitle("让步段 · 以退为进"));

    // 介绍 tab is the default; the practice history sits behind its own tab.
    expect(await screen.findByText("介绍")).toBeInTheDocument();
    fireEvent.click(screen.getByText("我的练习历史"));
    expect(screen.getByText("练习次数")).toBeInTheDocument();
    expect(screen.getByText("熟练度（星）")).toBeInTheDocument();
  });

  it("shows an empty-state instead of a usage record for a card the student hasn't encountered", async () => {
    mockCatalog();
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByTitle("论证构建卡（图尔敏）"));
    expect(await screen.findByText("我的练习历史")).toBeInTheDocument();
    fireEvent.click(screen.getByText("我的练习历史"));
    expect(screen.getByText("你还没在项目里练过这张卡")).toBeInTheDocument();
    expect(screen.queryByText("练习次数")).toBeNull();
  });

  it("offers a jump-to-course link that calls onOpenCourse with the course slug", async () => {
    mockCatalog();
    const onOpenCourse = vi.fn();
    render(<ToolkitCards onOpenCourse={onOpenCourse} />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByTitle("信源辨识卡 CRAAP / CRRAAB"));
    const btn = await screen.findByText("一条网络信息，该不该信");
    fireEvent.click(btn);
    expect(onOpenCourse).toHaveBeenCalledWith("co-craap");
  });

  it("persists a new theme and re-fetches covers when a swatch is picked", async () => {
    const getCat = vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
    vi.spyOn(api, "listCourses").mockResolvedValue(COURSES as never);
    const setTheme = vi.spyOn(api, "setCardTheme").mockResolvedValue("cyber-sage" as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByLabelText("青绿"));
    await waitFor(() => expect(setTheme).toHaveBeenCalledWith("cyber-sage"));
    expect(getCat).toHaveBeenCalledWith("cyber-sage");
  });
});
