import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import type { Anchor, TraceEvent } from "@mind-imprint/contracts";
import { StudioAnnotateCard, anchorMode } from "./StudioAnnotateCard";

const anchors = [
  { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "", dimension: "currency", author: "ai" as const, question: "数据是哪一年的？", answer: "" },
  { id: "a1", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "", dimension: "authority", author: "ai" as const, question: "谁站在这条主张背后？", answer: "" },
];
const spec = { id: "craap", name: "信源辨识卡 CRAAP / CRRAAB", category: "信息素养", purpose: "x", primitive: "annotate" } as any;

test("renders one answerable row per anchor and locks with filled anchors + risk_note", () => {
  const onSubmit = vi.fn();
  render(<StudioAnnotateCard spec={spec} anchors={anchors} onSubmit={onSubmit} onSkip={() => {}} />);
  const boxes = screen.getAllByRole("textbox");
  // one per anchor + one risk-note field
  expect(boxes).toHaveLength(anchors.length + 1);

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[0]!, { target: { value: "数据是 2019 年的，偏旧" } });
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[1]!, { target: { value: "只是博主，无机构背景" } });
  // all anchors answered but risk_note still empty — still disabled.
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[anchors.length]!, { target: { value: "支撑核心数据，但单一来源有风险" } });
  expect(lockButton).not.toBeDisabled();

  fireEvent.click(lockButton);
  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];
  expect(env.anchors.find((a: any) => a.dimension === "currency").answer).toBe("数据是 2019 年的，偏旧");
  const risk = env.anchors.find((a: any) => a.dimension === "risk_note");
  expect(risk).toBeTruthy();
  expect(risk.author).toBe("student");
  expect(risk.answer).toBe("支撑核心数据，但单一来源有风险");
});

// REGRESSION FENCE (N3c task 8, test 2 of the brief): L1 anchors — author
// "ai" — must render byte-identical to today: no locate button, no "找不到"
// escape, no question input. Extends the test above rather than replacing it,
// so the very first thing any first-time student sees stays pinned.
test("L1 anchors render with no locate control and no question input (regression fence)", () => {
  render(<StudioAnnotateCard spec={spec} anchors={anchors} onSubmit={vi.fn()} onSkip={() => {}} />);
  expect(screen.queryByRole("button", { name: "去文章里选出这句" })).not.toBeInTheDocument();
  expect(screen.queryByText("找不到合适的句子")).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText(/写下你想问的问题/)).not.toBeInTheDocument();
});

// The mode is derived from the anchor's own shape (author/question), never
// from which props the host happens to pass. An "ai"-authored anchor stays
// answer-only even when the host offers onRequestLocate/onSpanNotFound.
test("L1 anchors stay answer-only even when the host offers onRequestLocate/onSpanNotFound", () => {
  render(
    <StudioAnnotateCard
      spec={spec}
      anchors={anchors}
      onSubmit={vi.fn()}
      onSkip={() => {}}
      onRequestLocate={vi.fn()}
      onSpanNotFound={vi.fn()}
    />,
  );
  expect(screen.queryByRole("button", { name: "去文章里选出这句" })).not.toBeInTheDocument();
  expect(screen.queryByText("找不到合适的句子")).not.toBeInTheDocument();
});

// FIX 2 (whole-branch review CRITICAL): `anchors.every(...)` is vacuously
// true on an empty array — studioturn.go's surfaceAnchors degrades to []
// anchors on any generator failure (parse error / provider hiccup / empty
// generation), so a zero-anchor CRAAP card was lockable the instant the
// student typed only the risk note. The server can never actually complete
// such a submit (craap.json's every_tag_present needs 5 named tags that
// don't exist here), so locking it only ever produced the exact FIX 1
// scenario this whole fix wave exists to close: a stuck-active card with
// nothing to show for it. RED without the `hasAnchors` guard (revert it and
// this fails: the button enables immediately after the risk note alone).
test("refuses to lock a zero-anchor card even once the risk note is filled", () => {
  const onSubmit = vi.fn();
  render(<StudioAnnotateCard spec={spec} anchors={[]} onSubmit={onSubmit} onSkip={() => {}} />);
  // Zero per-anchor rows + the one risk-note field.
  const boxes = screen.getAllByRole("textbox");
  expect(boxes).toHaveLength(1);

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[0]!, { target: { value: "支撑核心数据，但单一来源有风险" } });
  expect(lockButton).toBeDisabled();

  fireEvent.click(lockButton);
  expect(onSubmit).not.toHaveBeenCalled();
});

