import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { loadRegistry } from "@mind-imprint/contracts";
import { CardRenderer } from "@/cards/CardRenderer";

const reg = loadRegistry();

describe("CardRenderer (schema-driven)", () => {
  it("renders the SIFT card's always-step fields from JSON", () => {
    render(<CardRenderer card={reg.sift_craap!} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByLabelText(/Stop/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "+ 添加来源" })).toBeInTheDocument();
    expect(screen.getByLabelText(/Find better coverage/)).toBeInTheDocument();
  });
  it("keeps the on_demand CRAAP step collapsed until expanded, then shows 5 ratings", async () => {
    const onExpandStep = vi.fn();
    render(<CardRenderer card={reg.sift_craap!} values={{}} onField={vi.fn()} onExpandStep={onExpandStep} />);
    expect(screen.queryByText(/Currency/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /CRAAP/ }));
    expect(onExpandStep).toHaveBeenCalledWith("craap");
    expect(screen.getByText(/Currency/)).toBeInTheDocument();
    expect(screen.getAllByRole("radiogroup")).toHaveLength(5);
  });
  it("renders the concession card (no on_demand steps) from the same component", () => {
    render(<CardRenderer card={reg.concession!} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByLabelText("你的中心论点是什么？")).toBeInTheDocument();
    expect(screen.getByLabelText("先承认它（让步）")).toBeInTheDocument();
  });
});
