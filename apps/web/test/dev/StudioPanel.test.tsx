import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StudioPanel } from "@/dev/StudioPanel";

describe("StudioPanel (harness)", () => {
  it("clicking a station in the rail switches the center view", () => {
    render(<StudioPanel />);
    // "论证构建" legitimately renders twice: station rail row + center header.
    expect(screen.getAllByText("论证构建").length).toBe(2);         // S4 default
    fireEvent.click(screen.getByText("信源评估"));                 // S3
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();      // 素材 view now shown
  });
});
