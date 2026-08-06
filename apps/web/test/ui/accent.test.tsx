import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { ACCENT_PRESETS, applyAccent, AccentProvider, useAccent } from "@/ui/accent";

beforeEach(() => localStorage.clear());

describe("accent theming", () => {
  it("ships 8 presets with vermilion default first", () => {
    expect(ACCENT_PRESETS).toHaveLength(8);
    expect(ACCENT_PRESETS[0].id).toBe("vermilion");
    expect(ACCENT_PRESETS[0].scale[500]).toBe("#EA5140");
  });

  it("applyAccent sets the accent-500 variable", () => {
    const el = document.createElement("div");
    applyAccent(el, "teal");
    expect(el.style.getPropertyValue("--mk-accent-500")).toBe("#1F9488");
    expect(el.style.getPropertyValue("--mk-accent")).toBe("var(--mk-accent-500)");
  });

  it("useAccent persists selection", () => {
    function Probe() {
      const { id, setAccent } = useAccent();
      return (
        <button data-id={id} onClick={() => setAccent("rose")}>
          go
        </button>
      );
    }
    render(
      <AccentProvider>
        <Probe />
      </AccentProvider>,
    );
    expect(screen.getByRole("button").dataset.id).toBe("vermilion");
    act(() => screen.getByRole("button").click());
    expect(screen.getByRole("button").dataset.id).toBe("rose");
    expect(localStorage.getItem("mk-accent")).toBe("rose");
  });
});
