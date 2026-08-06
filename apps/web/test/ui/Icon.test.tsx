import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Icon } from "@/ui/Icon";
import { Search } from "lucide-react";

describe("Icon", () => {
  it("renders an svg with 1.75 stroke and given size", () => {
    const { container } = render(<Icon icon={Search} size={16} />);
    const svg = container.querySelector("svg")!;
    expect(svg).toBeInTheDocument();
    expect(svg.getAttribute("stroke-width")).toBe("1.75");
    expect(svg.getAttribute("width")).toBe("16");
  });

  it("defaults to size 20 when omitted", () => {
    const { container } = render(<Icon icon={Search} />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("width")).toBe("20");
    expect(svg.getAttribute("stroke-width")).toBe("1.75");
  });

  it("forwards other SVG props such as className", () => {
    const { container } = render(<Icon icon={Search} className="mk-icon" />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("class")).toContain("mk-icon");
  });
});