// A disabled control with no explanation reads as a broken product. The
// zero-anchor card's lock button is legitimately dead (it can never satisfy
// the completion predicate), so the card must SAY so and point at the exit
// she does have — skipping — rather than leave her clicking a button that
// will never respond.
test("explains why a zero-anchor card cannot be locked, instead of just deadening the button", () => {
  render(<StudioAnnotateCard spec={spec} anchors={[]} onSubmit={vi.fn()} onSkip={() => {}} />);
  expect(screen.getByText(/没能取到要核对的句子/)).toBeInTheDocument();
});

test("shows no such explanation when the card has anchors to ask about", () => {
  render(<StudioAnnotateCard spec={spec} anchors={anchors} onSubmit={vi.fn()} onSkip={() => {}} />);
  expect(screen.queryByText(/没能取到要核对的句子/)).not.toBeInTheDocument();
});

// --- N3c task 8: guidance-level modes (spec §2, §5, §6) --------------------

const makeAnchor = (overrides: Partial<Anchor>): Anchor => ({
  id: "x",
  material_id: "m1",
  block_id: "",
  start: 0,
  end: 0,
  quote: "",
  dimension: "currency",
  author: "ai",
  question: "q",
  answer: "",
  ...overrides,
});

// Test 1 (brief): anchorMode table. The level is never a stored field — it is
// derived from author + question, exactly as spec §2 states.
test("anchorMode derives answer/locate/elicit from author + question — never a stored level field", () => {
  expect(anchorMode(makeAnchor({ author: "ai", question: "AI 写好的问题" }))).toBe("answer");
  expect(anchorMode(makeAnchor({ author: "student", question: "这条信息是什么时候发布的？" }))).toBe("locate");
  expect(anchorMode(makeAnchor({ author: "student", question: "" }))).toBe("elicit");
  // blank/whitespace-only question is still "elicit" — she hasn't written one yet.
  expect(anchorMode(makeAnchor({ author: "student", question: "   " }))).toBe("elicit");
});

// Test 3 (brief): L2 — AI wrote the question, student locates + answers.
test("L2 anchors render the AI's question read-only, plus a locate button and the not-found escape", () => {
  const l2 = [makeAnchor({ id: "b0", author: "student", question: "这条信息是什么时候发布的？" })];
  render(
    <StudioAnnotateCard spec={spec} anchors={l2} onSubmit={vi.fn()} onSkip={vi.fn()} onRequestLocate={vi.fn()} onSpanNotFound={vi.fn()} />,
  );
  expect(screen.getByText("这条信息是什么时候发布的？")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "去文章里选出这句" })).toBeInTheDocument();
  expect(screen.getByText("找不到合适的句子")).toBeInTheDocument();
  expect(screen.queryByPlaceholderText(/写下你想问的问题/)).not.toBeInTheDocument();
});

// Test 4 (brief): L3 — student elicits the question herself too.
test("L3 anchors render an empty question input plus the locate button and escape, no AI question text", () => {
  const l3 = [makeAnchor({ id: "c0", author: "student", question: "" })];
  render(
    <StudioAnnotateCard spec={spec} anchors={l3} onSubmit={vi.fn()} onSkip={vi.fn()} onRequestLocate={vi.fn()} onSpanNotFound={vi.fn()} />,
  );
  const questionInput = screen.getByPlaceholderText(/写下你想问的问题/);
  expect(questionInput).toHaveValue("");
  expect(screen.getByRole("button", { name: "去文章里选出这句" })).toBeInTheDocument();
  expect(screen.getByText("找不到合适的句子")).toBeInTheDocument();
});

// Test 5 (brief): lock gating at L2 — the escape must ALWAYS be able to
// unblock the lock (铁律 2, an offer is never a wall).
test("L2 lock gate: taking the not-found escape always unblocks the lock", () => {
  const onSpanNotFound = vi.fn();
  const l2 = [makeAnchor({ id: "d0", author: "student", question: "这条信息是什么时候发布的？" })];
  render(
    <StudioAnnotateCard spec={spec} anchors={l2} onSubmit={vi.fn()} onSkip={vi.fn()} onRequestLocate={vi.fn()} onSpanNotFound={onSpanNotFound} />,
  );
  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "答案" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险说明" } });

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.click(screen.getByRole("button", { name: "找不到合适的句子" }));
  expect(onSpanNotFound).toHaveBeenCalledWith("d0", "currency");
  expect(lockButton).not.toBeDisabled();
});

