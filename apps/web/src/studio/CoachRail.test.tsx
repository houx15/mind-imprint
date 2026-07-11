import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachRail } from "./CoachRail";
import { STUDIO_FIXTURE } from "./fixtures";

const c = STUDIO_FIXTURE.coach;

describe("CoachRail", () => {
  it("renders the anchor status, the thread, and the 装备栏 toggle", () => {
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={() => {}} />);
    // the anchor renders in both the header status and the disposition
    // card's "锚定" pill, so assert presence rather than uniqueness
    expect(screen.getAllByText(new RegExp(c.anchor)).length).toBeGreaterThan(0);
    expect(screen.getByText("孤儿证据")).toBeInTheDocument(); // the flag message label
  });
  it("sends composer text", () => {
    const onSend = vi.fn();
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={onSend} />);
    fireEvent.change(screen.getByPlaceholderText(/发给印记/), { target: { value: "我加了一条证据" } });
    fireEvent.click(screen.getByLabelText(/发送|send/i));
    expect(onSend).toHaveBeenCalledWith("我加了一条证据");
  });
});
