import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Illustration, EmptyState } from "@/ui/Illustration";

describe("Illustration", () => {
  it("renders an img with a non-empty src for a known name", () => {
    const { container } = render(<Illustration name="reading" />);
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img?.getAttribute("src")).toBeTruthy();
  });

  it("stamps the tone prop as data-tone", () => {
    const { container } = render(<Illustration name="reading" tone="peach" />);
    expect(container.querySelector("img")).toHaveAttribute("data-tone", "peach");
  });
});

describe("EmptyState", () => {
  it("renders title, body, and the action button; clicking fires onClick", async () => {
    const onClick = vi.fn();
    render(
      <EmptyState
        illustration="emptyProjects"
        title="快来创建你的第一个写作项目吧！"
        body="从一个真实任务开始，AI 会陪你一起想。"
        action={{ label: "新建项目", onClick }}
      />,
    );
    expect(screen.getByText("快来创建你的第一个写作项目吧！")).toBeInTheDocument();
    expect(screen.getByText("从一个真实任务开始，AI 会陪你一起想。")).toBeInTheDocument();
    const button = screen.getByRole("button", { name: "新建项目" });
    await userEvent.click(button);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("renders no button when action is omitted", () => {
    render(<EmptyState illustration="emptyProjects" title="标题" body="正文" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
