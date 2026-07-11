import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SourceDossier } from "./SourceDossier";
import { SOURCE_FIXTURES } from "./fixtures";

describe("SourceDossier", () => {
  it("lists sources with locked count and opens one, emitting source_opened", () => {
    const onEvent = vi.fn();
    render(<SourceDossier sources={SOURCE_FIXTURES} onEvent={onEvent} />);

    expect(screen.getByText(/信源档案/)).toBeInTheDocument();
    expect(screen.getByText(/已收集 2 篇/)).toBeInTheDocument();
    expect(screen.getByText(/已锁定 1\/2/)).toBeInTheDocument();

    fireEvent.click(screen.getByText(SOURCE_FIXTURES[0]!.name));
    expect(onEvent).toHaveBeenCalledWith(
      expect.objectContaining({ type: "source_opened", surface: "studio", url: SOURCE_FIXTURES[0]!.id, time_spent_s: 0 }),
    );
  });

  it("in an article source, clicking a span reveals its question; back returns to the list", () => {
    render(<SourceDossier sources={SOURCE_FIXTURES} />);
    const article = SOURCE_FIXTURES.find((s) => s.view === "article")!;
    const span = article.annotate.spans[0]!;

    fireEvent.click(screen.getByText(article.name));
    expect(screen.queryByText(article.name)).toBeInTheDocument();

    fireEvent.click(screen.getByText("根据 NASA 卫星数据"));
    expect(screen.getByText(span.tag)).toBeInTheDocument();
    expect(screen.getByText(new RegExp(span.note.slice(0, 10)))).toBeInTheDocument();

    fireEvent.click(screen.getByText(/返回信源列表/));
    expect(screen.getByText(/已收集 2 篇/)).toBeInTheDocument();
  });

  it("opens a summary source and shows its takeaway", () => {
    render(<SourceDossier sources={SOURCE_FIXTURES} />);
    const summary = SOURCE_FIXTURES.find((s) => s.view === "summary")!;

    fireEvent.click(screen.getByText(summary.name));
    expect(screen.getByText(summary.takeaway!)).toBeInTheDocument();
  });
});
