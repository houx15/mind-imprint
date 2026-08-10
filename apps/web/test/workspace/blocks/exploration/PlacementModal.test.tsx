import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PlacementModal } from "../../../../src/workspace/blocks/exploration/PlacementModal";
import * as expl from "../../../../src/api/exploration";

describe("PlacementModal", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("suggests then attaches on pick", async () => {
    vi.spyOn(expl, "getExploration").mockResolvedValue({
      leads: [{ id: "q1", text: "主问题", status: "open", origin: "guide", sourceReferenceId: null, connectedReferenceId: null, position: 0, parentLeadId: null, createdAt: "2026-08-10T00:00:00Z" }],
      danglingSourceIds: [],
      edges: [],
    } as any);
    vi.spyOn(expl, "suggestPlacement").mockResolvedValue({ leadId: "q1", reason: "很贴" });
    const attach = vi.spyOn(expl, "attachReference").mockResolvedValue({ lead: {} as any, reference: {} as any });
    const onAttached = vi.fn();
    const onClose = vi.fn();

    render(<PlacementModal projectId="p1" referenceId="r1" onClose={onClose} onAttached={onAttached} />);
    await waitFor(() => expect(screen.getByText("很贴")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /主问题/ }));
    await waitFor(() => expect(attach).toHaveBeenCalledWith("p1", "r1", "q1"));
    expect(onAttached).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });
});
