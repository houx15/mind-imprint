import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TreeBody } from "./TreeBody";
import type { ProcessNode } from "./processTree";

const nodes: ProcessNode[] = [
  { id: "root", type: "task_root", parent_id: null, title: "任务根", sub: undefined },
  { id: "n1", type: "card_use", parent_id: "root", title: "用了 CRAAP", sub: "信息素养" },
];

describe("TreeBody", () => {
  it("renders node titles and the growth footer", () => {
    render(<TreeBody nodes={nodes} />);
    expect(screen.getByText("用了 CRAAP")).toBeInTheDocument();
    expect(screen.getByText("边做边长 · 随评估归并枝节")).toBeInTheDocument();
  });
});
