import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { Sort } from "./Sort";
import type { Bucket } from "./serialize";
import type { SortState } from "@mind-imprint/contracts";

const buckets: Bucket[] = [
  { id: "事实", label: "可查证的事实", hint: "能被独立核查的陈述" },
  { id: "观点", label: "需论证的观点", hint: "成立需要给出理由" },
  { id: "价值判断", label: "藏价值的判断", hint: "带「应该/好坏」，藏着价值排序" },
];
const itemPrompt = "把一句话贴进来或写下来";
const reasonPrompt = "检验/理由（它能被查证吗？需要理由吗？藏了「应该」吗？）";

const twoItemState: SortState = {
  items: [
    { id: "i1", text: "中国是碳排放第一大国", bucket: "", reason: "", author: "student" },
    { id: "i2", text: "中国应该做得更多", bucket: "", reason: "", author: "student" },
  ],
};

test("renders one row per item and every bucket chip", () => {
  render(
    <Sort buckets={buckets} state={twoItemState} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getAllByPlaceholderText(itemPrompt)).toHaveLength(2);
  expect(screen.getAllByPlaceholderText(reasonPrompt)).toHaveLength(2);
  expect(screen.getAllByText("可查证的事实")).toHaveLength(2);
  expect(screen.getAllByText("需论证的观点")).toHaveLength(2);
  expect(screen.getAllByText("藏价值的判断")).toHaveLength(2);
});

test("picking a bucket calls onChange with that item's bucket set", () => {
  const onChange = vi.fn();
  render(
    <Sort buckets={buckets} state={twoItemState} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getAllByText("需论证的观点")[0]!);
  expect(onChange).toHaveBeenCalledWith({
    items: [
      { id: "i1", text: "中国是碳排放第一大国", bucket: "观点", reason: "", author: "student" },
      { id: "i2", text: "中国应该做得更多", bucket: "", reason: "", author: "student" },
    ],
  });
});

test("添加一句 appends an empty student-authored row", () => {
  const onChange = vi.fn();
  render(
    <Sort buckets={buckets} state={{ items: [] }} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getByText("添加一句"));
  expect(onChange).toHaveBeenCalledTimes(1);
  const call = onChange.mock.calls[0][0] as SortState;
  expect(call.items).toHaveLength(1);
  expect(call.items[0]).toMatchObject({ text: "", bucket: "", reason: "", author: "student" });
});

test("the remove control drops that row", () => {
  const onChange = vi.fn();
  render(
    <Sort buckets={buckets} state={twoItemState} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getByLabelText("删除第 1 句"));
  expect(onChange).toHaveBeenCalledWith({ items: [twoItemState.items[1]] });
});

// Real fill-then-lock flow, mirroring Graph.test.tsx: drive the rendered
// controls, rerender with the state onChange produced, and assert the lock
// button only enables once classifiedCount reaches minItems AND every filled
// row also carries a bucket + non-empty reason.
test("lock is gated below minItems and gated on bucket+reason completeness, then enables and fires onLock", () => {
  const onLock = vi.fn();
  let state: SortState = {
    items: [{ id: "i1", text: "中国是碳排放第一大国", bucket: "", reason: "", author: "student" }],
  };
  const onChange = (s: SortState) => {
    state = s;
  };
  const { rerender } = render(
    <Sort buckets={buckets} state={state} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={onChange} onLock={onLock} onSkip={() => {}} />,
  );
  const rerenderWithState = () =>
    rerender(<Sort buckets={buckets} state={state} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={onChange} onLock={onLock} onSkip={() => {}} />);

  const lock = screen.getByRole("button", { name: "完成并钉到过程树" });
  expect(lock).toBeDisabled(); // 0 classified, minItems 2

  // classify the one row, but leave its reason blank.
  fireEvent.click(screen.getByText("可查证的事实"));
  rerenderWithState();
  expect(lock).toBeDisabled(); // 0 complete rows (no reason yet) < minItems 2

  // add a second row and classify it too, both reasons still blank.
  fireEvent.click(screen.getByText("添加一句"));
  rerenderWithState();
  const secondTextBox = screen.getAllByPlaceholderText(itemPrompt)[1]!;
  fireEvent.change(secondTextBox, { target: { value: "中国应该做得更多" } });
  rerenderWithState();
  fireEvent.click(screen.getAllByText("藏价值的判断")[1]!);
  rerenderWithState();
  expect(lock).toBeDisabled(); // 2 bucketed, but 0 complete — reasons are blank

  // fill both reasons.
  const reasonBoxes = screen.getAllByPlaceholderText(reasonPrompt);
  fireEvent.change(reasonBoxes[0]!, { target: { value: "可用官方数据核查" } });
  rerenderWithState();
  fireEvent.change(screen.getAllByPlaceholderText(reasonPrompt)[1]!, { target: { value: "藏着「应该」的价值排序" } });
  rerenderWithState();
  expect(lock).not.toBeDisabled();

  fireEvent.click(lock);
  expect(onLock).toHaveBeenCalledTimes(1);
});

