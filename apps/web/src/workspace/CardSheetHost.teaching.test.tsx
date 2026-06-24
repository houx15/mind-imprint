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

it("a card without a teaching module shows no 给我讲讲这个 and keeps 方法 panels", () => {
  renderCard("concession");                                            // 无 teaching 的卡
  expect(screen.queryByRole("button", { name: "给我讲讲这个" })).toBeNull();
  expect(screen.getAllByRole("button", { name: "方法" }).length).toBeGreaterThan(0);
});

it("teaching cards hide their per-step 方法 panels", () => {
  renderCard("sift_craap");
  expect(screen.queryByRole("button", { name: "方法" })).toBeNull();
});
