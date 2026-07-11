import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Bean } from "./Bean";

describe("Bean", () => {
  it("renders an svg and picks a light eye on a dark fill, dark eye on a light fill", () => {
    const { container: dark } = render(<Bean color="#2A3B7A" />);
    expect(dark.querySelector("svg")).toBeTruthy();
    expect(dark.querySelectorAll("ellipse")[0]!.getAttribute("fill")).toBe("#FFFFFF");
    const { container: light } = render(<Bean color="#E8A33D" />);
    expect(light.querySelectorAll("ellipse")[0]!.getAttribute("fill")).toBe("#17223B");
  });
});
