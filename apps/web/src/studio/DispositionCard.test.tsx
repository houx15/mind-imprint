import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DispositionCard } from "./DispositionCard";

describe("DispositionCard (三键处置)", () => {
  it("requires a >=15-rune reason before submit", () => {
    const onDisposition = vi.fn();
    render(<DispositionCard tag="D5" anchor="治理决心主张" body="这条主张还没有素材支撑" onDisposition={onDisposition} />);
    fireEvent.click(screen.getByText("不采纳"));
    const box = screen.getByRole("textbox");
    fireEvent.change(box, { target: { value: "太短" } });
    fireEvent.click(screen.getByText(/提交|钉/));
    expect(onDisposition).not.toHaveBeenCalled();
    fireEvent.change(box, { target: { value: "这条追问和我的方向不一致，我想先按自己的思路推进" } });
    fireEvent.click(screen.getByText(/提交|钉/));
    expect(onDisposition).toHaveBeenCalledWith("reject", expect.stringContaining("方向"));
  });

  it("fires onDisposition with the wire-contract literal \"rewrite\" (not \"revise\") for 我自己改", () => {
    const onDisposition = vi.fn();
    render(<DispositionCard tag="D5" anchor="治理决心主张" body="这条主张还没有素材支撑" onDisposition={onDisposition} />);
    fireEvent.click(screen.getByText("我自己改"));
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "我想先按自己的思路把这段重新组织一下" } });
    fireEvent.click(screen.getByText(/提交|钉/));
    expect(onDisposition).toHaveBeenCalledWith("rewrite", expect.any(String));
  });

  it("renders the tag, anchor and body copy passed in", () => {
    render(
      <DispositionCard
        tag="分析与评估"
        anchor="核心主张"
        body="建议把核心主张收敛为更贴合证据的说法。"
        onDisposition={() => {}}
      />
    );
    expect(screen.getByText("分析与评估")).toBeInTheDocument();
    expect(screen.getByText(/锚定/)).toHaveTextContent("核心主张");
    expect(screen.getByText("建议把核心主张收敛为更贴合证据的说法。")).toBeInTheDocument();
  });

  it("shows a grey hint before typing and a green hint once >=15 runes are written", () => {
    render(<DispositionCard tag="D5" anchor="治理决心主张" body="body" onDisposition={() => {}} />);
    fireEvent.click(screen.getByText("接受"));
    const box = screen.getByRole("textbox");
    expect(screen.getByText(/留一句理由才算数/)).toBeInTheDocument();
    // exactly 15 runes should already flip the hint green
    fireEvent.change(box, { target: { value: "一二三四五六七八九十一二三四五" } });
    expect(screen.getByText(/已记录/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/提交|钉/));
    expect(screen.getByText(/提交|钉/).closest("button")).toBeDefined();
  });
});
