import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Skeleton, SkeletonText, SkeletonCard, SkeletonRow } from "@/ui/Skeleton";

describe("Skeleton", () => {
  it("renders a .mk-skeleton box", () => {
    const { container } = render(<Skeleton />);
    expect(container.querySelector(".mk-skeleton")).toBeInTheDocument();
  });

  it("applies exactly one rounded-mk-* class for the requested radius", () => {
    const { container } = render(<Skeleton radius="md" />);
    const el = container.querySelector(".mk-skeleton") as HTMLElement;
    expect(el.className).toContain("rounded-mk-md");
    expect(el.className).not.toContain("rounded-mk-sm");
    expect(el.className).not.toContain("rounded-mk-full");
  });

  it("defaults to radius='sm'", () => {
    const { container } = render(<Skeleton />);
    const el = container.querySelector(".mk-skeleton") as HTMLElement;
    expect(el.className).toContain("rounded-mk-sm");
  });

  it("applies numeric w/h as inline pixel dimensions", () => {
    const { container } = render(<Skeleton w={80} h={20} />);
    const el = container.querySelector(".mk-skeleton") as HTMLElement;
    expect(el.style.width).toBe("80px");
    expect(el.style.height).toBe("20px");
  });

  it("passes string w/h through verbatim", () => {
    const { container } = render(<Skeleton w="60%" h="2rem" />);
    const el = container.querySelector(".mk-skeleton") as HTMLElement;
    expect(el.style.width).toBe("60%");
    expect(el.style.height).toBe("2rem");
  });

  it("defaults height to ~14px when omitted", () => {
    const { container } = render(<Skeleton />);
    const el = container.querySelector(".mk-skeleton") as HTMLElement;
    expect(el.style.height).toBe("14px");
  });
});

describe("SkeletonText", () => {
  it("renders exactly `lines` bars, each carrying .mk-skeleton", () => {
    const { container } = render(<SkeletonText lines={3} />);
    const bars = container.querySelectorAll(".mk-skeleton");
    expect(bars.length).toBe(3);
  });

  it("defaults to 3 lines", () => {
    const { container } = render(<SkeletonText />);
    expect(container.querySelectorAll(".mk-skeleton").length).toBe(3);
  });

  it("shortens the last bar for a ragged edge", () => {
    const { container } = render(<SkeletonText lines={2} />);
    const bars = Array.from(container.querySelectorAll(".mk-skeleton")) as HTMLElement[];
    const last = bars.at(-1);
    expect(last).toBeDefined();
    expect(last!.style.width).toBe("60%");
  });
});

describe("SkeletonCard", () => {
  it("renders .mk-skeleton descendants (cover + text lines)", () => {
    const { container } = render(<SkeletonCard />);
    const bars = container.querySelectorAll(".mk-skeleton");
    // cover block + 2 text lines
    expect(bars.length).toBe(3);
  });
});

describe("SkeletonRow", () => {
  it("renders .mk-skeleton descendants (thumb + two short lines)", () => {
    const { container } = render(<SkeletonRow />);
    const bars = container.querySelectorAll(".mk-skeleton");
    // 40px thumb + 2 text lines
    expect(bars.length).toBe(3);
  });
});
