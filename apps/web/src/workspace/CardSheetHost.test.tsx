import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardSheetHost } from "./CardSheetHost";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

function setup() {
  const onClose = vi.fn();
  const onSkip = vi.fn();
  const onSubmit = vi.fn();
  const spec = CARD_REGISTRY["sift_craap"]!;
  const ci = newEnvelope("sift_craap", "t1", () => "2026-01-01T00:00:00.000Z", () => "ci-1");
  render(
    <CardSheetHost
      cardInstance={ci}
      spec={spec}
      onSubmit={onSubmit}
      onClose={onClose}
      onSkip={onSkip}
    />
  );
  return { onClose, onSkip, onSubmit };
}

it("X (关闭) closes without skipping", async () => {
  const { onClose, onSkip } = setup();
  await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  expect(onClose).toHaveBeenCalledWith("ci-1");
  expect(onSkip).not.toHaveBeenCalled();
});

it("取消 closes without skipping", async () => {
  const { onClose, onSkip } = setup();
  await userEvent.click(screen.getByRole("button", { name: "取消" }));
  expect(onClose).toHaveBeenCalledWith("ci-1");
  expect(onSkip).not.toHaveBeenCalled();
});

it("跳过这张卡 records a deliberate skip", async () => {
  const { onClose, onSkip } = setup();
  await userEvent.click(screen.getByRole("button", { name: "跳过这张卡" }));
  expect(onSkip).toHaveBeenCalledWith("ci-1");
  expect(onClose).not.toHaveBeenCalled();
});
