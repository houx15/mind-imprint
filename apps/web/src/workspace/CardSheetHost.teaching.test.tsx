import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardSheetHost } from "./CardSheetHost";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

function renderCard(cardId: string, onSubmit = vi.fn()) {
  const ci = newEnvelope(cardId, "t1", () => "2026-01-01T00:00:00.000Z", () => "ci-1");
  render(<CardSheetHost cardInstance={ci} spec={CARD_REGISTRY[cardId]!}
    onSubmit={onSubmit} onClose={vi.fn()} onSkip={vi.fn()} />);
}

it("shows 给我讲讲这个 and opens the teaching modal for sift_craap", async () => {
  renderCard("sift_craap");
  // Before clicking: the modal is not mounted, so its 下一步 nav button is absent.
  // (Assert on content UNIQUE to TeachingModal — CardSheetHost's own X button is
  //  already aria-label 关闭, so a 关闭 assertion would be vacuous and pass even
  //  if the modal never mounted.)
  expect(screen.queryByRole("button", { name: "下一步" })).toBeNull();
  const btn = screen.getByRole("button", { name: "给我讲讲这个" });
  await userEvent.click(btn);
  // After clicking: sift_craap has 5 chapters, so chapter 1 shows 下一步 (not 完成).
  expect(screen.getByRole("button", { name: "下一步" })).toBeTruthy();   // 弹窗打开
});

it("re-opening the teaching modal records note_open only once (dedupe)", async () => {
  const onSubmit = vi.fn();
  renderCard("sift_craap", onSubmit);

  // Helper: close the open TeachingModal. While it is open there are TWO 关闭
  // buttons (CardSheetHost's X + the modal's). The modal's is rendered later in
  // the DOM, so it is the last match.
  const closeModal = async () => {
    const closers = screen.getAllByRole("button", { name: "关闭" });
    await userEvent.click(closers[closers.length - 1]!);
  };

  // Open → close → open again. Without the ref guard, each open emits note_open.
  await userEvent.click(screen.getByRole("button", { name: "给我讲讲这个" }));
  expect(screen.getByRole("button", { name: "下一步" })).toBeTruthy(); // modal open
  await closeModal();
  expect(screen.queryByRole("button", { name: "下一步" })).toBeNull();  // modal closed
  await userEvent.click(screen.getByRole("button", { name: "给我讲讲这个" }));
  await closeModal();

  // Submit and inspect the real recorded trace on the captured CardInstance.
  await userEvent.click(screen.getByRole("button", { name: /提交并钉到过程树/ }));
  expect(onSubmit).toHaveBeenCalledTimes(1);
  const submitted = onSubmit.mock.calls[0]![1] as { event_trace: { kind: string }[] };
  const noteOpens = submitted.event_trace.filter((e) => e.kind === "note_open");
  expect(noteOpens).toHaveLength(1);
});

it("a card without a teaching module shows no 给我讲讲这个 and keeps 方法 panels", () => {
  renderCard("concession");                                            // 无 teaching 的卡
  expect(screen.queryByRole("button", { name: "给我讲讲这个" })).toBeNull();
  expect(screen.getAllByRole("button", { name: "方法" }).length).toBeGreaterThan(0);
});

it("teaching cards hide their per-step 方法 panels", () => {
  renderCard("sift_craap");
  expect(screen.queryByRole("button", { name: "方法" })).toBeNull();
});
