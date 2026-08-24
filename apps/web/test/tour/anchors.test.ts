import { describe, it, expect, vi, afterEach } from "vitest";
import { resolveAnchor } from "@/tour/anchors";

afterEach(() => { document.body.innerHTML = ""; vi.useRealTimers(); });

describe("resolveAnchor", () => {
  it("resolves immediately when the element is already present", async () => {
    const el = document.createElement("div");
    el.setAttribute("data-tour", "x");
    document.body.appendChild(el);
    await expect(resolveAnchor('[data-tour="x"]')).resolves.toBe(el);
  });

  it("resolves null after the timeout when the element never appears", async () => {
    vi.useFakeTimers();
    const p = resolveAnchor('[data-tour="missing"]', { timeoutMs: 200, intervalMs: 50 });
    await vi.advanceTimersByTimeAsync(250);
    await expect(p).resolves.toBeNull();
  });
});
