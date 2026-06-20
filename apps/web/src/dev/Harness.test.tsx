import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardInstance } from "@mind-imprint/contracts";
import { Harness } from "./Harness";

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
});
