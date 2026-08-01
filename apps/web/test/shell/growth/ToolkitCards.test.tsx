import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
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

describe("ToolkitCards gallery", () => {
  it("shows all cards with unlocked count, category groups, and 4 theme swatches", async () => {
    vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    expect(screen.getByText(/已解锁/)).toBeTruthy();
    // category headers for the three represented categories
    expect(screen.getByText("知识工具")).toBeTruthy();
    expect(screen.getByText("信息素养")).toBeTruthy();
    expect(screen.getByText("论证结构")).toBeTruthy();
    // 4 colorway swatches (by their aria-labels)
    for (const label of ["浅色", "青绿", "石板", "暖调"]) {
      expect(screen.getByLabelText(label)).toBeTruthy();
    }
  });

  it("opens a detail modal with methodology + usage record for an encountered card", async () => {
    vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByTitle("让步段 · 以退为进"));
    // modal detail sections
    expect(await screen.findByText("这张卡帮你做什么")).toBeInTheDocument();
    expect(screen.getByText("为什么用")).toBeInTheDocument();
    // encountered → usage record present
    expect(screen.getByText("你的使用记录")).toBeInTheDocument();
  });

  it("does not show a usage record for a card the student hasn't encountered", async () => {
    vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByTitle("论证构建卡（图尔敏）"));
    expect(await screen.findByText("这张卡帮你做什么")).toBeInTheDocument();
    expect(screen.queryByText("你的使用记录")).toBeNull();
  });

  it("offers a jump-to-course link that calls onOpenCourse with the card's courseId", async () => {
    vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
    const onOpenCourse = vi.fn();
    render(<ToolkitCards onOpenCourse={onOpenCourse} />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByTitle("信源辨识卡 CRAAP / CRRAAB"));
    const btn = await screen.findByText(/去学这张卡的课程/);
    fireEvent.click(btn);
    expect(onOpenCourse).toHaveBeenCalledWith("00000000-0000-0000-0000-0000000000c1");
  });

  it("persists a new theme and re-fetches covers when a swatch is picked", async () => {
    const getCat = vi.spyOn(api, "getCardsCatalog").mockResolvedValue(CATALOG as never);
    const setTheme = vi.spyOn(api, "setCardTheme").mockResolvedValue("cyber-sage" as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/全部 3 张/)).toBeTruthy());
    fireEvent.click(screen.getByLabelText("青绿"));
    await waitFor(() => expect(setTheme).toHaveBeenCalledWith("cyber-sage"));
    expect(getCat).toHaveBeenCalledWith("cyber-sage");
  });
});
