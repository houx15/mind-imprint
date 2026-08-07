import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { ProjectCover } from "@/ui/ProjectCover";
import { coverGradientStyle } from "@/ui/cover";

describe("ProjectCover", () => {
  it("renders a full-bleed <img> for an img: cover with a resolved coverUrl", () => {
    const { container } = render(
      <ProjectCover project={{ id: "p1", title: "文章", cover: "img:3", coverUrl: "https://cdn.example.com/covers/3.webp" }} />,
    );
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute("src", "https://cdn.example.com/covers/3.webp");
    expect(img?.className).toContain("object-cover");
    // No gradient div rendered alongside it.
    expect(container.querySelectorAll("div").length).toBe(0);
  });

  it("falls back to the title-hash gradient when an img: cover has no coverUrl (unsigned)", () => {
    const { container } = render(<ProjectCover project={{ id: "p1", title: "文章", cover: "img:3", coverUrl: "" }} />);
    expect(container.querySelector("img")).toBeNull();
    const div = container.querySelector("div");
    expect(div).not.toBeNull();
    expect(div?.getAttribute("style")).toContain(coverGradientStyle("文章").backgroundImage as string);
  });

  it("renders the named macaron gradient for a grad: cover", () => {
    const { container } = render(<ProjectCover project={{ id: "p1", title: "文章", cover: "grad:matcha" }} />);
    expect(container.querySelector("img")).toBeNull();
    const div = container.querySelector("div");
    expect(div?.getAttribute("style")).toContain(coverGradientStyle("matcha").backgroundImage as string);
  });

  it("falls back to the deterministic title/id-hash gradient when cover is unset (old projects render unchanged)", () => {
    const { container } = render(<ProjectCover project={{ id: "p1", title: "旧项目标题" }} />);
    expect(container.querySelector("img")).toBeNull();
    const div = container.querySelector("div");
    expect(div?.getAttribute("style")).toContain(coverGradientStyle("旧项目标题").backgroundImage as string);
  });
});
