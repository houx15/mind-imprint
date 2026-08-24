import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { WelcomeModal } from "@/tour/WelcomeModal";

describe("WelcomeModal", () => {
  it("greets by name and routes the three choices", () => {
    const onPick = vi.fn(); const onDismiss = vi.fn();
    render(<WelcomeModal open displayName="Phoebe" onPick={onPick} onDismiss={onDismiss} />);
    expect(screen.getByText(/Phoebe/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "课程" }));
    expect(onPick).toHaveBeenCalledWith("courses");
    fireEvent.click(screen.getByRole("button", { name: "项目" }));
    expect(onPick).toHaveBeenCalledWith("projects");
    fireEvent.click(screen.getByRole("button", { name: "稍后再说" }));
    expect(onDismiss).toHaveBeenCalled();
  });
});
