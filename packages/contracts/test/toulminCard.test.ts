import { expect, test } from "vitest";
import { CardSpec } from "../src/cardSpec";
import toulmin from "../cards/toulmin.json";

test("toulmin.json validates against CardSpec", () => {
  const r = CardSpec.safeParse(toulmin);
  if (!r.success) throw new Error(JSON.stringify(r.error.issues, null, 2));
  expect(r.success).toBe(true);
});
