import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { DigCandidate } from "@mind-imprint/contracts";
import { TraceSourcePanel } from "@/studio/reading/TraceSourcePanel";

const CAND: DigCandidate = {
  doi: "10.1000/upstream",
  title: "Upstream primary data on adolescent sleep",
  authors: "Origin et al.",
  year: "2014",
  journal: "Journal of Sleep",
  abstract: "The original dataset behind the widely-cited twelve-percent figure.",
  url: "https://doi.org/10.1000/upstream",
};

describe("TraceSourcePanel", () => {
  it("cites → lists upstream works → adopting one calls onAdoptSource and marks it added", async () => {
    const onTraceCitation = vi.fn().mockResolvedValue([CAND]);
    const onTraceSearch = vi.fn().mockResolvedValue([]);
    const onAdoptSource = vi.fn().mockResolvedValue(undefined);

    render(
      <TraceSourcePanel
        seedQuote="twelve percent drop"
        sourceUrl="https://doi.org/10.1000/thispaper"
        onTraceCitation={onTraceCitation}
        onTraceSearch={onTraceSearch}
        onAdoptSource={onAdoptSource}
        onClose={() => {}}
      />,
    );

    fireEvent.click(screen.getByText("看这篇引用的文献"));
    await screen.findByText("Upstream primary data on adolescent sleep");
    expect(onTraceCitation).toHaveBeenCalledWith("https://doi.org/10.1000/thispaper");

    fireEvent.click(screen.getByText("采纳并加入我的来源"));
    await waitFor(() => expect(onAdoptSource).toHaveBeenCalledWith(CAND));
    expect(await screen.findByText("已加入我的来源 ✓")).toBeInTheDocument();
  });

  it("no-DOI source: citation returns empty → guidance to search; search box seeded from the referenced quote", async () => {
    const onTraceCitation = vi.fn().mockResolvedValue([]);
    const onTraceSearch = vi.fn().mockResolvedValue([CAND]);
    const onAdoptSource = vi.fn().mockResolvedValue(undefined);

    render(
      <TraceSourcePanel
        seedQuote="twelve percent drop"
        sourceUrl=""
        onTraceCitation={onTraceCitation}
        onTraceSearch={onTraceSearch}
        onAdoptSource={onAdoptSource}
        onClose={() => {}}
      />,
    );

    // The search box is pre-seeded with the passage she referenced.
    expect(screen.getByDisplayValue("twelve percent drop")).toBeInTheDocument();

    fireEvent.click(screen.getByText("看这篇引用的文献"));
    expect(await screen.findByText(/没有可解析的 DOI/)).toBeInTheDocument();

    fireEvent.click(screen.getByText("搜原始出处"));
    await screen.findByText("Upstream primary data on adolescent sleep");
    expect(onTraceSearch).toHaveBeenCalledWith("twelve percent drop");
  });
});
