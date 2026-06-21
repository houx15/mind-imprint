import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import argument from "../cards/argument-map.json";
import money from "../cards/money-trail.json";
import source from "../cards/source-map.json";

describe("画布导图 cards un-stubbed (composed)", () => {
  for (const [name, json] of [["argument-map", argument], ["money-trail", money], ["source-map", source]] as const) {
    it(`${name} is full and uses a repeatable_group`, () => {
      const c = CardSpec.parse(json);
      expect(c.body_status).toBe("full");
      expect(c.steps.flatMap((s) => s.fields).some((f) => f.type === "repeatable_group")).toBe(true);
    });
  }
});
