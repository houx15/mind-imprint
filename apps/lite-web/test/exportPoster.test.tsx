import { render, screen, cleanup, fireEvent, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReportPoster } from "@lite/reports/ReportPoster";
import { exportPoster } from "@lite/reports/exportPoster";
import { ReportPanel } from "@lite/reports/ReportPanel";
import type { LiteReport } from "@lite/api/reports";

/**
 * `exportPoster` + `ReportPoster` + the `ReportPanel` 导出图片 button —
 * the "picture worth showing someone" (task-11-brief.md). `html-to-image`'s
 * `toPng` is mocked throughout: it is NOT re-implemented here, so no test
 * may assert anything about the returned data URL's length or prefix — that
 * would only assert the mock's own stub. What IS asserted is the call shape
 * `exportPoster` produces (the poster's own node, not a wrapper;
 * `pixelRatio: 2`; a download triggered with a filename carrying the
 * title), that the value `toPng` hands back genuinely flows into the
 * anchor's `href` (a real, useful assertion — separate from the forbidden
 * "assert the mock's own stub value is long/PNG-shaped" one), and the
 * poster's content discipline (name/date/stats present, at most three 金句
 * even when more are supplied). Whether a real PNG comes out is Task 14's
 * e2e, in a real browser.
 *
 * ## Fix round 1 — the button's own wiring, not just its two halves
 *
 * The `describe("ReportPanel → 导出图片")` block below covers the ONE thing
 * the first pass shipped untested: `ReportPanel.handleExport` itself —
 * `document.body.appendChild` → `createRoot` → `flushSync` render → ref
 * capture → `exportPoster` → `finally` unmount + remove. `exportPoster`'s
 * own tests above prove the rasterizer call shape in isolation;
 * `reportPanel.test.tsx` predates the button and never clicks it. Neither
 * proves the detached-mount choreography itself is correct — that a bug
 * like appending after `createRoot` (a detached node rasterizes empty,
 * silently) or a `finally` that leaks a container would surface here,
 * before Task 14's e2e.
 */

const toPng = vi.fn();
vi.mock("html-to-image", () => ({
  toPng: (...args: unknown[]) => toPng(...args),
}));

