import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Nav } from "@/shell/Nav";

describe("Nav", () => {
  it("renders the four entries in order: 首页 / 项目 / 课程 / 我", () => {
    render(<Nav tab="home" onTab={() => {}} user={{ display_name: "Phoebe" }} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(4);
    expect(tabs.map((t) => t.textContent)).toEqual(["首页", "项目", "课程", "P我"]);
  });

  it("marks the active tab with aria-selected + the active tile fill", () => {
    render(<Nav tab="courses" onTab={() => {}} user={{ display_name: "Phoebe" }} />);
    const tabs = screen.getAllByRole("tab");
    const courses = tabs.find((t) => t.textContent?.includes("课程"))!;
    expect(courses.getAttribute("aria-selected")).toBe("true");
    // On the accent rail the active entry is a translucent-white pill.
    expect(courses.className).toContain("bg-white/15");

    const home = tabs.find((t) => t.textContent?.includes("首页"))!;
    expect(home.getAttribute("aria-selected")).toBe("false");
    expect(home.className).not.toContain("bg-white/15");
  });

  it("fires onTab with the entry's key when clicked", () => {
    const onTab = vi.fn();
    render(<Nav tab="home" onTab={onTab} user={{ display_name: "Phoebe" }} />);
    fireEvent.click(screen.getByText("项目"));
    expect(onTab).toHaveBeenCalledWith("projects");
    fireEvent.click(screen.getByText("课程"));
    expect(onTab).toHaveBeenCalledWith("courses");
    fireEvent.click(screen.getByText("P"));
    expect(onTab).toHaveBeenCalledWith("me");
  });

  it("renders the user's display-name initial for 我 when no display name is given", () => {
    render(<Nav tab="home" onTab={() => {}} user={null} />);
    expect(screen.getByText("?")).toBeTruthy();
  });
});
