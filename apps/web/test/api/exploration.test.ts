import { describe, it, expect, vi, beforeEach } from "vitest";
import * as client from "../../src/api/client";
import { attachReference, suggestPlacement } from "../../src/api/exploration";

describe("source-placement client", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("attachReference POSTs referenceId+parentLeadId and parses the lead", async () => {
    const spy = vi.spyOn(client, "apiFetch").mockResolvedValue({
      lead: { id: "l1", text: "T", status: "connected", origin: "manual", sourceReferenceId: null, connectedReferenceId: "r1", position: 0, parentLeadId: "q1", createdAt: "2026-08-10T00:00:00Z" },
      reference: { id: "r1", title: "T" },
    } as unknown);
    const out = await attachReference("p1", "r1", "q1");
    expect(spy).toHaveBeenCalledWith("/api/v1/projects/p1/exploration/attach", expect.objectContaining({ method: "POST" }));
    const body = JSON.parse((spy.mock.calls[0]![1] as { body: string }).body);
    expect(body).toEqual({ referenceId: "r1", parentLeadId: "q1" });
    expect(out.lead.connectedReferenceId).toBe("r1");
  });

  it("suggestPlacement parses a null leadId", async () => {
    vi.spyOn(client, "apiFetch").mockResolvedValue({ leadId: null, reason: "" } as unknown);
    expect(await suggestPlacement("p1", "r1")).toEqual({ leadId: null, reason: "" });
  });
});
