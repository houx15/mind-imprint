import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MindImprintIndicator } from "./MindImprintIndicator";

describe("MindImprintIndicator", () => {
  it("renders nothing when not visible", () => {
    const { container } = render(<MindImprintIndicator visible={false} onOpen={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it("shows the calm chip and fires onOpen when clicked", () => {
    let opened = false;
    render(<MindImprintIndicator visible onOpen={() => { opened = true; }} />);
    const chip = screen.getByText(/你的思维印记有新内容/);
    expect(chip).toBeTruthy();
    fireEvent.click(chip);
    expect(opened).toBe(true);
  });
});
