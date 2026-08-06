import { describe, it, expect, beforeEach, vi } from "vitest";
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

  it("initialAccent applies on mount and ignores a differing localStorage value", () => {
    localStorage.setItem("mk-accent", "teal");

    function Probe() {
      const { id } = useAccent();
      return <span data-id={id}>probe</span>;
    }
    render(
      <AccentProvider initialAccent="indigo">
        <Probe />
      </AccentProvider>,
    );
    expect(screen.getByText("probe").dataset.id).toBe("indigo");
    expect(document.documentElement.style.getPropertyValue("--mk-accent-500")).toBe("#3A63A8");
  });

  it("setAccent fires onPersist with the new id and still writes localStorage", () => {
    const onPersist = vi.fn();

    function Probe() {
      const { id, setAccent } = useAccent();
      return (
        <button data-id={id} onClick={() => setAccent("rose")}>
          go
        </button>
      );
    }
    render(
      <AccentProvider onPersist={onPersist}>
        <Probe />
      </AccentProvider>,
    );
    act(() => screen.getByRole("button").click());
    expect(screen.getByRole("button").dataset.id).toBe("rose");
    expect(localStorage.getItem("mk-accent")).toBe("rose");
    expect(onPersist).toHaveBeenCalledWith("rose");
  });

  it("a rejecting onPersist does not throw and does not block the local change", () => {
    const onPersist = vi.fn().mockRejectedValue(new Error("network down"));

    function Probe() {
      const { id, setAccent } = useAccent();
      return (
        <button data-id={id} onClick={() => setAccent("violet")}>
          go
        </button>
      );
    }
    render(
      <AccentProvider onPersist={onPersist}>
        <Probe />
      </AccentProvider>,
    );
    expect(() => act(() => screen.getByRole("button").click())).not.toThrow();
    expect(screen.getByRole("button").dataset.id).toBe("violet");
    expect(localStorage.getItem("mk-accent")).toBe("violet");
  });

  it("with no initialAccent/onPersist, mount still uses the localStorage-first path", () => {
    localStorage.setItem("mk-accent", "bamboo");

    function Probe() {
      const { id } = useAccent();
      return <span data-id={id}>probe</span>;
    }
    render(
      <AccentProvider>
        <Probe />
      </AccentProvider>,
    );
    expect(screen.getByText("probe").dataset.id).toBe("bamboo");
  });
});
