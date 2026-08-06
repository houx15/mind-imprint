import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Surface, Card, CompactRow } from "@/ui/Card";

describe("Surface", () => {
  it("renders children", () => {
    render(<Surface>hello surface</Surface>);
    expect(screen.getByText("hello surface")).toBeInTheDocument();
  });

  it("at level='hairline' (default) applies border + xs shadow classes", () => {
    render(<Surface>content</Surface>);
    const el = screen.getByText("content");
    expect(el.className).toContain("border-mk-border");
    expect(el.className).toContain("shadow-mk-xs");
  });

  it("at level='md' applies the md shadow class", () => {
    render(<Surface level="md">content</Surface>);
    expect(screen.getByText("content").className).toContain("shadow-mk-md");
  });
});

describe("Card", () => {
  it("renders as a md-level surface with the card radius", () => {
    render(<Card>card body</Card>);
    const el = screen.getByText("card body");
    expect(el.className).toContain("shadow-mk-md");
    expect(el.className).toContain("rounded-mk-md");
  });
});

describe("CompactRow", () => {
  it("renders title, meta, and trailing regions", () => {
    render(
      <CompactRow
        thumb={<span>📎</span>}
        title="任务标题"
        meta="2 天前"
        trailing={<span>更多</span>}
      />,
    );
    expect(screen.getByText("任务标题")).toBeInTheDocument();
    expect(screen.getByText("2 天前")).toBeInTheDocument();
    expect(screen.getByText("更多")).toBeInTheDocument();
  });

  it("fires onClick when clicked", async () => {
    const onClick = vi.fn();
    render(<CompactRow title="点我" onClick={onClick} />);
    await userEvent.click(screen.getByText("点我"));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
