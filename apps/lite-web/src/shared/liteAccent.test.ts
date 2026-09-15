import { describe, expect, it } from "vitest";
import { coerceLiteAccent, initialLiteAccent, LITE_DEFAULT_ACCENT } from "./liteAccent";

describe("coerceLiteAccent", () => {
  it("keeps a lite preset id", () => expect(coerceLiteAccent("indigo")).toBe("indigo"));
  it.each([["#4A5578"], ["coral"], [""], [null], [undefined]])("drops %s", (value) =>
    expect(coerceLiteAccent(value)).toBeUndefined(),
  );
});

describe("initialLiteAccent", () => {
  it("prefers the server's saved preset", () => expect(initialLiteAccent("violet", () => "rose")).toBe("violet"));
  it("falls back to the browser's preset when the server value is not a preset", () =>
    expect(initialLiteAccent("#4A5578", () => "rose")).toBe("rose"));
  it("uses the lite default when neither is a preset", () =>
    expect(initialLiteAccent(null, () => "coral")).toBe(LITE_DEFAULT_ACCENT));
  it("uses the lite default when storage throws", () =>
    expect(
      initialLiteAccent(undefined, () => {
        throw new Error("blocked");
      }),
    ).toBe(LITE_DEFAULT_ACCENT));
});
