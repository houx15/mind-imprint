import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReportPoster } from "@lite/reports/ReportPoster";
import { exportPoster } from "@lite/reports/exportPoster";
import type { LiteReport } from "@lite/api/reports";

/**
 * `exportPoster` + `ReportPoster` — the "picture worth showing someone"
 * (task-11-brief.md). `html-to-image`'s `toPng` is mocked throughout: it is
 * NOT re-implemented here, so no test may assert anything about the
 * returned data URL's length or prefix — that would only assert the mock's
 * own stub. What IS asserted is the call shape `exportPoster` produces
 * (the poster's own node, not a wrapper; `pixelRatio: 2`; a download
 * triggered with a filename carrying the title) and the poster's content
 * discipline (name/date/stats present, at most three 金句 even when more
 * are supplied). Whether a real PNG comes out is Task 14's e2e, in a real
 * browser.
 */

const toPng = vi.fn();
vi.mock("html-to-image", () => ({
  toPng: (...args: unknown[]) => toPng(...args),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function report(over: Partial<LiteReport> = {}): LiteReport {
  return {
    version: 1,
    kind: "reading",
    title: "中国是否让地球变得更可持续？",
    studentName: "Phoebe",
    finishedAt: "2026-08-29T03:00:00Z",
    stats: [
      { key: "focusMinutes", label: "专注时长", value: 12, unit: "分钟" },
      { key: "wordsRead", label: "阅读字数", value: 860, unit: "字" },
    ],
    moments: [
      { quote: "中国的碳排放量是全球第一，这个事实我没法绕过去。", where: "第 3 段" },
      { quote: "让步不是认输，是把对方的反例先说出来。", where: "第 5 段" },
      { quote: "溯源之后我才敢用这条数据。", where: "CRAAP 工具卡" },
      { quote: "第四句金句不该出现在海报上。", where: "第 9 段" },
    ],
    keep: null,
    gains: [],
    ...over,
  };
}

describe("exportPoster", () => {
  it("hands the poster node to the rasterizer with the right options", async () => {
    toPng.mockResolvedValue("data:image/png;base64,stub");

    const node = document.createElement("div");
    document.body.appendChild(node);

    const realCreateElement = document.createElement.bind(document);
    const anchor = realCreateElement("a") as HTMLAnchorElement;
    const clickSpy = vi.spyOn(anchor, "click").mockImplementation(() => {});
    const createElementSpy = vi
      .spyOn(document, "createElement")
      .mockImplementation((tag: string) => (tag === "a" ? anchor : realCreateElement(tag)));

    await exportPoster(node, "中国是否让地球变得更可持续？.png");

    // called with the poster NODE ITSELF — no wrapper — and pixelRatio 2.
    expect(toPng).toHaveBeenCalledTimes(1);
    expect(toPng).toHaveBeenCalledWith(node, { pixelRatio: 2, cacheBust: true });

    // a download was triggered, with a filename carrying the title.
    expect(anchor.download).toContain("中国是否让地球变得更可持续？");
    expect(anchor.href).toBe("data:image/png;base64,stub");
    expect(clickSpy).toHaveBeenCalledTimes(1);

    createElementSpy.mockRestore();
    document.body.removeChild(node);
  });

  it("never throws when the rasterizer rejects — a failed export must not break the room", async () => {
    toPng.mockRejectedValue(new Error("rasterize failed"));
    const node = document.createElement("div");

    await expect(exportPoster(node, "标题.png")).resolves.toBeUndefined();
  });

  it("does nothing when handed no node", async () => {
    await expect(exportPoster(null, "标题.png")).resolves.toBeUndefined();
    expect(toPng).not.toHaveBeenCalled();
  });
});

describe("ReportPoster", () => {
  it("carries her name, the date, the stats and at most three 金句", () => {
    const { container } = render(<ReportPoster report={report()} />);

    expect(screen.getByText("中国是否让地球变得更可持续？")).toBeTruthy();
    expect(screen.getByText(/Phoebe/)).toBeTruthy();
    expect(screen.getByText(/2026年8月29日/)).toBeTruthy();
    expect(screen.getByText("12")).toBeTruthy();
    expect(screen.getByText("专注时长")).toBeTruthy();
    expect(screen.getByText("860")).toBeTruthy();

    // at most three 金句, even though four were supplied.
    const quotes = container.querySelectorAll("blockquote");
    expect(quotes.length).toBe(3);
    expect(screen.queryByText(/第四句金句不该出现在海报上/)).toBeNull();

    // nothing else — no 收获 list, no share/export chrome inside the poster.
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("is rendered offscreen (fixed + off-canvas), never display:none", () => {
    const { container } = render(<ReportPoster report={report()} />);
    const posterNode = container.firstElementChild as HTMLElement;

    expect(posterNode.style.position).toBe("fixed");
    expect(posterNode.style.left).toBe("-99999px");
    expect(posterNode.style.display).not.toBe("none");
  });

  it("uses the system font stack only, no web font", () => {
    const { container } = render(<ReportPoster report={report()} />);
    const posterNode = container.firstElementChild as HTMLElement;

    expect(posterNode.style.fontFamily).toContain("PingFang SC");
    expect(posterNode.style.fontFamily).toContain("Microsoft YaHei");
  });
});
