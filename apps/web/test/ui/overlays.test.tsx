import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Modal, Menu, toast, ToastHost } from "@/ui/overlays";

describe("Modal", () => {
  it("renders nothing when open=false", () => {
    const { container } = render(
      <Modal open={false} onClose={() => {}} title="确认删除">
        正文内容
      </Modal>,
    );
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByText("确认删除")).not.toBeInTheDocument();
  });

  it("shows the title and children when open", () => {
    render(
      <Modal open onClose={() => {}} title="确认删除">
        <p>正文内容</p>
      </Modal>,
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("确认删除")).toBeInTheDocument();
    expect(screen.getByText("正文内容")).toBeInTheDocument();
  });

  it("calls onClose on Escape keydown", () => {
    const onClose = vi.fn();
    render(
      <Modal open onClose={onClose} title="确认删除">
        正文内容
      </Modal>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose on scrim click but NOT on panel click", () => {
    const onClose = vi.fn();
    render(
      <Modal open onClose={onClose} title="确认删除">
        <p>正文内容</p>
      </Modal>,
    );

    fireEvent.click(screen.getByText("正文内容"));
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("dialog"));
    expect(onClose).not.toHaveBeenCalled();

    // The scrim is the sibling before the dialog panel, marked aria-hidden.
    const scrim = document.querySelector('[aria-hidden="true"]') as HTMLElement;
    fireEvent.click(scrim);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders the danger footer button using the danger token", () => {
    render(
      <Modal
        open
        onClose={() => {}}
        title="确认删除"
        footer={
          <button type="button" className="bg-mk-danger text-white">
            删除
          </button>
        }
      >
        正文内容
      </Modal>,
    );
    const footerBtn = screen.getByRole("button", { name: "删除" });
    expect(footerBtn.className).toContain("bg-mk-danger");
  });
});

describe("Menu", () => {
  const items = [
    { key: "rename", label: "重命名", onSelect: vi.fn() },
    { key: "delete", label: "删除", onSelect: vi.fn(), tone: "danger" as const },
  ];

  it("does not show items until the trigger is clicked", () => {
    render(<Menu trigger={<button type="button">操作</button>} items={items} />);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("opens the popover on trigger click", async () => {
    render(<Menu trigger={<button type="button">操作</button>} items={items} />);
    await userEvent.click(screen.getByText("操作"));
    expect(screen.getByRole("menu")).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "重命名" })).toBeInTheDocument();
  });

  it("calls onSelect and closes when an item is clicked", async () => {
    const onSelect = vi.fn();
    const localItems = [{ key: "rename", label: "重命名", onSelect }];
    render(<Menu trigger={<button type="button">操作</button>} items={localItems} />);
    await userEvent.click(screen.getByText("操作"));
    await userEvent.click(screen.getByRole("menuitem", { name: "重命名" }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("applies the danger text token to danger-tone items", async () => {
    render(<Menu trigger={<button type="button">操作</button>} items={items} />);
    await userEvent.click(screen.getByText("操作"));
    const deleteItem = screen.getByRole("menuitem", { name: "删除" });
    expect(deleteItem.className).toContain("text-mk-danger");
    const renameItem = screen.getByRole("menuitem", { name: "重命名" });
    expect(renameItem.className).not.toContain("text-mk-danger");
  });
});

describe("toast + ToastHost", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the toast message then removes it after the timeout", () => {
    render(<ToastHost />);

    act(() => {
      toast("已保存", { duration: 1000 });
    });
    expect(screen.getByText("已保存")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(999);
    });
    expect(screen.getByText("已保存")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.queryByText("已保存")).not.toBeInTheDocument();
  });

  it("defaults to a 3000ms auto-dismiss when no duration is given", () => {
    render(<ToastHost />);

    act(() => {
      toast("默认时长");
    });
    expect(screen.getByText("默认时长")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.queryByText("默认时长")).not.toBeInTheDocument();
  });
});
