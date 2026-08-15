import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { BlockSessionState } from "@mind-imprint/course-contract";
import type { SliceEmitter } from "@mind-imprint/course-runtime";
import { PdfRenderer } from "../../src/blocks/media/PdfRenderer";
import type { PdfBlock } from "../../src/blocks/types";
import { PdfEngineProvider } from "../../src/media/pdfEngine";
import { FakePdfEngine } from "../support/fakePdfEngine";

const assetResolver = { resolve: (p: string) => `/resolved/${p}` };
const baseState: BlockSessionState = { visible: true, enabled: true, completed: false };

interface Recorded {
  sourceId: string;
  type: string;
  payload: unknown;
}

const pdfBlock: PdfBlock = {
  id: "source-paper",
  type: "pdf",
  title: "Original Research Paper",
  source: "assets/pdfs/source-paper.pdf",
  initialPage: 3,
};

function renderPdf(block: PdfBlock = pdfBlock, engine = new FakePdfEngine(5)) {
  const events: Recorded[] = [];
  const emit: SliceEmitter = (sourceId, type, payload) => events.push({ sourceId, type: String(type), payload });
  const utils = render(
    <PdfEngineProvider value={engine.factory}>
      <PdfRenderer block={block} assetResolver={assetResolver} state={baseState} visible enabled emit={emit} />
    </PdfEngineProvider>,
  );
  return { ...utils, events, engine };
}

const typeNames = (events: Recorded[]) => events.map((e) => e.type);

describe("PdfRenderer", () => {
  it("emits pdf.opened on first view and renders the intact PDF at initialPage", () => {
    const { events, engine } = renderPdf();
    expect(typeNames(events)).toContain("pdf.opened");
    expect(engine.renders[0]).toMatchObject({ url: "/resolved/assets/pdfs/source-paper.pdf", page: 3 });
  });

  it("clicking next emits pdf.pageChanged and asks the engine to go to the page", async () => {
    const user = userEvent.setup();
    const { events, engine } = renderPdf();
    await user.click(screen.getByRole("button", { name: "下一页" }));
    const changed = events.find((e) => e.type === "pdf.pageChanged");
    expect(changed).toMatchObject({ payload: { page: 4 } });
    expect(engine.gotos).toContain(4);
  });

  it("clicking download emits pdf.downloaded and links to the resolved source", async () => {
    const user = userEvent.setup();
    const { events, container } = renderPdf();
    const link = container.querySelector("a.course-pdf__download")!;
    expect(link).toHaveAttribute("href", "/resolved/assets/pdfs/source-paper.pdf");
    await user.click(link);
    expect(typeNames(events)).toContain("pdf.downloaded");
  });

  it("NEVER emits block.completed (page views are not learning completion, §9.3)", async () => {
    const user = userEvent.setup();
    const { events, container } = renderPdf();
    await user.click(screen.getByRole("button", { name: "下一页" }));
    await user.click(screen.getByRole("button", { name: "上一页" }));
    await user.click(container.querySelector("a.course-pdf__download")!);
    expect(typeNames(events)).not.toContain("block.completed");
  });

  it("does not page below 1 or above totalPages", async () => {
    const user = userEvent.setup();
    // initialPage 1, total 2
    const { events, engine } = renderPdf({ ...pdfBlock, initialPage: 1 }, new FakePdfEngine(2));
    await user.click(screen.getByRole("button", { name: "上一页" })); // clamp at 1: no change
    expect(events.filter((e) => e.type === "pdf.pageChanged")).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "下一页" })); // → 2
    await user.click(screen.getByRole("button", { name: "下一页" })); // clamp at 2: no change
    const changes = events.filter((e) => e.type === "pdf.pageChanged");
    expect(changes).toHaveLength(1);
    expect(changes[0]).toMatchObject({ payload: { page: 2 } });
    expect(engine.gotos).toEqual([2]);
  });
});
