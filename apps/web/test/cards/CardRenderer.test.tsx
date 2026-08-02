import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { loadRegistry } from "@mind-imprint/contracts";
import { CardRenderer } from "@/cards/CardRenderer";

const reg = loadRegistry();

describe("CardRenderer (schema-driven)", () => {
  it("renders the CRAAP card's always-step fields from JSON", () => {
    render(<CardRenderer card={reg.craap!} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByText("贴入来源")).toBeInTheDocument();
    expect(screen.getByText("C · Currency 时效性")).toBeInTheDocument();
    expect(screen.getByText("时效性判断")).toBeInTheDocument();
  });
  it("keeps the on_demand Bias step collapsed until expanded, then shows its fields", async () => {
    const onExpandStep = vi.fn();
    render(<CardRenderer card={reg.craap!} values={{}} onField={vi.fn()} onExpandStep={onExpandStep} />);
    expect(screen.queryByText("偏见程度判断")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Bias/ }));
    expect(onExpandStep).toHaveBeenCalledWith("bias");
    expect(screen.getByText("偏见程度判断")).toBeInTheDocument();
  });
  it("renders the concession card (no on_demand steps) from the same component", () => {
    render(<CardRenderer card={reg.concession!} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByLabelText("你的中心论点是什么？")).toBeInTheDocument();
    expect(screen.getByLabelText("先承认它（让步）")).toBeInTheDocument();
  });
});
