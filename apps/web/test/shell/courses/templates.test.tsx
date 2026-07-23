import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TeachingTemplate } from "@/shell/courses/TeachingTemplate";
import { ChallengeTemplate } from "@/shell/courses/ChallengeTemplate";

describe("course templates", () => {
  it("TeachingTemplate renders title, subtitle caption, and body", () => {
    render(<TeachingTemplate content={{ title: "先别急着信", subtitle: "停一下。", body: ["第一段。", "第二段。"], foreground_asset_id: null }} />);
    expect(screen.getByText("先别急着信")).toBeInTheDocument();
    expect(screen.getByText("停一下。")).toBeInTheDocument();
    expect(screen.getByText("第一段。")).toBeInTheDocument();
  });

  it("ChallengeTemplate renders the prompt + anchored questions, and reveals feedback on submit", () => {
    render(<ChallengeTemplate content={{
      title: "现在轮到你", prompt: "哪句是事实？", reason_hint: "说说理由",
      anchors: [{ id: "a0", material_id: "m0", block_id: "b0", start: 0, end: 0, quote: "美航局发现", dimension: "权威性", author: "ai", question: "作者是权威吗？", answer: "" }],
    }} />);
    expect(screen.getAllByText("现在轮到你").length).toBeGreaterThan(0);
    expect(screen.getByText("作者是权威吗？")).toBeInTheDocument();
    expect(screen.queryByText(/很好，你已经开始像个核查者/)).toBeNull();
    fireEvent.click(screen.getByText("提交我的判断"));
    expect(screen.getByText(/很好，你已经开始像个核查者/)).toBeInTheDocument();
  });
});
