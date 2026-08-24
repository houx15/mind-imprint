import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { demoMockFor } from "@/tour/mocks";

describe("demoMockFor", () => {
  it("question-card: shows the 提问卡 pill, the 4 methodology steps, and sample chat bubbles, with a disabled non-interactive input", () => {
    render(<>{demoMockFor("question-card")}</>);

    expect(screen.getByText("提问卡")).toBeInTheDocument();
    expect(screen.getByText("从大题目，问出一个值得研究的问题")).toBeInTheDocument();

    expect(screen.getByText("拆解")).toBeInTheDocument();
    expect(screen.getByText("追问")).toBeInTheDocument();
    expect(screen.getByText("连接")).toBeInTheDocument();
    expect(screen.getByText("连不上就去探索")).toBeInTheDocument();

    expect(screen.getByText("可能是「可持续」吧，它可以指很多方面。")).toBeInTheDocument();
    expect(
      screen.getByText("「中国是否让地球更可持续」这个题目挺大的。我们先拆一下——你觉得这里面哪个词最需要先说清楚？"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("很好。那在你最关心的那个方面——比如能源、碳排放、还是生物多样性——你更想聚焦哪一个？先选一个，我们把题目缩小。"),
    ).toBeInTheDocument();

    const input = screen.getByPlaceholderText("用你自己的话说……");
    expect(input).toBeDisabled();
    const send = screen.getByRole("button", { name: "发送" });
    expect(send).toBeDisabled();
  });
});
