import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, test, vi } from "vitest";
import { Matrix } from "./Matrix";
import type { Col } from "./serialize";
import type { MatrixState } from "@mind-imprint/contracts";

const cols: Col[] = [
  { id: "position", label: "立场主张", q: "这个视角主张什么？用一句话说清。" },
  { id: "grounds", label: "依据", q: "它凭什么这么主张？它最强的依据是什么？" },
  { id: "blind_spot", label: "盲区", q: "这个视角看不见什么？它绕开了哪个事实？" },
];
const rowPrompt = "这是谁的视角？（例：地方政府 / 环保组织 / 受影响居民 / 能源企业）";

const twoRowState: MatrixState = {
  rows: [
    { id: "r1", label: "地方政府", cells: { position: "", grounds: "", blind_spot: "" }, author: "student" },
    { id: "r2", label: "环保组织", cells: { position: "", grounds: "", blind_spot: "" }, author: "student" },
  ],
};

test("renders one card per row and one field per column, with col.label visible and col.q as placeholder", () => {
  render(
    <Matrix cols={cols} state={twoRowState} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getAllByPlaceholderText(rowPrompt)).toHaveLength(2);
  expect(screen.getAllByText("立场主张")).toHaveLength(2);
  expect(screen.getAllByText("依据")).toHaveLength(2);
  expect(screen.getAllByText("盲区")).toHaveLength(2);
  expect(screen.getAllByPlaceholderText(cols[0]!.q)).toHaveLength(2);
  expect(screen.getAllByPlaceholderText(cols[1]!.q)).toHaveLength(2);
  expect(screen.getAllByPlaceholderText(cols[2]!.q)).toHaveLength(2);
});

