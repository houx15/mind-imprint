import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { loadRegistry, type CardSpec } from "@mind-imprint/contracts";
import { ActiveSheet } from "./ActiveSheet";

const reg = loadRegistry();
const props = () => ({
  card: reg.sift_craap!, values: {},
  onField: vi.fn(), onExpandStep: vi.fn(), onNoteOpen: vi.fn(), onSubmit: vi.fn(), onClose: vi.fn(),
});

// Synthetic card carrying structured methodology (real cards migrate in P2).
const methodologyCard = {
  id: "t", category: "信息素养", name: "测试卡", purpose: "p", trigger_condition: "tc", rubric_tags: [],
  steps: [{
    key: "s1", title: "第一步", disclose: "always",
    methodology: { why: "因为重要", how: "这样做", when: "卡住时" },
    fields: [{ type: "textarea", key: "x", label: "L" }],
  }],
} as unknown as CardSpec;

describe("ActiveSheet", () => {
  it("shows the takeover header and the card name", () => {
    render(<ActiveSheet {...props()} />);
    expect(screen.getByText("现在轮到你想")).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });
  it("forwards the per-step 方法 panel's expand to onNoteOpen", async () => {
    const p = { ...props(), card: methodologyCard };
    render(<ActiveSheet {...p} />);
    expect(screen.queryByText("这样做")).not.toBeInTheDocument();
    await userEvent.click(screen.getByText("方法"));
    expect(p.onNoteOpen).toHaveBeenCalledWith("s1");
    expect(screen.getByText("这样做")).toBeInTheDocument();
  });
  it("提交 fires onSubmit", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    await userEvent.click(screen.getByRole("button", { name: "提交" }));
    expect(p.onSubmit).toHaveBeenCalledOnce();
  });
  it("close button fires onClose", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    await userEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(p.onClose).toHaveBeenCalledOnce();
  });
});
