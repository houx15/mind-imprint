import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { Graph } from "./Graph";
import type { GraphState } from "@mind-imprint/contracts";

const slots = [
  { id: "claim", role: "核心主张", needSrc: false, q: "你要论证的核心判断，用一句话说清。" },
  { id: "evidence", role: "支撑证据", needSrc: true, q: "挑一条证据，用自己的话概括它如何支撑主张。" },
];
const lockedSources = [{ id: "m_nasa", name: "NASA Earth Observatory" }];

test("renders one row per slot with its verbatim role and question", () => {
  render(<Graph slots={slots} state={{ nodes: [], edges: [] }} lockedSources={lockedSources} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />);
  expect(screen.getByText("核心主张")).toBeInTheDocument();
  expect(screen.getByText("支撑证据")).toBeInTheDocument();
  // claim (the first slot) starts active by default, so its coach question shows.
  expect(screen.getByText("你要论证的核心判断，用一句话说清。")).toBeInTheDocument();
});

// Real fill-then-lock flow, mirroring StudioAnnotateCard.test.tsx: drive the
// rendered controls (activate a slot, type its textarea, toggle a source
// chip), rerender with the state onChange produced, and assert the lock
// button only enables once every slot clears the same gate the backend's
// slotComplete() enforces (>=12 trimmed runes; needSrc slots need >=1 source).
test("lock is gated until every slot has text and needSrc slots have a source, then submits", () => {
  const onLock = vi.fn();
  let state: GraphState = { nodes: [], edges: [] };
  const onChange = (s: GraphState) => {
    state = s;
  };
  const { rerender } = render(
    <Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={onLock} onSkip={() => {}} />,
  );
  const rerenderWithState = () =>
    rerender(<Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={onLock} onSkip={() => {}} />);

  const lock = screen.getByRole("button", { name: /锁定|完成/ });
  expect(lock).toBeDisabled();

  // claim starts active — fill its textarea (>=12 runes, needSrc: false).
  const claimBox = screen.getByPlaceholderText("用你自己的话写……");
  fireEvent.change(claimBox, { target: { value: "核心判断一句话说清楚不能少于十二个字。" } });
  rerenderWithState();
  expect(lock).toBeDisabled(); // evidence still untouched

  // open the evidence slot.
  fireEvent.click(screen.getByText("支撑证据"));
  rerenderWithState();

  // evidence needs a source — toggle the one locked source chip.
  fireEvent.click(screen.getByText("NASA Earth Observatory"));
  rerenderWithState();
  expect(lock).toBeDisabled(); // source picked, but evidence text still empty

  // fill evidence's textarea (>=12 runes).
  const evidenceBox = screen.getByPlaceholderText("用你自己的话写……");
  fireEvent.change(evidenceBox, { target: { value: "证据如何支撑主张写满十二个字以上。" } });
  rerenderWithState();
  expect(lock).not.toBeDisabled();

  fireEvent.click(lock);
  expect(onLock).toHaveBeenCalledTimes(1);

  // The state onChange accumulated is exactly what graphStateToAnchors would
  // serialize: two student-authored nodes + one cites edge from evidence.
  const claimNode = state.nodes.find((n) => n.id === "claim");
  const evidenceNode = state.nodes.find((n) => n.id === "evidence");
  expect(claimNode).toEqual({ id: "claim", type: "claim", text: "核心判断一句话说清楚不能少于十二个字。", author: "student" });
  expect(evidenceNode).toEqual({ id: "evidence", type: "evidence", text: "证据如何支撑主张写满十二个字以上。", author: "student" });
  expect(state.edges).toContainEqual(expect.objectContaining({ from: "evidence", to: "m_nasa", type: "cites" }));
});

