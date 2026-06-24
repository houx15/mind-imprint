import { render } from "@testing-library/react";
import { StopIcon, InvestigateIcon, FindIcon, TraceIcon, CraapIcon } from "./SiftIcons";
import { INNER_PARTS } from "./InnerPartsCast";
import { Battery } from "./Battery";
import { Radar } from "./Radar";
import { SourceChip } from "./SourceChip";

it("renders all five SIFT icons as svg", () => {
  for (const Icon of [StopIcon, InvestigateIcon, FindIcon, TraceIcon, CraapIcon]) {
    const { container } = render(<Icon size={48} />);
    expect(container.querySelector("svg")).toBeTruthy();
  }
});

it("exposes the six inner parts with names, quotes and avatars", () => {
  expect(INNER_PARTS.map((p) => p.key)).toEqual(
    ["protector","perfect","procrastinator","anxious","pleaser","littleadult"]);
  for (const p of INNER_PARTS) {
    expect(p.name).toBeTruthy(); expect(p.quote).toBeTruthy(); expect(p.protects).toBeTruthy();
    const { container } = render(<p.Avatar size={48} />);
    expect(container.querySelector("svg")).toBeTruthy();
  }
});

it("battery renders `level` filled cells out of max", () => {
  const { container } = render(<Battery level={2} max={5} />);
  expect(container.querySelectorAll('[data-cell="on"]').length).toBe(2);
});

it("radar and source chip render", () => {
  expect(render(<Radar values={[4,3,5,2,4]} labels={["时效","相关","权威","准确","目的"]} />).container.querySelector("svg")).toBeTruthy();
  expect(render(<SourceChip name="NASA" verdict="ok" />).container.textContent).toContain("NASA");
});