// Test 6 (brief): lock gating at L3 additionally requires a written question
// — there is no "找不到" escape for eliciting, because writing your own
// question cannot fail the way searching can.
test("L3 lock gate additionally requires a written question, with no escape for it", () => {
  const l3 = [makeAnchor({ id: "e0", author: "student", question: "" })];
  render(
    <StudioAnnotateCard spec={spec} anchors={l3} onSubmit={vi.fn()} onSkip={vi.fn()} onRequestLocate={vi.fn()} onSpanNotFound={vi.fn()} />,
  );
  const questionInput = screen.getByPlaceholderText(/写下你想问的问题/);
  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "答案" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险说明" } });
  // take the locate escape — this alone must NOT be enough at L3.
  fireEvent.click(screen.getByRole("button", { name: "找不到合适的句子" }));

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.change(questionInput, { target: { value: "这条数据来自哪个机构？" } });
  expect(lockButton).not.toBeDisabled();
});

// Test 7 (brief): the submitted envelope carries the located span merged
// onto the anchor, the student's typed L3 question, and span_not_found trace
// events for the dimensions she gave up searching (铁律 4 — that's DATA).
test("submitted envelope merges the located span, keeps the typed question, and records span_not_found for skipped dimensions", () => {
  const onSubmit = vi.fn();
  const mixed = [
    makeAnchor({ id: "f0", dimension: "currency", author: "student", question: "这条信息是什么时候发布的？" }), // located via prop
    makeAnchor({ id: "f1", dimension: "authority", author: "student", question: "这条信息是谁写的？" }), // takes the escape
    makeAnchor({ id: "f2", dimension: "tone", author: "student", question: "" }), // L3, elicits + takes the escape
  ];
  const locatedSpans = { f0: { block_id: "b3", start: 12, end: 30, quote: "过去二十年里发生的事" } };
  render(
    <StudioAnnotateCard
      spec={spec}
      anchors={mixed}
      onSubmit={onSubmit}
      onSkip={vi.fn()}
      onRequestLocate={vi.fn()}
      onSpanNotFound={vi.fn()}
      locatedSpans={locatedSpans}
    />,
  );

  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "2019年的" } });
  fireEvent.change(screen.getByRole("textbox", { name: "authority-answer" }), { target: { value: "记者写的" } });
  fireEvent.change(screen.getByRole("textbox", { name: "tone-answer" }), { target: { value: "偏乐观" } });
  fireEvent.change(screen.getByRole("textbox", { name: "tone-question" }), { target: { value: "这段的语气是什么？" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险说明" } });

  // f0 is already located via the prop, so it shows no escape control —
  // only f1 and f2 do.
  const escapeButtons = screen.getAllByRole("button", { name: "找不到合适的句子" });
  expect(escapeButtons).toHaveLength(2);
  fireEvent.click(escapeButtons[0]!);
  fireEvent.click(escapeButtons[1]!);

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).not.toBeDisabled();
  fireEvent.click(lockButton);

  const env = onSubmit.mock.calls[0][0];
  const f0 = env.anchors.find((a: any) => a.id === "f0");
  expect(f0).toMatchObject({ block_id: "b3", start: 12, end: 30, quote: "过去二十年里发生的事" });
  const f2 = env.anchors.find((a: any) => a.id === "f2");
  expect(f2.question).toBe("这段的语气是什么？");

  expect(env.event_trace).toContainEqual(expect.objectContaining({ kind: "span_not_found", dimension: "authority" }));
  expect(env.event_trace).toContainEqual(expect.objectContaining({ kind: "span_not_found", dimension: "tone" }));
  expect(env.event_trace.some((e: any) => e.kind === "span_not_found" && e.dimension === "currency")).toBe(false);
});

