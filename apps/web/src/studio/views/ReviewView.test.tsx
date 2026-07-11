import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ReviewView } from "./ReviewView";
import { STUDIO_FIXTURE } from "../fixtures";

describe("ReviewView (评估/S6 shell)", () => {
  it("renders the 8 gauge tables with names and notes", () => {
    render(<ReviewView gauges={STUDIO_FIXTURE.views.review} />);
    for (const g of STUDIO_FIXTURE.views.review) {
      expect(screen.getByText(g.table)).toBeInTheDocument();
      expect(screen.getByText(g.note)).toBeInTheDocument();
    }
  });

  it("renders the lit-lamp count matching a fixture gauge", () => {
    render(<ReviewView gauges={STUDIO_FIXTURE.views.review} />);
    const g = STUDIO_FIXTURE.views.review.find((r) => r.table === "表D")!;
    const card = screen.getByTestId(`gauge-${g.table}`);
    const lamps = card.querySelectorAll("[data-lamp]");
    expect(lamps.length).toBe(g.total);
    const litLamps = card.querySelectorAll('[data-lamp="lit"]');
    expect(litLamps.length).toBe(g.lit);
  });
});
