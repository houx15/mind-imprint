import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { GrowthPlaceholder } from "./GrowthPlaceholder";

describe("GrowthPlaceholder", () => {
  it("says plainly that the growth report is being rebuilt", () => {
    render(<GrowthPlaceholder />);
    expect(screen.getByText("成长报告正在重建")).toBeTruthy();
  });
});