test("typing a cell calls onChange with that row's cells[colId] set", () => {
  const onChange = vi.fn();
  render(
    <Matrix cols={cols} state={twoRowState} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  const groundsBoxes = screen.getAllByPlaceholderText(cols[1]!.q);
  fireEvent.change(groundsBoxes[0]!, { target: { value: "GDP 与就业指标" } });
  expect(onChange).toHaveBeenCalledWith({
    rows: [
      { id: "r1", label: "地方政府", cells: { position: "", grounds: "GDP 与就业指标", blind_spot: "" }, author: "student" },
      { id: "r2", label: "环保组织", cells: { position: "", grounds: "", blind_spot: "" }, author: "student" },
    ],
  });
});

test("typing a row's label calls onChange with that row's label set", () => {
  const onChange = vi.fn();
  render(
    <Matrix cols={cols} state={twoRowState} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  const labelBoxes = screen.getAllByPlaceholderText(rowPrompt);
  fireEvent.change(labelBoxes[1]!, { target: { value: "受影响居民" } });
  expect(onChange).toHaveBeenCalledWith({
    rows: [
      twoRowState.rows[0],
      { id: "r2", label: "受影响居民", cells: { position: "", grounds: "", blind_spot: "" }, author: "student" },
    ],
  });
});

test("添加一个视角 appends an empty student-authored row with every column pre-keyed", () => {
  const onChange = vi.fn();
  render(<Matrix cols={cols} state={{ rows: [] }} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={onChange} onLock={() => {}} onSkip={() => {}} />);
  fireEvent.click(screen.getByText("添加一个视角"));
  expect(onChange).toHaveBeenCalledTimes(1);
  const call = onChange.mock.calls[0][0] as MatrixState;
  expect(call.rows).toHaveLength(1);
  expect(call.rows[0]).toMatchObject({
    label: "",
    cells: { position: "", grounds: "", blind_spot: "" },
    author: "student",
  });
});

test("the remove control drops that row", () => {
  const onChange = vi.fn();
  render(
    <Matrix cols={cols} state={twoRowState} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getByLabelText("删除第 1 个视角"));
  expect(onChange).toHaveBeenCalledWith({ rows: [twoRowState.rows[1]] });
});

test("lock is disabled at 1 complete row when minItems is 2, enabled at 2", () => {
  const onLock = vi.fn();
  let state: MatrixState = {
    rows: [{ id: "r1", label: "地方政府", cells: { position: "发展优先", grounds: "GDP 指标", blind_spot: "环境成本" }, author: "student" }],
  };
  const onChange = (s: MatrixState) => {
    state = s;
  };
  const { rerender } = render(
    <Matrix cols={cols} state={state} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={onChange} onLock={onLock} onSkip={() => {}} />,
  );
  const rerenderWithState = () =>
    rerender(<Matrix cols={cols} state={state} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={onChange} onLock={onLock} onSkip={() => {}} />);

  const lock = screen.getByRole("button", { name: "完成并钉到过程树" });
  expect(lock).toBeDisabled(); // 1 complete row < minItems 2

  // add a second row and fully complete it too.
  fireEvent.click(screen.getByText("添加一个视角"));
  rerenderWithState();
  const labelBoxes = screen.getAllByPlaceholderText(rowPrompt);
  fireEvent.change(labelBoxes[1]!, { target: { value: "环保组织" } });
  rerenderWithState();
  const groundsBoxes = screen.getAllByPlaceholderText(cols[1]!.q);
  fireEvent.change(groundsBoxes[1]!, { target: { value: "长期健康数据" } });
  rerenderWithState();
  expect(lock).toBeDisabled(); // second row labeled + one cell filled, still incomplete

  fireEvent.change(screen.getAllByPlaceholderText(cols[0]!.q)[1]!, { target: { value: "治理优先" } });
  rerenderWithState();
  fireEvent.change(screen.getAllByPlaceholderText(cols[2]!.q)[1]!, { target: { value: "短期就业冲击" } });
  rerenderWithState();
  expect(lock).not.toBeDisabled();

  fireEvent.click(lock);
  expect(onLock).toHaveBeenCalledTimes(1);
});

// The lock gate must count COMPLETE rows the way the server does — a row is
// complete once its label is non-blank and every column has a non-blank
// answer, and the card is done once that COUNT reaches minItems, regardless
// of how many additional incomplete rows also exist. Task 6 shipped this bug
// (an `every(isComplete)` gate that walled a student out of a lock the
// backend would accept) and had to be fixed; this is the regression test.
test("an extra half-filled row does not block the lock once minItems rows are complete", () => {
  const state: MatrixState = {
    rows: [
      { id: "r1", label: "地方政府", cells: { position: "发展优先", grounds: "GDP 指标", blind_spot: "环境成本" }, author: "student" },
      { id: "r2", label: "环保组织", cells: { position: "治理优先", grounds: "长期健康数据", blind_spot: "短期就业冲击" }, author: "student" },
      // started a third perspective, named it, but has not filled any column yet.
      { id: "r3", label: "受影响居民", cells: { position: "", grounds: "", blind_spot: "" }, author: "student" },
    ],
  };
  render(
    <Matrix cols={cols} state={state} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByRole("button", { name: "完成并钉到过程树" })).not.toBeDisabled();
});

test("skip fires onSkip without requiring completion", () => {
  const onSkip = vi.fn();
  render(<Matrix cols={cols} state={{ rows: [] }} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={() => {}} onLock={() => {}} onSkip={onSkip} />);
  fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
  expect(onSkip).toHaveBeenCalledTimes(1);
});

test("progress line is plain text, never a bar or score", () => {
  render(
    <Matrix cols={cols} state={twoRowState} minItems={3} rowPrompt={rowPrompt} rowNoun="视角" onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByText("已完成 0 个视角 · 还差 3 个")).toBeInTheDocument();
  expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
});

// Whole-branch review IMPORTANT 3: a row's identity on the server IS the label
// the student wrote — anchors are grouped by Anchor.Quote — so two rows both
// labelled 政府 collapse into ONE row there. Counting them as two here enabled
// the lock, the submit then found matrix_complete false, and the student got
// back done{status:"active"} with no error and no explanation: the card stuck
// open forever with no way to tell why.
test("two identically-labelled complete rows count as one — the lock stays disabled", () => {
  const state: MatrixState = {
    rows: [
      { id: "r1", label: "政府", cells: { position: "发展优先", grounds: "GDP 指标", blind_spot: "环境成本" }, author: "student" },
      // same label (trailing whitespace and all) — the server sees ONE row.
      { id: "r2", label: " 政府 ", cells: { position: "治理优先", grounds: "环境公报", blind_spot: "就业冲击" }, author: "student" },
    ],
  };
  render(
    <Matrix cols={cols} state={state} minItems={2} rowPrompt={rowPrompt} rowNoun="视角" onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByRole("button", { name: "完成并钉到过程树" })).toBeDisabled();
  expect(screen.getByText("已完成 1 个视角 · 还差 1 个")).toBeInTheDocument();
});

describe("Matrix row-noun", () => {
  const testCols = [{ id: "a", label: "A", q: "qa" }];

  it("uses the given rowNoun in the add button and progress line", () => {
    render(
      <Matrix cols={testCols} state={{ rows: [] }} minItems={1} rowPrompt="p" rowNoun="检索方向"
        onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
    );
    expect(screen.getByText(/添加一个检索方向/)).toBeInTheDocument();
    expect(screen.getByText(/已完成 0 个检索方向/)).toBeInTheDocument();
  });

  it("defaults nothing — the noun is always supplied by the host (perspective-matrix passes 视角)", () => {
    render(
      <Matrix cols={testCols} state={{ rows: [] }} minItems={1} rowPrompt="p" rowNoun="视角"
        onChange={() => {}} onLock={() => {}} onSkip={() => {}} />,
    );
    expect(screen.getByText(/添加一个视角/)).toBeInTheDocument();
  });
});
