import { render } from "@testing-library/react";
import { INNER_PARTS } from "@/cards/teaching/assets/InnerPartsCast";
import { Battery } from "@/cards/teaching/assets/Battery";

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
