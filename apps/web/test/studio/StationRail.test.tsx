import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StationRail } from "@/studio/StationRail";
import { STUDIO_FIXTURE } from "@/studio/fixtures";
import type { Station } from "@/studio/state";

const noopReopen = () => {};

describe("StationRail", () => {
  it("renders 7 stations and the current station's gate strip", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={() => {}} onReopen={noopReopen} />);
    expect(screen.getByText("任务解码")).toBeInTheDocument();
    expect(screen.getByText("反思归档")).toBeInTheDocument();
    expect(screen.getByText(/本环节门禁 5 项，已过 2/)).toBeInTheDocument();
  });

  it("shows the 有据修正 backflow pill on S3", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={() => {}} onReopen={noopReopen} />);
    expect(screen.getByText("有据修正")).toBeInTheDocument();
  });

  it("clicking a LOCKED station still fires onSelect (soft-lock preview)", () => {
    const onSelect = vi.fn();
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={onSelect} onReopen={noopReopen} />);
    fireEvent.click(screen.getByText("反思归档")); // S6, locked
    expect(onSelect).toHaveBeenCalledWith("S6");
  });

  it("in focus mode hides station names", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={true} onSelect={() => {}} onReopen={noopReopen} />);
    expect(screen.queryByText("任务解码")).not.toBeInTheDocument();
  });
});

const waivedStations: Station[] = [
  { code: "S0", name: "任务解码", view: "评估", state: "waived" },
  { code: "S3", name: "信源评估", view: "素材", state: "current" },
] as Station[];

describe("StationRail waived", () => {
  it("renders a waived station as 已跳过 · 可恢复 and calls onReopen", () => {
    const onReopen = vi.fn();
    render(<StationRail stations={waivedStations} active="S3" focus={false} onSelect={() => {}} onReopen={onReopen} />);
    expect(screen.getByText(/已跳过/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/恢复/));
    expect(onReopen).toHaveBeenCalledWith("S0");
  });

  it("clicking 恢复 does not also fire onSelect for the row", () => {
    const onReopen = vi.fn();
    const onSelect = vi.fn();
    render(<StationRail stations={waivedStations} active="S3" focus={false} onSelect={onSelect} onReopen={onReopen} />);
    fireEvent.click(screen.getByText(/恢复/));
    expect(onSelect).not.toHaveBeenCalled();
  });
});
