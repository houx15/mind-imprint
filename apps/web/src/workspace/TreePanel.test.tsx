import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { TreePanel } from "./TreePanel";
import type { ProcessNode } from "./processTree";

// ─── fixtures ─────────────────────────────────────────────────────────────────

const rootNode: ProcessNode = {
  id: "n0",
  type: "task_root",
  parent_id: null,
  title: "中国是否让地球变得更可持续？",
  sub: "学生带进来的任务",
};

const cardNode: ProcessNode = {
  id: "n1",
  type: "card_use",
  parent_id: "n0",
  title: "SIFT×CRAAP 信息核查",
  sub: "进行中",
};

// ─── TreePanel tests ─────────────────────────────────────────────────────────

describe("TreePanel", () => {
  it("shows header 过程树 when open", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode]} />);
    expect(screen.getByText("过程树")).toBeInTheDocument();
  });

  it("shows 只读 badge when open", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode]} />);
    expect(screen.getByText("只读")).toBeInTheDocument();
  });

  it("shows footer 边做边长 · 随评估归并枝节 when open", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode]} />);
    expect(screen.getByText("边做边长 · 随评估归并枝节")).toBeInTheDocument();
  });

  it("shows empty-state hint when only root node (nodes.length === 1)", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode]} />);
    // Empty state: the footer copy is the hint
    expect(screen.getByText("边做边长 · 随评估归并枝节")).toBeInTheDocument();
    // The root node title should NOT be rendered as a node row (only 1 node)
    expect(screen.queryByText("任务根")).not.toBeInTheDocument();
  });

  it("renders card node title when nodes includes root + card", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode, cardNode]} />);
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });

  it("renders the 工具卡使用 tag for card_use node", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode, cardNode]} />);
    expect(screen.getByText("工具卡使用")).toBeInTheDocument();
  });

  it("renders the task_root node title when nodes.length > 1", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode, cardNode]} />);
    expect(screen.getByText("中国是否让地球变得更可持续？")).toBeInTheDocument();
  });

  it("collapsed rail (open=false) renders 过程树 vertical label", () => {
    render(<TreePanel open={false} onToggle={vi.fn()} nodes={[]} />);
    expect(screen.getByText("过程树")).toBeInTheDocument();
  });

  it("collapsed rail does NOT show 只读 badge", () => {
    render(<TreePanel open={false} onToggle={vi.fn()} nodes={[]} />);
    expect(screen.queryByText("只读")).not.toBeInTheDocument();
  });

  it("collapsed rail renders the expand button with 展开过程树 label", () => {
    render(<TreePanel open={false} onToggle={vi.fn()} nodes={[]} />);
    expect(screen.getByRole("button", { name: /展开过程树/ })).toBeInTheDocument();
  });

  it("header has collapse button with 折叠过程树 label when open", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode]} />);
    expect(screen.getByRole("button", { name: /折叠过程树/ })).toBeInTheDocument();
  });

  it("sub text is shown for card node when present", () => {
    render(<TreePanel open={true} onToggle={vi.fn()} nodes={[rootNode, cardNode]} />);
    expect(screen.getByText("进行中")).toBeInTheDocument();
  });
});
