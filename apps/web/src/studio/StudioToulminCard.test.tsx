import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { StudioToulminCard } from "./StudioToulminCard";

// Minimal two-slot spec (mirrors toulmin.json's params.slots shape): one
// plain slot + one needSrc slot, so both the text gate and the source gate
// are exercised without dragging in all five real roles.
const spec = {
  id: "toulmin",
  name: "论证构建卡（图尔敏）",
  category: "论证结构",
  primitive: "graph",
  params: {
    slots: [
      { id: "claim", role: "核心主张", needSrc: false, q: "你要论证的核心判断，用一句话说清。" },
      { id: "evidence", role: "支撑证据", needSrc: true, q: "挑一条证据，用自己的话概括它如何支撑主张。" },
    ],
  },
} as any;
const lockedSources = [{ id: "m_nasa", name: "NASA" }];

test("submits serialized anchors on lock (text anchor + source anchor, both student-authored)", () => {
  const onSubmit = vi.fn();
  render(
    <StudioToulminCard
      spec={spec}
      cardInstanceId="ci1"
      lockedSources={lockedSources}
      onSubmit={onSubmit}
      onSkip={() => {}}
    />,
  );

  const lockButton = screen.getByRole("button", { name: /全部锁定|完成论证|锁定/ });
  expect(lockButton).toBeDisabled();

  // The first slot (claim) is expanded by default — fill its sentence with
  // >=12 runes to clear the backend's slotComplete gate.
  fireEvent.change(screen.getByPlaceholderText("用你自己的话写……"), {
    target: { value: "这是一个足够长的核心主张句子表述" },
  });
  // Still short of lockable: the evidence slot needs text AND a source.
  expect(lockButton).toBeDisabled();

  // Expand the evidence slot, write its sentence, attach the locked source.
  fireEvent.click(screen.getByText("支撑证据"));
  fireEvent.change(screen.getByPlaceholderText("用你自己的话写……"), {
    target: { value: "这条证据足够长可以支撑上面的主张" },
  });
  fireEvent.click(screen.getByRole("button", { name: "NASA" }));

  expect(lockButton).not.toBeDisabled();
  fireEvent.click(lockButton);

  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];

  // One text anchor per slot: the claim sentence, student-authored.
  const claimText = env.anchors.find((a: any) => a.dimension === "claim" && a.answer);
  expect(claimText).toBeTruthy();
  expect(claimText.author).toBe("student");
  expect(claimText.answer).toBe("这是一个足够长的核心主张句子表述");

  // One source anchor per cites edge: the evidence slot → the real locked
  // material id (not "", not a zero-uuid).
  const evidenceSrc = env.anchors.find((a: any) => a.dimension === "evidence" && a.material_id);
  expect(evidenceSrc).toBeTruthy();
  expect(evidenceSrc.material_id).toBe("m_nasa");
  expect(evidenceSrc.author).toBe("student");
});

test("skip forwards the scaffold event trace", () => {
  const onSkip = vi.fn();
  render(
    <StudioToulminCard
      spec={spec}
      cardInstanceId="ci1"
      lockedSources={lockedSources}
      onSubmit={() => {}}
      onSkip={onSkip}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
  expect(onSkip).toHaveBeenCalledTimes(1);
  expect(Array.isArray(onSkip.mock.calls[0][0])).toBe(true);
});

test("rehydrates in-progress slots from persisted anchors on mount (reload FIX-E)", () => {
  const onSubmit = vi.fn();
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "", dimension: "claim", author: "student" as const, question: "", answer: "已经写好的核心主张的一句话表述" },
  ];
  render(
    <StudioToulminCard
      spec={spec}
      cardInstanceId="ci1"
      anchors={anchors}
      lockedSources={lockedSources}
      onSubmit={onSubmit}
      onSkip={() => {}}
    />,
  );
  // The persisted claim sentence is restored into its (default-expanded) slot.
  expect((screen.getByPlaceholderText("用你自己的话写……") as HTMLTextAreaElement).value).toBe(
    "已经写好的核心主张的一句话表述",
  );
});
