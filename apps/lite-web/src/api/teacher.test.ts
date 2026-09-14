import { describe, expect, it, vi } from "vitest";

vi.mock("./client", () => ({
  apiFetch: vi.fn(async () => ({
    roster: [{ id: "u1", displayName: "林同学", minutesThisWeek: -1 }],
  })),
}));

import { getRoster } from "./teacher";

describe("getRoster", () => {
  it("fills missing numeric fields with 0 and keeps -1", async () => {
    const [row] = await getRoster("c1");
    if (!row) throw new Error("expected a roster row");
    expect(row.minutesThisWeek).toBe(-1);
    expect(row.turns).toBe(0);
    expect(row.lastActiveAt).toBeNull();
  });
});
