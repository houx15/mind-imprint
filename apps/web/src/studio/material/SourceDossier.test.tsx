import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { Anchor } from "@mind-imprint/contracts";
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

  it("highlights a live anchor span in the open article source", () => {
    const article = SOURCE_FIXTURES.find((s) => s.view === "article")!;
    const liveAnchor: Anchor = {
      id: "anchor-live-b2",
      material_id: article.annotate.material_id,
      block_id: "b2",
      start: 0,
      end: 11,
      quote: "变化大到能从太空里看见",
      dimension: "准确性",
      author: "ai",
      question: "这个「变化大到能从太空里看见」的说法，原始论文里是怎么表述的？",
      answer: "",
    };

    render(<SourceDossier sources={SOURCE_FIXTURES} anchors={[liveAnchor]} />);
    fireEvent.click(screen.getByText(article.name));

    fireEvent.click(screen.getByText("变化大到能从太空里看见"));
    expect(screen.getByText(liveAnchor.dimension)).toBeInTheDocument();
    expect(screen.getByText(liveAnchor.question)).toBeInTheDocument();
  });

  it("skips a live anchor with no block_ref and no range (nowhere to highlight)", () => {
    const article = SOURCE_FIXTURES.find((s) => s.view === "article")!;
    const riskNoteAnchor: Anchor = {
      id: "risk_note",
      material_id: article.annotate.material_id,
      block_id: "",
      start: 0,
      end: 0,
      quote: "",
      dimension: "risk_note",
      author: "student",
      question: "风险提示是什么？",
      answer: "作者说读者应留意的风险",
    };

    render(<SourceDossier sources={SOURCE_FIXTURES} anchors={[riskNoteAnchor]} />);
    fireEvent.click(screen.getByText(article.name));

    expect(screen.queryByText("作者说读者应留意的风险")).not.toBeInTheDocument();
  });
});
