import { render, screen, fireEvent } from "@testing-library/react";
import { test, expect, vi } from "vitest";
import { ReviewView } from "./ReviewView";
import type { GaugeFx } from "../state";

const gauges: GaugeFx[] = [
  { code: "表D", name: "来源与证据", lit: 3, total: 4, note: "孤儿证据没接上", level: "partial" },
  { code: "表E", name: "分析", lit: 0, total: 4, note: "", level: "empty" },
  { code: "表F", name: "评估", lit: 3, total: 3, note: "", level: "full" },
  { code: "表H", name: "表达与组织", lit: 3, total: 3, note: "", level: "full" },
];

const baseTerminalProps = {
  canFinish: false,
  finished: false,
  finishing: false,
  finishError: null,
  onFinish: () => {},
};

test("renders code and name for each table", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  expect(screen.getByText("表D")).toBeInTheDocument();
  expect(screen.getByText("来源与证据")).toBeInTheDocument();
});

test("renders the summary line from lamp sums", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  // Σlit = 3+0+3+3 = 9, Σtotal = 4+4+3+3 = 14
  expect(screen.getByText("已点亮 9/14 格")).toBeInTheDocument();
});

test("empty readiness shows 0/N and unlit cards", () => {
  const empty: GaugeFx[] = gauges.map((g) => ({ ...g, lit: 0, note: "", level: "empty" as const }));
  render(<ReviewView gauges={empty} {...baseTerminalProps} />);
  expect(screen.getByText("已点亮 0/14 格")).toBeInTheDocument();
});

test("canFinish renders the 完成任务 · 归档 button and fires onFinish on click", () => {
  const onFinish = vi.fn();
  render(<ReviewView gauges={gauges} canFinish finished={false} finishing={false} finishError={null} onFinish={onFinish} />);
  const button = screen.getByText("完成任务 · 归档");
  expect(button).toBeInTheDocument();
  fireEvent.click(button);
  expect(onFinish).toHaveBeenCalledTimes(1);
});

test("finished renders the inert 已归档 state and no button", () => {
  render(<ReviewView gauges={gauges} canFinish={false} finished finishing={false} finishError={null} onFinish={() => {}} />);
  expect(screen.getByText("已归档 · 成长报告已生成")).toBeInTheDocument();
  expect(screen.queryByText("完成任务 · 归档")).toBeNull();
});

test("neither canFinish nor finished renders no terminal UI", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  expect(screen.queryByText("完成任务 · 归档")).toBeNull();
  expect(screen.queryByText("已归档 · 成长报告已生成")).toBeNull();
});

test("shows finishError text when present", () => {
  render(
    <ReviewView gauges={gauges} canFinish finished={false} finishing={false} finishError="归档失败，请重试" onFinish={() => {}} />,
  );
  expect(screen.getByText("归档失败，请重试")).toBeInTheDocument();
});
