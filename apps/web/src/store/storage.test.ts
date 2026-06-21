import { describe, it, expect } from "vitest";
import { makeMemoryStorage, STORE_KEY } from "./storage";

describe("makeMemoryStorage", () => {
  it("returns null for a missing key", () => {
    const s = makeMemoryStorage();
    expect(s.getItem(STORE_KEY)).toBeNull();
  });
  it("round-trips a written value", () => {
    const s = makeMemoryStorage();
    s.setItem(STORE_KEY, "hello");
    expect(s.getItem(STORE_KEY)).toBe("hello");
  });
  it("overwrites on a second set", () => {
    const s = makeMemoryStorage();
    s.setItem(STORE_KEY, "a");
    s.setItem(STORE_KEY, "b");
    expect(s.getItem(STORE_KEY)).toBe("b");
  });
});
