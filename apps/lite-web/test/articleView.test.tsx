import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ArticleView } from "@lite/reports/ArticleView";
import type { LiteReport } from "@lite/api/reports";

/**
 * ArticleView — her finished piece, presented as an article.
 *
 * What is worth pinning here is not "the title rendered". It is the handful
 * of decisions that are invisible on a green page and that a redesign could
 * silently undo: the paragraph split, the byline's three parts, the word
 * count coming from the report's OWN stat rather than a second count, and the
 * door to the record page.
 */

afterEach(cleanup);

function report(over: Partial<LiteReport> = {}): LiteReport {
  return {
    version: 1,
    kind: "writing",
    title: "转弯中的国家",
    studentName: "Phoebe",
    finishedAt: "2026-08-30T09:30:00Z",
    stats: [{ key: "words", label: "写了", value: 842, unit: "字" }],
    moments: [],
    keep: null,
    gains: [],
    lensNotes: [],
    notes: [],
    piece: "中国的碳排放总量确实是世界第一。\n\n但把人均和增速放在一起看，结论就没那么干脆了。",
    ...over,
  };
}

describe("ArticleView", () => {
  // Blank-line paragraphs must survive as separate <p>s. Collapsed into one
  // block this is a wall of text, and nobody notices in a passing test suite.
  it("renders her paragraphs as separate paragraphs", () => {
    render(<ArticleView report={report()} onOpenRecord={vi.fn()} />);

    expect(screen.getByText("中国的碳排放总量确实是世界第一。")).toBeTruthy();
    expect(screen.getByText("但把人均和增速放在一起看，结论就没那么干脆了。")).toBeTruthy();
  });

  // "make this page has the author name, date, word count"
  it("carries a byline of name, date and word count", () => {
    render(<ArticleView report={report()} onOpenRecord={vi.fn()} />);

    expect(screen.getByText("Phoebe")).toBeTruthy();
    expect(screen.getByText("2026年8月30日")).toBeTruthy();
    expect(screen.getByText("842 字")).toBeTruthy();
  });

  // 🚨 The count comes from the report's own `words` stat, NOT from counting
  // the piece here. A second count would quietly disagree with the number the
  // record page prints for the same piece — and the server's is the one that
  // knows the language rules (countWordsForLang).
  it("takes the word count from the report's stat, not from the text", () => {
    render(
      <ArticleView
        report={report({ stats: [{ key: "words", label: "写了", value: 1234, unit: "字" }] })}
        onOpenRecord={vi.fn()}
      />,
    );

    expect(screen.getByText("1,234 字")).toBeTruthy();
  });

  // 0 字 is absence, not a fact worth printing — the same rule the stat strip
  // follows. The rest of the byline still renders.
  it("drops the word count when there is no words stat", () => {
    render(<ArticleView report={report({ stats: [] })} onOpenRecord={vi.fn()} />);

    expect(screen.getByText("Phoebe")).toBeTruthy();
    expect(screen.queryByText(/字$/)).toBeNull();
  });

  it("offers the way through to the record", () => {
    const onOpenRecord = vi.fn();
    render(<ArticleView report={report()} onOpenRecord={onOpenRecord} />);

    fireEvent.click(screen.getByRole("button", { name: /这一篇是怎么写出来的/ }));
    expect(onOpenRecord).toHaveBeenCalled();
  });

  // The article page carries NONE of the record: no stat tiles, no 金句, no
  // 收获. That separation is the whole point of splitting the pages, and it
  // is the thing a well-meaning "just show a summary too" would undo.
  it("shows nothing of the record", () => {
    render(
      <ArticleView
        report={report({
          moments: [{ quote: "总量第一和人均第五十，说的不是同一件事。", where: "第 3 段" }],
          keep: { label: "我的收获", text: "把反例写进去，段落反而更站得住。", source: "coach" },
          gains: ["用「先承认，再反驳」写完了让步段"],
        })}
        onOpenRecord={vi.fn()}
      />,
    );

    expect(screen.queryByText("金句")).toBeNull();
    expect(screen.queryByText("我的收获")).toBeNull();
    expect(screen.queryByText("这次的收获")).toBeNull();
    expect(screen.queryByText("总量第一和人均第五十，说的不是同一件事。")).toBeNull();
    // …and no stat tile, even though `words` is in the report and the byline
    // reads it: the byline is a line of type, not the strip.
    expect(screen.queryByText("写了")).toBeNull();
  });

  // 铁律②: a page she publishes is a record, never a verdict.
  it("never renders a score, grade, rank or comparison", () => {
    render(<ArticleView report={report()} onOpenRecord={vi.fn()} />);

    const text = document.body.textContent ?? "";
    expect(text).not.toMatch(/分数|评分|等级|排名|超过|第\s*\d+\s*名/);
  });

  // A writing can be finished on an empty draft. Say so plainly rather than
  // printing a headline and a byline over nothing.
  it("says so when there is no body text", () => {
    render(<ArticleView report={report({ piece: "  \n\n " })} onOpenRecord={vi.fn()} />);

    expect(screen.getByText("这一篇还没有正文。")).toBeTruthy();
  });
});