// Missing test named by the Task 8 review: `.trim()` already guards
// `allQuestionsWritten`, but nothing pinned it — a whitespace-only "question"
// must not count as written any more than an empty one does.
test("a whitespace-only typed question at the elicit level keeps the lock disabled", () => {
  const l3 = [makeAnchor({ id: "h0", author: "student", question: "" })];
  render(
    <StudioAnnotateCard spec={spec} anchors={l3} onSubmit={vi.fn()} onSkip={vi.fn()} onRequestLocate={vi.fn()} onSpanNotFound={vi.fn()} />,
  );
  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "答案" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险说明" } });
  fireEvent.change(screen.getByPlaceholderText(/写下你想问的问题/), { target: { value: "   " } });

  expect(screen.getByRole("button", { name: /锁定|评估完成|完成/ })).toBeDisabled();
});

// Minor (Task 7/8 review): taking the "找不到合适的句子" escape used to be
// permanent — an accidental click left no way back to the locate controls
// for that dimension. The escape must ALWAYS be able to unblock the lock
// (铁律 2), but an unrecoverable accidental click is itself a small wall.
test("the not-found escape can be undone, restoring the locate/escape controls", () => {
  const l2 = [makeAnchor({ id: "i0", author: "student", question: "这条信息是什么时候发布的？" })];
  render(
    <StudioAnnotateCard spec={spec} anchors={l2} onSubmit={vi.fn()} onSkip={vi.fn()} onRequestLocate={vi.fn()} onSpanNotFound={vi.fn()} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "找不到合适的句子" }));
  expect(screen.getByText(/已记录：这条没能在文章里找到合适的句子/)).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "重新找一下" }));
  expect(screen.queryByText(/已记录：这条没能在文章里找到合适的句子/)).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "去文章里选出这句" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "找不到合适的句子" })).toBeInTheDocument();
});

// Whole-branch review IMPORTANT 1: a located span used to be permanent — the
// `located` branch was a bare div with no control at all, so an accidental
// two-character drag in the article (Annotate commits ANY non-collapsed
// selection) permanently locked her into a fragment, with no way back. Now
// `onRelocate` mirrors the escaped branch's undo: it lets her clear the
// stale span and re-pick, and the lock stays reachable throughout (via the
// re-pick path here, exactly as the escape already covers the other path).
test("an accidental located span can be re-picked and replaced, and the lock stays reachable throughout", () => {
  const onRelocate = vi.fn();
  const l2 = [makeAnchor({ id: "j0", dimension: "currency", author: "student", question: "这条信息是什么时候发布的？" })];
  const locatedSpans = { j0: { block_id: "b0", start: 0, end: 2, quote: "过去" } };

  const { rerender } = render(
    <StudioAnnotateCard
      spec={spec}
      anchors={l2}
      onSubmit={vi.fn()}
      onSkip={vi.fn()}
      onRequestLocate={vi.fn()}
      onRelocate={onRelocate}
      onSpanNotFound={vi.fn()}
      locatedSpans={locatedSpans}
    />,
  );

  // The accidental fragment is shown, with a re-pick control and no bare
  // dead end — no 「去文章里选出这句」/「找不到合适的句子」 while located.
  expect(screen.getByText("已在文章里定位：「过去」")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "去文章里选出这句" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "找不到合适的句子" })).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "重新选一句" }));
  expect(onRelocate).toHaveBeenCalledWith("j0", "currency");

  // onRelocate is the HOST's job (it owns locatedSpans) — simulate the
  // container clearing the stale entry and re-issuing the locate request,
  // then a fresh selection landing as a real located span.
  rerender(
    <StudioAnnotateCard
      spec={spec}
      anchors={l2}
      onSubmit={vi.fn()}
      onSkip={vi.fn()}
      onRequestLocate={vi.fn()}
      onRelocate={onRelocate}
      onSpanNotFound={vi.fn()}
      locatedSpans={{}}
    />,
  );
  // With the stale span cleared, the lock is reachable again via the
  // ordinary locate/escape controls — she is never stranded.
  expect(screen.getByRole("button", { name: "去文章里选出这句" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "找不到合适的句子" })).toBeInTheDocument();

  rerender(
    <StudioAnnotateCard
      spec={spec}
      anchors={l2}
      onSubmit={vi.fn()}
      onSkip={vi.fn()}
      onRequestLocate={vi.fn()}
      onRelocate={onRelocate}
      onSpanNotFound={vi.fn()}
      locatedSpans={{ j0: { block_id: "b0", start: 12, end: 30, quote: "一整段真正相关的句子" } }}
    />,
  );
  expect(screen.getByText("已在文章里定位：「一整段真正相关的句子」")).toBeInTheDocument();
  expect(screen.queryByText("已在文章里定位：「过去」")).not.toBeInTheDocument();
});

