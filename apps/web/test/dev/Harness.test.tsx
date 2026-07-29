import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardInstance } from "@mind-imprint/contracts";
import { Harness } from "@/dev/Harness";

describe("Harness end-to-end (fake data)", () => {
  it("proposed -> open -> submit yields a schema-valid completed envelope shown as JSON", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "打开卡" }));
    await userEvent.click(screen.getByRole("button", { name: "提交" }));

    const json = screen.getByTestId("envelope-json").textContent ?? "{}";
    const parsed = JSON.parse(json);
    expect(CardInstance.safeParse(parsed).success).toBe(true);
    expect(parsed.status).toBe("completed");
    expect(parsed.completed_at).not.toBeNull();
  });

  it("skip from the proposal yields a skipped envelope", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "暂不，先继续" }));
    const parsed = JSON.parse(screen.getByTestId("envelope-json").textContent ?? "{}");
    expect(parsed.status).toBe("skipped");
  });

  it("lists the full registry of cards as options", () => {
    render(<Harness />);
    const picker = screen.getByLabelText("选择工具卡");
    const options = within(picker).getAllByRole("option");
    // the picker is data-driven over CARD_REGISTRY (45 cards), not a hardcoded list
    expect(options.length).toBe(45);
    // the demo cards are present by their exact names
    expect(within(picker).getByRole("option", { name: "SIFT×CRAAP 信息核查" })).toBeInTheDocument();
    expect(within(picker).getByRole("option", { name: "让步段 · 以退为进" })).toBeInTheDocument();
  });
});
