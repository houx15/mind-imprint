import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InnerPartsRenderer } from "./InnerPartsRenderer";
import { pickCardBody } from "../customRenderers";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

const ALLOWED = new Set([
  "energy_level",
  "inner_part_choice",
  "not_want_reason",
  "micro_action_choice",
]);

it("is registered for emotional-alignment", () => {
  expect(pickCardBody("emotional-alignment")).toBe(InnerPartsRenderer);
});

it("character cards write the exact JSON option string into inner_part_choice", async () => {
  const vals: Record<string, unknown> = {};
  render(
    <InnerPartsRenderer
      card={CARD_REGISTRY["emotional-alignment"]!}
      values={vals}
      onField={(k, v) => {
        vals[k] = v;
      }}
      onExpandStep={() => {}}
    />,
  );
  await userEvent.click(screen.getByText("保护者"));
  expect(vals["inner_part_choice"]).toBe(
    "保护者——「别去冒险，可能会受伤」",
  );
});

it("writes only schema keys (guardrail)", async () => {
  const writes: string[] = [];
  const vals: Record<string, unknown> = {};
  render(
    <InnerPartsRenderer
      card={CARD_REGISTRY["emotional-alignment"]!}
      values={vals}
      onField={(k, v) => {
        writes.push(k);
        vals[k] = v;
      }}
      onExpandStep={() => {}}
    />,
  );

  // 1. Battery: click first cell → energy_level written
  await userEvent.click(screen.getByLabelText(/电量/));

  // 2. Character card: click 完美小人 → inner_part_choice written
  await userEvent.click(screen.getByText("完美小人"));

  // 3. Textarea: type into not_want_reason
  await userEvent.type(
    screen.getByLabelText(/如果你想说说/),
    "测试",
  );

  // 4. Micro-action: expand on_demand step then click a button
  await userEvent.click(screen.getByRole("button", { name: /3 分钟小行动/ }));
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const microOption = (CARD_REGISTRY["emotional-alignment"]!.steps
    .find((s) => s.key === "micro_action")!
    .fields[0] as any).options[0] as string;
  await userEvent.click(screen.getByText(microOption));

  // (a) NO key outside ALLOWED may ever be written
  for (const k of writes) expect(ALLOWED.has(k)).toBe(true);

  // (b) All four field paths were actually exercised — non-vacuous
  expect(writes).toContain("energy_level");
  expect(writes).toContain("inner_part_choice");
  expect(writes).toContain("not_want_reason");
  expect(writes).toContain("micro_action_choice");
});
