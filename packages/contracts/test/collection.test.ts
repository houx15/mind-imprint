import { describe, it, expect } from "vitest";
import { Collection } from "../src/collection";

describe("Collection", () => {
  it("parses a top-level collection (parentId null)", () => {
    const ok = Collection.parse({ id: "c1", name: "一手研究", parentId: null, position: 0 });
    expect(ok.parentId).toBeNull();
  });
  it("rejects a non-string name", () => {
    expect(() => Collection.parse({ id: "c1", name: 42, parentId: null, position: 0 } as any)).toThrow();
  });
});
