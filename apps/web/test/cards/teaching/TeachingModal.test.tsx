import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeachingModal } from "@/cards/teaching/TeachingModal";
import type { TeachingModule } from "@/cards/teaching/types";

const mod: TeachingModule = { cardId: "x", title: "测试课", category: "测试", chapters: [
  { key: "a", label: "第一章", Component: () => <p>章节A内容</p> },
  { key: "b", label: "第二章", Component: () => <p>章节B内容</p> },
]};

it("shows the first chapter and advances to the next", async () => {
  const onClose = vi.fn();
  render(<TeachingModal module={mod} onClose={onClose} />);
  expect(screen.getByText("章节A内容")).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: "下一步" }));
  expect(screen.getByText("章节B内容")).toBeTruthy();
});

it("last chapter's 完成 closes the modal", async () => {
  const onClose = vi.fn();
  render(<TeachingModal module={mod} onClose={onClose} />);
  await userEvent.click(screen.getByRole("button", { name: "下一步" }));
  await userEvent.click(screen.getByRole("button", { name: "完成" }));
  expect(onClose).toHaveBeenCalled();
});

it("close button closes the modal", async () => {
  const onClose = vi.fn();
  render(<TeachingModal module={mod} onClose={onClose} />);
  await userEvent.click(screen.getByRole("button", { name: "关闭" }));
  expect(onClose).toHaveBeenCalled();
});
