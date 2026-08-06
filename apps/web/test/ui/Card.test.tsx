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
  it("renders as a md-level surface with exactly the md radius (no competing sm radius)", () => {
    render(<Card>card body</Card>);
    const el = screen.getByText("card body");
    expect(el.className).toContain("shadow-mk-md");
    expect(el.className).toContain("rounded-mk-md");
    // Regression guard: Tailwind emits `rounded-mk-*` utilities in
    // alphabetical order in the compiled stylesheet, so stacking both
    // `rounded-mk-sm` (from Surface's base) and `rounded-mk-md` (from Card)
    // silently resolves to sm, not md. Card must carry exactly one radius
    // class.
    expect(el.className).not.toContain("rounded-mk-sm");
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

  it("is keyboard-accessible when onClick is set (role=button, Enter/Space fire onClick)", async () => {
    const onClick = vi.fn();
    render(<CompactRow title="点我" onClick={onClick} />);
    const row = screen.getByRole("button", { name: /点我/ });
    expect(row).toHaveAttribute("tabIndex", "0");
    row.focus();
    await userEvent.keyboard("{Enter}");
    expect(onClick).toHaveBeenCalledOnce();
    await userEvent.keyboard(" ");
    expect(onClick).toHaveBeenCalledTimes(2);
  });

  it("has no button role when onClick is absent", () => {
    render(<CompactRow title="静态行" />);
    expect(screen.queryByRole("button", { name: /静态行/ })).not.toBeInTheDocument();
  });
});
