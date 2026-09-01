import { afterEach, describe, expect, it } from "vitest";
import { applyTheme, readStoredTheme, storeTheme } from "./theme";

afterEach(() => {
  document.documentElement.removeAttribute("data-theme");
  try {
    localStorage.clear();
  } catch {
    /* nothing to clear */
  }
});

describe("applyTheme", () => {
  it("sets the attribute for dark and REMOVES it for light", () => {
    applyTheme("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    applyTheme("light");
    // Not `data-theme="light"`: the light palette has exactly one definition,
    // the bare :root block, and must not gain a second.
    expect(document.documentElement.getAttribute("data-theme")).toBeNull();
  });
});

describe("readStoredTheme", () => {
  it("reads a stored dark preference", () => {
    storeTheme("dark");
    expect(readStoredTheme()).toBe("dark");
  });

  it("falls back to light for a value it does not know", () => {
    localStorage.setItem("mk-theme", "midnight");
    expect(readStoredTheme()).toBe("light");
  });

  it("falls back to light when there is nothing stored", () => {
    expect(readStoredTheme()).toBe("light");
  });

  // Some browsers THROW on the localStorage accessor rather than returning
  // null. A theme preference must never cost someone the whole app.
  it("survives storage throwing outright", () => {
    const original = Object.getOwnPropertyDescriptor(window, "localStorage");
    Object.defineProperty(window, "localStorage", {
      get() {
        throw new Error("site data blocked");
      },
      configurable: true,
    });
    try {
      expect(() => readStoredTheme()).not.toThrow();
      expect(readStoredTheme()).toBe("light");
      expect(() => storeTheme("dark")).not.toThrow();
    } finally {
      if (original) Object.defineProperty(window, "localStorage", original);
    }
  });
});
