import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StationRail } from "./StationRail";
import { STUDIO_FIXTURE } from "./fixtures";

describe("StationRail", () => {
  it("renders 7 stations and the current station's gate strip", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={() => {}} />);
    expect(screen.getByText("任务解码")).toBeInTheDocument();
    expect(screen.getByText("反思归档")).toBeInTheDocument();
    expect(screen.getByText(/本环节门禁 5 项，已过 2/)).toBeInTheDocument();
  });

  it("shows the 有据修正 backflow pill on S3", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={() => {}} />);
    expect(screen.getByText("有据修正")).toBeInTheDocument();
  });

  it("clicking a LOCKED station still fires onSelect (soft-lock preview)", () => {
    const onSelect = vi.fn();
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={onSelect} />);
    fireEvent.click(screen.getByText("反思归档")); // S6, locked
    expect(onSelect).toHaveBeenCalledWith("S6");
  });

  it("in focus mode hides station names", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={true} onSelect={() => {}} />);
    expect(screen.queryByText("任务解码")).not.toBeInTheDocument();
  });
});
