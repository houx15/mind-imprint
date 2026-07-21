import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { Scale } from "./Scale";
import type { Bucket } from "./serialize";
import type { ScaleState } from "@mind-imprint/contracts";

const stops: Bucket[] = [
  { id: "个人猜测", label: "个人猜测", hint: "只有直觉，没有证据" },
  { id: "有据推断", label: "有据推断", hint: "有证据，但推理仍可争议" },
  { id: "强证据", label: "强证据", hint: "多个独立来源支撑" },
  { id: "科学共识", label: "科学共识", hint: "领域内已达成共识" },
  { id: "逻辑必然", label: "逻辑必然", hint: "由定义或推理必然为真" },
];
const itemPrompt = "你的结论是什么？";
const reasonPrompt = "为什么是这个位置？支撑它的证据类型是什么？";
const rewritePrompt = "用与该位置相称的语气词重写结论句（可能 / 大概 / 很可能 / 几乎确定 / 必然）";

const oneItemState: ScaleState = {
  items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "", reason: "", author: "student" }],
  rewrite: "",
};

test("renders every stop in declaration order", () => {
  render(
    <Scale stops={stops} state={oneItemState} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  // the axis header renders one label per stop, plus the per-item chip row
  // repeats the same labels — every declared stop must appear at least once,
  // and in declaration order relative to each other (leftmost = first stop).
  const labels = stops.map((s) => s.label);
  for (const label of labels) {
    expect(screen.getAllByText(label).length).toBeGreaterThan(0);
  }
  const allText = document.body.textContent ?? "";
  let cursor = -1;
  for (const label of labels) {
    const idx = allText.indexOf(label, cursor + 1);
    expect(idx).toBeGreaterThan(cursor);
    cursor = idx;
  }
});

test("clicking a stop assigns it to the item and calls onChange", () => {
  const onChange = vi.fn();
  render(
    <Scale stops={stops} state={oneItemState} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getAllByRole("button", { name: "有据推断" })[0]!);
  expect(onChange).toHaveBeenCalledWith({
    items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "有据推断", reason: "", author: "student" }],
    rewrite: "",
  });
});

test("＋ 添加一句 appends an empty student-authored row", () => {
  const onChange = vi.fn();
  render(
    <Scale stops={stops} state={{ items: [], rewrite: "" }} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getByText("＋ 添加一句"));
  expect(onChange).toHaveBeenCalledTimes(1);
  const call = onChange.mock.calls[0][0] as ScaleState;
  expect(call.items).toHaveLength(1);
  expect(call.items[0]).toMatchObject({ text: "", stop: "", reason: "", author: "student" });
  expect(call.rewrite).toBe("");
});

test("the remove control drops that row", () => {
  const onChange = vi.fn();
  const twoItemState: ScaleState = {
    items: [
      { id: "i1", text: "中国的碳排放正在下降", stop: "", reason: "", author: "student" },
      { id: "i2", text: "地球在变暖", stop: "", reason: "", author: "student" },
    ],
    rewrite: "",
  };
  render(
    <Scale stops={stops} state={twoItemState} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getByLabelText("删除第 1 句"));
  expect(onChange).toHaveBeenCalledWith({ items: [twoItemState.items[1]], rewrite: "" });
});

test("the rewrite textarea calls onChange with the updated rewrite text", () => {
  const onChange = vi.fn();
  render(
    <Scale stops={stops} state={oneItemState} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.change(screen.getByPlaceholderText(rewritePrompt), { target: { value: "中国的碳排放大概正在下降" } });
  expect(onChange).toHaveBeenCalledWith({ ...oneItemState, rewrite: "中国的碳排放大概正在下降" });
});

test("rewritePrompt is not rendered when blank, and the rewrite clause does not gate the lock", () => {
  const completeNoRewrite: ScaleState = {
    items: [{ id: "i1", text: "地球在变暖", stop: "科学共识", reason: "IPCC 多份报告共识", author: "student" }],
    rewrite: "",
  };
  render(
    <Scale stops={stops} state={completeNoRewrite} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt="" onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.queryByPlaceholderText(rewritePrompt)).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "完成并钉到过程树" })).not.toBeDisabled();
});

// Real fill-then-lock flow, mirroring Sort.test.tsx: drive the rendered
// controls, rerender with the state onChange produced, and assert the lock
// button only enables once placedCount reaches minItems AND every filled row
// also carries a stop + non-empty reason AND (when rewritePrompt is
// non-empty) the rewrite field is non-blank.
test("lock is gated below minItems, gated on stop+reason completeness, gated on blank rewrite, then enables and fires onLock", () => {
  const onLock = vi.fn();
  let state: ScaleState = {
    items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "", reason: "", author: "student" }],
    rewrite: "",
  };
  const onChange = (s: ScaleState) => {
    state = s;
  };
  const { rerender } = render(
    <Scale stops={stops} state={state} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={onChange} onLock={onLock} onSkip={() => {}} />,
  );
  const rerenderWithState = () =>
    rerender(<Scale stops={stops} state={state} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={onChange} onLock={onLock} onSkip={() => {}} />);

  const lock = screen.getByRole("button", { name: "完成并钉到过程树" });
  expect(lock).toBeDisabled(); // stop unset

  fireEvent.click(screen.getAllByRole("button", { name: "有据推断" })[0]!);
  rerenderWithState();
  expect(lock).toBeDisabled(); // stop set, reason still blank

  fireEvent.change(screen.getByPlaceholderText(reasonPrompt), { target: { value: "有统计数据但口径可争议" } });
  rerenderWithState();
  expect(lock).toBeDisabled(); // item complete, but rewrite still blank

  fireEvent.change(screen.getByPlaceholderText(rewritePrompt), { target: { value: "中国的碳排放大概正在下降" } });
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
  const state: ScaleState = {
    items: [
      { id: "i1", text: "地球在变暖", stop: "科学共识", reason: "IPCC 多份报告共识", author: "student" },
      // started, placed, but not yet reasoned about
      { id: "i2", text: "人类会在火星定居", stop: "个人猜测", reason: "", author: "student" },
    ],
    rewrite: "地球大概率正在变暖",
  };
  render(
    <Scale stops={stops} state={state} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByRole("button", { name: "完成并钉到过程树" })).not.toBeDisabled();
});

test("skip fires onSkip without requiring completion", () => {
  const onSkip = vi.fn();
  render(
    <Scale stops={stops} state={{ items: [], rewrite: "" }} minItems={1} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={() => {}} onLock={() => {}} onSkip={onSkip} />,
  );
  fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
  expect(onSkip).toHaveBeenCalledTimes(1);
});

test("progress line is plain text, never a bar or score", () => {
  render(
    <Scale stops={stops} state={oneItemState} minItems={3} itemPrompt={itemPrompt} reasonPrompt={reasonPrompt} rewritePrompt={rewritePrompt} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByText("已放置 0 句 · 还差 3 句")).toBeInTheDocument();
  expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
});
