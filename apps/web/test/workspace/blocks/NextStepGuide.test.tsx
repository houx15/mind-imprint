import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextStepGuide, deriveNextStep } from "@/workspace/blocks/NextStepGuide";

describe("deriveNextStep", () => {
  it("no plan yet → nothing (立项 already guides)", () => {
    expect(deriveNextStep({ hasPlan: false, writingFinished: false, room: "plan" })).toBeNull();
  });

  it("plan exists, not writing → offers 写作 (unless already there)", () => {
    expect(deriveNextStep({ hasPlan: true, writingFinished: false, room: "plan" })).toEqual({
      room: "writing",
      label: "一起去写作",
    });
    expect(deriveNextStep({ hasPlan: true, writingFinished: false, room: "writing" })).toBeNull();
  });

  it("draft finished → offers 回顾 (unless already there)", () => {
    expect(deriveNextStep({ hasPlan: true, writingFinished: true, room: "writing" })).toEqual({
      room: "reflection",
      label: "去回顾这段思考",
    });
    expect(deriveNextStep({ hasPlan: true, writingFinished: true, room: "reflection" })).toBeNull();
  });
});

describe("NextStepGuide", () => {
  it("renders nothing when there's no next step", () => {
    const { container } = render(
      <NextStepGuide hasPlan={false} writingFinished={false} room="plan" onGoRoom={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the offer and navigates on tap", async () => {
    const onGoRoom = vi.fn();
    render(<NextStepGuide hasPlan writingFinished={false} room="plan" onGoRoom={onGoRoom} />);
    const chip = screen.getByRole("button", { name: /一起去写作/ });
    await userEvent.click(chip);
    expect(onGoRoom).toHaveBeenCalledWith("writing");
  });
});