// N3c task 9: `pendingTrace` (the container's own accumulated span_located/
// span_not_found trace for this instance) becomes the submitted event_trace
// verbatim when supplied — the container is the only thing that can
// honestly timestamp a span_located event (it happens on the article pane,
// not at lock time), so this renderer must not try to reconstruct it.
test("uses the container's pendingTrace as the submitted event_trace when supplied, instead of reconstructing locally", () => {
  const onSubmit = vi.fn();
  const anchor = makeAnchor({ id: "j0", dimension: "currency", author: "student", question: "这条信息是什么时候发布的？" });
  const pendingTrace: TraceEvent[] = [
    { kind: "span_located", dimension: "currency", block_id: "b3", at: "2026-07-22T00:00:00Z" },
  ];
  render(
    <StudioAnnotateCard
      spec={spec}
      anchors={[anchor]}
      onSubmit={onSubmit}
      onSkip={vi.fn()}
      onRequestLocate={vi.fn()}
      onSpanNotFound={vi.fn()}
      locatedSpans={{ j0: { block_id: "b3", start: 0, end: 5, quote: "过去二十年" } }}
      pendingTrace={pendingTrace}
    />,
  );
  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "2019" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险" } });
  fireEvent.click(screen.getByRole("button", { name: /锁定|评估完成|完成/ }));

  const env = onSubmit.mock.calls[0][0];
  expect(env.event_trace).toEqual(pendingTrace);
});

// DEAD-CONTROL RULE: an older host that cannot switch panes has no
// onRequestLocate to pass. A control that cannot work must not be shown, and
// a gate with no way to satisfy it is a wall — so no locate/escape UI renders
// and the lock is not gated on locating at all.
test("without onRequestLocate, L2/L3 anchors render no locate control and are not gated on locating", () => {
  const onSubmit = vi.fn();
  const l2 = [makeAnchor({ id: "g0", author: "student", question: "这条信息是什么时候发布的？" })];
  render(<StudioAnnotateCard spec={spec} anchors={l2} onSubmit={onSubmit} onSkip={vi.fn()} />);

  expect(screen.queryByRole("button", { name: "去文章里选出这句" })).not.toBeInTheDocument();
  expect(screen.queryByText("找不到合适的句子")).not.toBeInTheDocument();

  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "2019年的" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险说明" } });

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).not.toBeDisabled();
  fireEvent.click(lockButton);
  expect(onSubmit).toHaveBeenCalledTimes(1);
});

// Per-anchor DEAD-CONTROL RULE (task 9 correctness fix, item 2): unlike the
// host-level rule above, here the HOST does offer onRequestLocate, but THIS
// anchor's own material_id is empty — the container's onRequestLocate
// correctly no-ops for it (nowhere to switch to, nothing to open), so
// 「去文章里选出这句」 must not even render for this one anchor. The escape
// stays fully live regardless (it never depended on material_id), so the
// lock must still be reachable through it.
test("an anchor with no material_id offers no locate button, but the escape keeps the lock reachable", () => {
  const onSubmit = vi.fn();
  const noMaterial = [makeAnchor({ id: "k0", material_id: "", author: "student", question: "这条信息是什么时候发布的？" })];
  render(
    <StudioAnnotateCard
      spec={spec}
      anchors={noMaterial}
      onSubmit={onSubmit}
      onSkip={vi.fn()}
      onRequestLocate={vi.fn()}
      onSpanNotFound={vi.fn()}
    />,
  );

  expect(screen.queryByRole("button", { name: "去文章里选出这句" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "找不到合适的句子" })).toBeInTheDocument();

  fireEvent.change(screen.getByRole("textbox", { name: "currency-answer" }), { target: { value: "答案" } });
  fireEvent.change(screen.getAllByRole("textbox").at(-1)!, { target: { value: "风险说明" } });
  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.click(screen.getByRole("button", { name: "找不到合适的句子" }));
  expect(lockButton).not.toBeDisabled();
  fireEvent.click(lockButton);
  expect(onSubmit).toHaveBeenCalledTimes(1);
});
