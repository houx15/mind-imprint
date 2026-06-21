import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createStore, makeMemoryStorage } from "../store";
import { StorePanel } from "./StorePanel";

describe("StorePanel", () => {
  it("creates a task and appends a message via the UI", async () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<StorePanel store={store} />);

    await userEvent.type(screen.getByLabelText("任务标题"), "气候");
    await userEvent.click(screen.getByRole("button", { name: "新建任务" }));
    expect(screen.getByText("气候")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /选择/ }));
    await userEvent.type(screen.getByLabelText("消息内容"), "你好");
    await userEvent.click(screen.getByRole("button", { name: "追加消息" }));
    expect(screen.getByText("你好")).toBeInTheDocument();
  });
});