// Isolates the needSrc-source clause of canLock: every slot's text is filled
// (>=12 runes), but the evidence slot's REQUIRED source is left unselected —
// so the ONLY thing gating the lock is the missing source. If canLock ever
// stopped requiring hasSource on needSrc slots, this test bites (the button
// would wrongly enable). This is the guard against a stuck-active card: the
// student locks, but the server's slotComplete refuses a needSrc slot with no
// source anchor, leaving the card active with nothing minted.
test("lock stays disabled when a needSrc slot has text but no source, and enables once a source is added", () => {
  const onLock = vi.fn();
  let state: GraphState = { nodes: [], edges: [] };
  const onChange = (s: GraphState) => {
    state = s;
  };
  const { rerender } = render(
    <Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={onLock} onSkip={() => {}} />,
  );
  const rerenderWithState = () =>
    rerender(<Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={onLock} onSkip={() => {}} />);

  const lock = screen.getByRole("button", { name: /锁定|完成/ });

  // Fill claim (active by default, needSrc: false) with >=12 runes.
  fireEvent.change(screen.getByPlaceholderText("用你自己的话写……"), { target: { value: "核心判断一句话说清楚不能少于十二个字。" } });
  rerenderWithState();

  // Open evidence and fill its text (>=12 runes) — but DO NOT select a source.
  fireEvent.click(screen.getByText("支撑证据"));
  rerenderWithState();
  fireEvent.change(screen.getByPlaceholderText("用你自己的话写……"), { target: { value: "证据如何支撑主张写满十二个字以上。" } });
  rerenderWithState();

  // Every slot has enough text, but the needSrc evidence slot has no source:
  // the source requirement is the only thing gating the lock.
  expect(state.nodes.find((n) => n.id === "evidence")?.text.trim().length).toBeGreaterThanOrEqual(12);
  expect(state.edges.filter((e) => e.type === "cites")).toHaveLength(0);
  expect(lock).toBeDisabled();

  fireEvent.click(lock);
  expect(onLock).not.toHaveBeenCalled();

  // Now add the missing source — the lock must enable, proving the source
  // requirement (not the text) was the gate.
  fireEvent.click(screen.getByText("NASA Earth Observatory"));
  rerenderWithState();
  expect(lock).not.toBeDisabled();
});

test("toggling a source chip twice removes the cites edge again", () => {
  let state: GraphState = { nodes: [], edges: [] };
  const onChange = (s: GraphState) => {
    state = s;
  };
  const { rerender } = render(
    <Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={() => {}} onSkip={() => {}} />,
  );
  fireEvent.click(screen.getByText("支撑证据"));
  rerender(<Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={() => {}} onSkip={() => {}} />);

  fireEvent.click(screen.getByText("NASA Earth Observatory"));
  expect(state.edges).toHaveLength(1);
  rerender(<Graph slots={slots} state={state} lockedSources={lockedSources} onChange={onChange} onLock={() => {}} onSkip={() => {}} />);

  fireEvent.click(screen.getByText("NASA Earth Observatory"));
  expect(state.edges).toHaveLength(0);
});

test("explains why a needSrc slot cannot be completed when there are no locked sources to offer, instead of leaving a dead picker unexplained", () => {
  render(<Graph slots={slots} state={{ nodes: [], edges: [] }} lockedSources={[]} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />);
  fireEvent.click(screen.getByText("支撑证据"));
  expect(screen.getByText(/还没有锁定的素材/)).toBeInTheDocument();
});

test("shows no such explanation for a slot that has sources to offer", () => {
  render(<Graph slots={slots} state={{ nodes: [], edges: [] }} lockedSources={lockedSources} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />);
  fireEvent.click(screen.getByText("支撑证据"));
  expect(screen.queryByText(/还没有锁定的素材/)).not.toBeInTheDocument();
});

test("skip fires onSkip without requiring completion", () => {
  const onSkip = vi.fn();
  render(<Graph slots={slots} state={{ nodes: [], edges: [] }} lockedSources={lockedSources} onChange={() => {}} onLock={() => {}} onSkip={onSkip} />);
  fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
  expect(onSkip).toHaveBeenCalledTimes(1);
});

test("a slot with no needSrc never renders a source picker or dead-picker hint", () => {
  render(<Graph slots={slots} state={{ nodes: [], edges: [] }} lockedSources={[]} onChange={() => {}} onLock={() => {}} onSkip={() => {}} />);
  // claim (needSrc: false) is active by default; its slot has no source UI at all.
  expect(screen.queryByText(/还没有锁定的素材/)).not.toBeInTheDocument();
});
