import { describe, it, expect } from "vitest";
import { CONTRACTS_VERSION } from "../src/index";

describe("contracts smoke", () => {
  it("exposes a version", () => {
    expect(CONTRACTS_VERSION).toBe("0.0.0");
  });
});
