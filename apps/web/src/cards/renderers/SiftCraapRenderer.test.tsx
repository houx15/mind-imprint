import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SiftCraapRenderer } from "./SiftCraapRenderer";
import { pickCardBody } from "../customRenderers";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

const ALLOWED = new Set([
  "stop",
  "sources",
  "better",
  "trace",
  "currency",
  "relevance",
  "authority",
  "accuracy",
  "purpose",
]);

it("is registered for sift_craap", () => {
  expect(pickCardBody("sift_craap")).toBe(SiftCraapRenderer);
});

it("writes ONLY schema field keys via onField, across BOTH a SIFT and a CRAAP write (standard envelope guardrail)", async () => {
  // One shared recorder captures EVERY key the renderer writes across ALL interactions.
  const written: string[] = [];
  render(
    <SiftCraapRenderer
      card={CARD_REGISTRY["sift_craap"]!}
      values={{}}
      onField={(k) => written.push(k)}
      onExpandStep={() => {}}
    />,
  );

  // SIFT write — type into the Stop textarea.
  await userEvent.type(
    screen.getByLabelText(/你打算用这条信息说明什么/),
    "测试",
  );

  // CRAAP write — expand the on_demand CRAAP step, then click a rating control.
  await userEvent.click(screen.getByRole("button", { name: /CRAAP/ }));
  await userEvent.click(screen.getByLabelText(/Currency/));

  // (a) NO key outside the whitelist may EVER be written — including the CRAAP write.
  for (const k of written) expect(ALLOWED.has(k)).toBe(true);
  // (b) both a SIFT write and a CRAAP write were actually captured under the whitelist.
  expect(written).toContain("stop");
  expect(written).toContain("currency");
});