// The lock gate must count COMPLETE rows the way the server's bucketedCount
// does, not demand that EVERY started row be complete. A student who has
// finished her minimum and then starts one more row she has not reasoned
// about yet must not be walled out of a lock the backend would accept.
test("an extra half-filled row does not block the lock once minItems rows are complete", () => {
  const state: SortState = {
    items: [
      { id: "i1", text: "中国是碳排放第一大国", bucket: "事实", reason: "可用官方数据核查", author: "student" },
      { id: "i2", text: "中国应该做得更多", bucket: "价值判断", reason: "藏着「应该」的价值排序", author: "student" },
      // started, classified, but not yet reasoned about
      { id: "i3", text: "中国的治理是认真的", bucket: "观点", reason: "", author: "student" },
    ],
  };
  render(
    <Sort buckets={buckets} state={state} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByRole("button", { name: "完成并钉到过程树" })).not.toBeDisabled();
});

test("skip fires onSkip without requiring completion", () => {
  const onSkip = vi.fn();
  render(
    <Sort buckets={buckets} state={{ items: [] }} minItems={2} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={() => {}} onLock={() => {}} onSkip={onSkip} />,
  );
  fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
  expect(onSkip).toHaveBeenCalledTimes(1);
});

test("progress line is plain text, never a bar or score", () => {
  render(
    <Sort buckets={buckets} state={twoItemState} minItems={3} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByText("已归类并写下理由 0 句 · 还差 3 句")).toBeInTheDocument();
  expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
});

// Whole-branch review IMPORTANT 2: the progress line used to count
// classifiedCount while the lock counted completeCount, so this exact state —
// fact-opinion-value's own min_items of 3, all 3 sentences bucketed, only 2
// reasoned about — rendered "已归类 3 句 · 还差 0 句" next to a greyed-out
// lock button with nothing on screen saying a 理由 was required. A dead end.
test("progress line counts what the lock actually needs: bucketed AND reasoned", () => {
  const state: SortState = {
    items: [
      { id: "i1", text: "中国是碳排放第一大国", bucket: "事实", reason: "可用官方数据核查", author: "student" },
      { id: "i2", text: "中国应该做得更多", bucket: "价值判断", reason: "藏着「应该」的价值排序", author: "student" },
      // bucketed, but she has not written a 理由 for this one yet
      { id: "i3", text: "中国的治理是认真的", bucket: "观点", reason: "", author: "student" },
    ],
  };
  render(
    <Sort buckets={buckets} state={state} minItems={3} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByRole("button", { name: "完成并钉到过程树" })).toBeDisabled();
  expect(screen.getByText("已归类并写下理由 2 句 · 还差 1 句")).toBeInTheDocument();
});
