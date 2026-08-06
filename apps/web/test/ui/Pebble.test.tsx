import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Pebble } from "@/ui/Pebble";

describe("Pebble", () => {
  it("renders an svg whose body path starts with the locked geometry", () => {
    const { container } = render(<Pebble />);
    expect(container.querySelector("svg")).toBeInTheDocument();
    const path = container.querySelector("path");
    expect(path).toBeInTheDocument();
    expect(path!.getAttribute("d")).toMatch(/^M20 6/);
  });

  it("body path fill follows the student accent, never a hardcoded color", () => {
    const { container } = render(<Pebble />);
    const path = container.querySelector("path");
    expect(path!.getAttribute("fill")).toBe("var(--mk-accent-500)");
  });

  it("defaults to state='idle' with no state-specific class", () => {
    const { container } = render(<Pebble />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("class")).toBe("mk-pebble");
  });

  it("state='done' applies the pop-animation class", () => {
    const { container } = render(<Pebble state="done" />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("class")).toContain("mk-pebble-done");
  });

  it("state='processing' renders the spinning ring element", () => {
    const { container } = render(<Pebble state="processing" />);
    expect(container.querySelector(".mk-pebble-ring2")).toBeInTheDocument();
  });

  it("state='thinking' renders the pulse ring and blinking eyes", () => {
    const { container } = render(<Pebble state="thinking" />);
    expect(container.querySelector(".mk-pebble-ring")).toBeInTheDocument();
    expect(container.querySelectorAll(".mk-pebble-eye").length).toBe(2);
  });

  it("state='generating' renders the darting eyes group", () => {
    const { container } = render(<Pebble state="generating" />);
    expect(container.querySelector(".mk-pebble-eyes")).toBeInTheDocument();
  });

  it("state='idle' renders no ring, no eyes group, no smile", () => {
    const { container } = render(<Pebble state="idle" />);
    expect(container.querySelector(".mk-pebble-ring")).not.toBeInTheDocument();
    expect(container.querySelector(".mk-pebble-ring2")).not.toBeInTheDocument();
    expect(container.querySelector(".mk-pebble-eyes")).not.toBeInTheDocument();
  });

  it("defaults size to 28px, enlarging eyes and omitting catchlights", () => {
    const { container } = render(<Pebble />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("width")).toBe("28");
    expect(svg.getAttribute("height")).toBe("28");
    const ellipse = container.querySelector("ellipse")!;
    expect(ellipse.getAttribute("rx")).toBe("2.1");
    expect(ellipse.getAttribute("ry")).toBe("2.7");
    expect(container.querySelectorAll("circle[fill='#fff']").length).toBe(0);
  });

  it("above 28px, uses the mockup's small-eye geometry with catchlights", () => {
    const { container } = render(<Pebble size={56} />);
    const ellipse = container.querySelector("ellipse")!;
    expect(ellipse.getAttribute("rx")).toBe("1.9");
    expect(ellipse.getAttribute("ry")).toBe("2.5");
    expect(container.querySelectorAll("circle[fill='#fff']").length).toBe(2);
  });

  it("applies the size prop to width/height", () => {
    const { container } = render(<Pebble size={64} />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("width")).toBe("64");
    expect(svg.getAttribute("height")).toBe("64");
  });
});
