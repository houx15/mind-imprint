import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Badge, CountBadge, Tabs, Segmented, Progress, Stepper, Tooltip } from "@/ui/feedback";

describe("Badge", () => {
  it("applies the progress tone classes (accent-50 / accent-700)", () => {
    render(<Badge tone="progress">进行中</Badge>);
    const el = screen.getByText("进行中");
    expect(el.className).toContain("bg-mk-accent-50");
    expect(el.className).toContain("text-mk-accent-700");
  });

  it("applies the done tone classes (success)", () => {
    render(<Badge tone="done">已完成</Badge>);
    const el = screen.getByText("已完成");
    expect(el.className).toContain("bg-mk-success-bg");
    expect(el.className).toContain("text-mk-success");
  });

  it("applies the draft tone classes (neutral + border)", () => {
    render(<Badge tone="draft">草稿</Badge>);
    const el = screen.getByText("草稿");
    expect(el.className).toContain("bg-mk-paper");
    expect(el.className).toContain("text-mk-muted");
    expect(el.className).toContain("border-mk-border");
  });

  it("applies the pending tone classes (warning)", () => {
    render(<Badge tone="pending">待处理</Badge>);
    const el = screen.getByText("待处理");
    expect(el.className).toContain("bg-mk-warning-bg");
    expect(el.className).toContain("text-mk-warning");
  });
});

describe("CountBadge", () => {
  it("renders the accent solid fill with the number", () => {
    render(<CountBadge n={3} />);
    const el = screen.getByText("3");
    expect(el.className).toContain("bg-mk-accent");
    expect(el.className).toContain("text-white");
  });
});

describe("Tabs", () => {
  const tabs = [
    { key: "a", label: "探索" },
    { key: "b", label: "写作" },
  ];

  it("marks the active tab aria-selected and inactive ones not selected", () => {
    render(<Tabs tabs={tabs} value="a" onChange={() => {}} />);
    expect(screen.getByRole("tab", { name: "探索" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "写作" })).toHaveAttribute("aria-selected", "false");
  });

  it("fires onChange with the clicked tab's key", async () => {
    const onChange = vi.fn();
    render(<Tabs tabs={tabs} value="a" onChange={onChange} />);
    await userEvent.click(screen.getByRole("tab", { name: "写作" }));
    expect(onChange).toHaveBeenCalledWith("b");
  });

  it("applies accent underline + ink text to the active tab, muted to inactive", () => {
    render(<Tabs tabs={tabs} value="a" onChange={() => {}} />);
    const active = screen.getByRole("tab", { name: "探索" });
    const inactive = screen.getByRole("tab", { name: "写作" });
    expect(active.className).toContain("border-mk-accent");
    expect(active.className).toContain("text-mk-ink");
    expect(inactive.className).toContain("text-mk-muted");
    expect(inactive.className).not.toContain("border-mk-accent");
  });
});

describe("Segmented", () => {
  const options = [
    { value: "grid", label: "网格" },
    { value: "list", label: "列表" },
  ];

  it("fires onChange with the clicked option's value", async () => {
    const onChange = vi.fn();
    render(<Segmented options={options} value="grid" onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "列表" }));
    expect(onChange).toHaveBeenCalledWith("list");
  });

  it("marks the active option aria-pressed with the white thumb styling", () => {
    render(<Segmented options={options} value="list" onChange={() => {}} />);
    const active = screen.getByRole("button", { name: "列表" });
    const inactive = screen.getByRole("button", { name: "网格" });
    expect(active).toHaveAttribute("aria-pressed", "true");
    expect(active.className).toContain("bg-mk-surface");
    expect(inactive).toHaveAttribute("aria-pressed", "false");
  });
});

describe("Progress", () => {
  it("clamps values above 100 to a 100% fill width", () => {
    render(<Progress value={150} />);
    const bar = screen.getByRole("progressbar");
    expect(bar).toHaveAttribute("aria-valuenow", "100");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("100%");
  });

  it("clamps negative values to a 0% fill width", () => {
    render(<Progress value={-10} />);
    const bar = screen.getByRole("progressbar");
    expect(bar).toHaveAttribute("aria-valuenow", "0");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("0%");
  });

  it("renders an in-range value as-is", () => {
    render(<Progress value={42} />);
    const bar = screen.getByRole("progressbar");
    expect(bar).toHaveAttribute("aria-valuenow", "42");
    const fill = bar.firstElementChild as HTMLElement;
    expect(fill.style.width).toBe("42%");
  });
});

describe("Stepper", () => {
  const steps = ["溯源", "论证", "让步", "定稿"];

  it("marks steps before current as done, the current step as current, and the rest as todo", () => {
    render(<Stepper steps={steps} current={2} />);
    const doneMarkers = document.querySelectorAll('[data-state="done"]');
    const currentMarkers = document.querySelectorAll('[data-state="current"]');
    const todoMarkers = document.querySelectorAll('[data-state="todo"]');
    expect(doneMarkers).toHaveLength(2);
    expect(currentMarkers).toHaveLength(1);
    expect(todoMarkers).toHaveLength(1);
  });

  it("gives done steps the accent fill + check icon, current the accent ring, todo the grey fill", () => {
    render(<Stepper steps={steps} current={2} />);
    const done = document.querySelectorAll('[data-state="done"]')[0] as HTMLElement;
    const current = document.querySelector('[data-state="current"]') as HTMLElement;
    const todo = document.querySelector('[data-state="todo"]') as HTMLElement;
    expect(done.className).toContain("bg-mk-accent");
    expect(done.querySelector("svg")).toBeTruthy();
    expect(current.className).toContain("ring-mk-accent");
    expect(todo.className).toContain("bg-mk-border");
  });

  it("renders all step labels", () => {
    render(<Stepper steps={steps} current={1} />);
    steps.forEach((label) => expect(screen.getByText(label)).toBeInTheDocument());
  });
});

describe("Tooltip", () => {
  it("renders the trigger content and a tooltip bubble with dark styling", () => {
    render(
      <Tooltip label="点击复制">
        <button type="button">按钮</button>
      </Tooltip>,
    );
    expect(screen.getByText("按钮")).toBeInTheDocument();
    const bubble = screen.getByRole("tooltip");
    expect(bubble).toHaveTextContent("点击复制");
    expect(bubble.className).toContain("bg-mk-ink");
    expect(bubble.className).toContain("text-white");
  });

  it("is keyboard-focus reachable via the wrapping trigger", () => {
    render(
      <Tooltip label="提示">
        <span>内容</span>
      </Tooltip>,
    );
    const wrapper = screen.getByText("内容").parentElement as HTMLElement;
    expect(wrapper).toHaveAttribute("tabIndex", "0");
  });
});
