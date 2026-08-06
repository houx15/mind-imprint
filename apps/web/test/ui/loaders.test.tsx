import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { PebbleProgress, PebbleInlineSpinner, RabbitHoleLoader } from "@/ui/loaders";

describe("PebbleProgress", () => {
  it("value=40 sets the fill element width to 40%", () => {
    const { getByTestId } = render(<PebbleProgress value={40} />);
    expect(getByTestId("pebble-progress-fill")).toHaveStyle({ width: "40%" });
  });

  it("clamps value=150 to 100%", () => {
    const { getByTestId } = render(<PebbleProgress value={150} />);
    expect(getByTestId("pebble-progress-fill")).toHaveStyle({ width: "100%" });
  });

  it("clamps value=-10 to 0%", () => {
    const { getByTestId } = render(<PebbleProgress value={-10} />);
    expect(getByTestId("pebble-progress-fill")).toHaveStyle({ width: "0%" });
  });

  it("with a value, does not apply the indeterminate looping animation class", () => {
    const { getByTestId } = render(<PebbleProgress value={40} />);
    expect(getByTestId("pebble-progress-fill").className).not.toContain("mk-pebble-indeterminate");
    expect(getByTestId("pebble-progress-rider").className).not.toContain("mk-pebble-indeterminate");
  });

  it("with a value, parks the rider at a computed left position instead of animating", () => {
    const { getByTestId } = render(<PebbleProgress value={40} />);
    const rider = getByTestId("pebble-progress-rider") as HTMLElement;
    expect(rider.style.left).toBeTruthy();
    expect(rider.style.left).toContain("40%");
  });

  it("no value → indeterminate: applies the looping fill/ride animation classes", () => {
    const { getByTestId } = render(<PebbleProgress />);
    expect(getByTestId("pebble-progress-fill").className).toContain("mk-pebble-indeterminate");
    expect(getByTestId("pebble-progress-rider").className).toContain("mk-pebble-indeterminate");
  });

  it("renders the 豆豆 rider body path (locked geometry)", () => {
    const { container } = render(<PebbleProgress value={40} />);
    const path = container.querySelector('[data-testid="pebble-progress-rider"] path');
    expect(path).toBeInTheDocument();
    expect(path!.getAttribute("d")).toMatch(/^M20 6/);
    expect(path!.getAttribute("fill")).toBe("var(--mk-accent-500)");
  });

  it("slim renders without crashing and keeps the same test hooks", () => {
    const { getByTestId } = render(<PebbleProgress value={20} slim />);
    expect(getByTestId("pebble-progress-fill")).toHaveStyle({ width: "20%" });
  });
});

describe("PebbleInlineSpinner", () => {
  it("renders the 豆豆 body path", () => {
    const { container } = render(<PebbleInlineSpinner />);
    const path = container.querySelector("path");
    expect(path).toBeInTheDocument();
    expect(path!.getAttribute("d")).toMatch(/^M20 6/);
    expect(path!.getAttribute("fill")).toBe("var(--mk-accent-500)");
  });

  it("defaults to size ~22px", () => {
    const { container } = render(<PebbleInlineSpinner />);
    const svg = container.querySelector("svg")!;
    expect(svg.getAttribute("width")).toBe("22");
  });

  it("applies the rolling animation wrapper class", () => {
    const { container } = render(<PebbleInlineSpinner />);
    expect(container.querySelector(".mk-pebble-inline-spin")).toBeInTheDocument();
  });
});

describe("RabbitHoleLoader", () => {
  it("renders the carrot path", () => {
    const { container } = render(<RabbitHoleLoader />);
    const carrot = container.querySelector(".mk-pebble-hole-carrot");
    expect(carrot).toBeInTheDocument();
    expect(carrot!.querySelectorAll("path").length).toBeGreaterThan(0);
  });

  it("renders the hole layers (rim, dark, lip)", () => {
    const { container } = render(<RabbitHoleLoader />);
    expect(container.querySelector(".mk-pebble-hole-rim")).toBeInTheDocument();
    expect(container.querySelector(".mk-pebble-hole-dark")).toBeInTheDocument();
    expect(container.querySelector(".mk-pebble-hole-lip")).toBeInTheDocument();
  });

  it("renders the hopping 豆豆", () => {
    const { container } = render(<RabbitHoleLoader />);
    const dou = container.querySelector(".mk-pebble-hole-dou");
    expect(dou).toBeInTheDocument();
    const path = dou!.querySelector("path");
    expect(path!.getAttribute("d")).toMatch(/^M20 6/);
  });

  it("defaults the caption to 正在钻兔子洞…", () => {
    const { getByText } = render(<RabbitHoleLoader />);
    expect(getByText("正在钻兔子洞…")).toBeInTheDocument();
  });

  it("accepts a custom caption", () => {
    const { getByText } = render(<RabbitHoleLoader caption="加载中…" />);
    expect(getByText("加载中…")).toBeInTheDocument();
  });
});
