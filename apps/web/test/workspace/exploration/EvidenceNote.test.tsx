import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EvidenceNote } from "@/workspace/blocks/exploration/ExplorationSidebar";
import type { Reference } from "@mind-imprint/contracts";

function ref(over: Partial<Reference> = {}): Reference {
  return {
    id: "r1", title: "t", classification: "", author: "", credentials: "", year: "", url: "",
    tags: [], collectionId: null, credibility: null, evaluation: "", decision: null, pending: false,
    searchHints: [], materialId: null, notes: [], readingStatus: "to_read",
    ...over,
  } as Reference;
}

describe("EvidenceNote (slice 4a)", () => {
  it("triage / nature / note edits call the setters", () => {
    const onSetEvidence = vi.fn();
    const onSetTriage = vi.fn();
    const onArchive = vi.fn();
    render(<EvidenceNote reference={ref()} onSetEvidence={onSetEvidence} onSetTriage={onSetTriage} onArchive={onArchive} />);

    // triage 必读 → red
    fireEvent.click(screen.getByText("必读"));
    expect(onSetTriage).toHaveBeenCalledWith("red");

    // nature 反驳/张力 → challenge (with the current note fields)
    fireEvent.click(screen.getByText("反驳/张力"));
    expect(onSetEvidence).toHaveBeenCalledWith(expect.objectContaining({ nature: "challenge" }));

    // note field → on blur saves the argument
    const argField = screen.getByPlaceholderText("这篇在说什么？");
    fireEvent.change(argField, { target: { value: "投资降低碳排" } });
    fireEvent.blur(argField);
    expect(onSetEvidence).toHaveBeenCalledWith(expect.objectContaining({ argument: "投资降低碳排" }));

    // archive
    fireEvent.click(screen.getByText(/归档/));
    expect(onArchive).toHaveBeenCalled();
  });

  it("shows the active triage + nature from the reference", () => {
    render(<EvidenceNote reference={ref({ triage: "yellow", evidenceNature: "support" })} onSetEvidence={vi.fn()} onSetTriage={vi.fn()} onArchive={vi.fn()} />);
    // toggling 待定 off (it's active) sends ""
    const onSetTriage = vi.fn();
    render(<EvidenceNote reference={ref({ triage: "yellow" })} onSetEvidence={vi.fn()} onSetTriage={onSetTriage} onArchive={vi.fn()} />);
    fireEvent.click(screen.getAllByText("待定")[1]!);
    expect(onSetTriage).toHaveBeenCalledWith("");
  });
});
