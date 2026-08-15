import type { PdfEngine } from "../../src/media/pdfEngine";

/**
 * Deterministic {@link PdfEngine} for tests: records render/goToPage calls and
 * reports a fixed page count, so page-navigation bounds and `initialPage` can be
 * asserted without a real PDF library under jsdom.
 */
export class FakePdfEngine implements PdfEngine {
  readonly renders: Array<{ url: string; page: number }> = [];
  readonly gotos: number[] = [];

  constructor(private readonly total = 5) {}

  render(_container: HTMLElement, url: string, page: number): void {
    this.renders.push({ url, page });
  }

  goToPage(page: number): void {
    this.gotos.push(page);
  }

  totalPages(): number {
    return this.total;
  }

  factory = (): PdfEngine => this;
}