// F2: ReportPanel fetches through `getReportEnvelope` now (report +
// share state together), not the bare `getReport` this file used to mock —
// see api/reports.ts and ReportPanel.tsx.
const getReportEnvelope = vi.fn();
const shareReport = vi.fn();
const unshareReport = vi.fn();
vi.mock("@lite/api/reports", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@lite/api/reports")>();
  return {
    ...actual,
    getReportEnvelope: (...args: unknown[]) => getReportEnvelope(...args),
    shareReport: (...args: unknown[]) => shareReport(...args),
    unshareReport: (...args: unknown[]) => unshareReport(...args),
  };
});

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
    lensNotes: [],
    notes: [],
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

  it("renders no stat tiles at all when every stat is 0 — the exported picture must not carry four zeros to a parent", () => {
    const allZero = report({
      stats: [
        { key: "focusMinutes", label: "专注时长", value: 0, unit: "分钟" },
        { key: "coachTurns", label: "和印记聊了", value: 0, unit: "轮" },
      ],
      moments: [],
    });

    render(<ReportPoster report={allZero} />);

    expect(screen.queryByText("专注时长")).toBeNull();
    expect(screen.queryByText("和印记聊了")).toBeNull();
  });

  it("renders only the non-zero stats from a mixed set", () => {
    const mixed = report({
      stats: [
        { key: "focusMinutes", label: "专注时长", value: 0, unit: "分钟" },
        { key: "wordsRead", label: "阅读字数", value: 860, unit: "字" },
      ],
      moments: [],
    });

    render(<ReportPoster report={mixed} />);

    expect(screen.queryByText("专注时长")).toBeNull();
    expect(screen.getByText("860")).toBeTruthy();
    expect(screen.getByText("阅读字数")).toBeTruthy();
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

/**
 * A thin report — no 金句 — deliberately, so these tests aren't tripped up
 * by `ReportView`'s own `<blockquote>`s coexisting in the DOM alongside the
 * transient `ReportPoster`'s while an export is in flight. The point of
 * this block is the button's wiring, not the poster's content (already
 * pinned above).
 */
function thinReport(over: Partial<LiteReport> = {}): LiteReport {
  return report({ moments: [], ...over });
}

/**
 * `exportPoster`'s real download step creates a real `<a href="data:…"
 * download>` and calls `.click()` on it. jsdom does not implement the
 * `download` attribute's "save, don't navigate" behaviour — it logs a
 * "Not implemented: navigation" error and tries to navigate instead. That
 * is a jsdom limitation, not a product bug (the exact same click already
 * works, unmocked, in a real browser — that's Task 14's e2e); the
 * `exportPoster` unit tests above sidestep it by mocking anchor creation,
 * and these `ReportPanel`-driven tests do the same so this file's own
 * output stays free of noise unrelated to what's under test here (the
 * detached-mount choreography, not the anchor/download mechanics already
 * pinned above).
 */
function stubDownloadAnchor(): () => void {
  const realCreateElement = document.createElement.bind(document);
  const anchorStub = realCreateElement("a") as HTMLAnchorElement;
  vi.spyOn(anchorStub, "click").mockImplementation(() => {});
  const createElementSpy = vi
    .spyOn(document, "createElement")
    .mockImplementation((tag: string) => (tag === "a" ? anchorStub : realCreateElement(tag)));
  return () => createElementSpy.mockRestore();
}

describe("ReportPanel → 导出图片", () => {
  it("drives the whole export path through the button, and the rasterizer sees a live, in-document node", async () => {
    const restore = stubDownloadAnchor();
    try {
      getReportEnvelope.mockResolvedValue({ report: thinReport(), shareToken: null });
      let sawNodeInDocument: boolean | null = null;
      toPng.mockImplementation(async (node: HTMLElement) => {
        sawNodeInDocument = document.body.contains(node);
        return "data:image/png;base64,stub";
      });

      render(<ReportPanel kind="reading" atomId="atom-1" />);
      await screen.findByText(thinReport().title);

      fireEvent.click(screen.getByRole("button", { name: "导出图片" }));

      await waitFor(() => expect(toPng).toHaveBeenCalledTimes(1));
      // this is the assertion that actually guards the detached-mount
      // choreography: a node appended after `createRoot`, or never
      // appended at all, would still be handed to `toPng` — it would just
      // rasterize empty. Only checking that the node reached `toPng` at
      // all (already covered by the `exportPoster` unit tests above)
      // would miss that bug.
      expect(sawNodeInDocument).toBe(true);
    } finally {
      restore();
    }
  });

  it("cleans up the temporary mount after a successful export — no stray poster, no stray container", async () => {
    const restore = stubDownloadAnchor();
    try {
      getReportEnvelope.mockResolvedValue({ report: thinReport(), shareToken: null });
      toPng.mockResolvedValue("data:image/png;base64,stub");

      render(<ReportPanel kind="reading" atomId="atom-1" />);
      await screen.findByText(thinReport().title);
      const bodyChildrenBeforeExport = document.body.children.length;

      fireEvent.click(screen.getByRole("button", { name: "导出图片" }));
      await waitFor(() => expect(toPng).toHaveBeenCalledTimes(1));

      // the button's own label returns from "生成图片中…" once the
      // `finally` has run and `setExporting(false)` has landed.
      await screen.findByRole("button", { name: "导出图片" });

      expect(document.body.children.length).toBe(bodyChildrenBeforeExport);
      expect(document.body.querySelector('div[style*="left: -99999px"]')).toBeNull();
    } finally {
      restore();
    }
  });

  it("cleans up even when the rasterizer rejects — no leaked container, no error escaping into the render", async () => {
    const restore = stubDownloadAnchor();
    try {
      getReportEnvelope.mockResolvedValue({ report: thinReport(), shareToken: null });
      toPng.mockRejectedValue(new Error("rasterize failed"));

      render(<ReportPanel kind="reading" atomId="atom-1" />);
      await screen.findByText(thinReport().title);
      const bodyChildrenBeforeExport = document.body.children.length;

      fireEvent.click(screen.getByRole("button", { name: "导出图片" }));
      await waitFor(() => expect(toPng).toHaveBeenCalledTimes(1));
      await screen.findByRole("button", { name: "导出图片" });

      expect(document.body.children.length).toBe(bodyChildrenBeforeExport);
      expect(document.body.querySelector('div[style*="left: -99999px"]')).toBeNull();
      // the report itself is still there — a failed export never took the
      // room down with it.
      expect(screen.getByText(thinReport().title)).toBeTruthy();
    } finally {
      restore();
    }
  });

  it("the exporting gate blocks a second click while the first export is still in flight", async () => {
    const restore = stubDownloadAnchor();
    try {
      getReportEnvelope.mockResolvedValue({ report: thinReport(), shareToken: null });
      let resolveToPng!: (v: string) => void;
      toPng.mockImplementation(
        () =>
          new Promise<string>((resolve) => {
            resolveToPng = resolve;
          }),
      );

      render(<ReportPanel kind="reading" atomId="atom-1" />);
      await screen.findByText(thinReport().title);
      const button = screen.getByRole("button", { name: "导出图片" });

      fireEvent.click(button);
      // the button is disabled synchronously (React flushes the
      // `exporting` state before the first `await` inside `handleExport`
      // suspends it) — a second click on a disabled button is a no-op,
      // but assert the OUTCOME (call count), not the DOM attribute, so
      // this pins the actual behaviour rather than an implementation
      // detail.
      fireEvent.click(button);

      resolveToPng("data:image/png;base64,stub");
      await waitFor(() => expect(toPng).toHaveBeenCalledTimes(1));
      await screen.findByRole("button", { name: "导出图片" });

      expect(toPng).toHaveBeenCalledTimes(1);
    } finally {
      restore();
    }
  });
});
