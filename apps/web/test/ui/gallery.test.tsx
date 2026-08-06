import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DesignSystemGallery } from "@/dev/DesignSystemGallery";

describe("DesignSystemGallery", () => {
  it("renders without throwing and shows the 设计系统 heading", () => {
    expect(() => render(<DesignSystemGallery />)).not.toThrow();
    expect(screen.getByRole("heading", { name: "设计系统" })).toBeInTheDocument();
  });

  it("renders at least one Pebble svg (locked body path)", () => {
    const { container } = render(<DesignSystemGallery />);
    const paths = container.querySelectorAll("svg path");
    const hasPebbleBody = Array.from(paths).some((p) => (p.getAttribute("d") ?? "").startsWith("M20 6"));
    expect(hasPebbleBody).toBe(true);
  });

  it("renders the 8 accent-picker swatches", () => {
    const { container } = render(<DesignSystemGallery />);
    const swatches = container.querySelectorAll('[data-testid="accent-swatch"]');
    expect(swatches.length).toBe(8);
  });
});
