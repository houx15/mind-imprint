import { createContext, useContext, useRef } from "react";

/**
 * §17.9 — the isolated PDF surface {@link PdfRenderer} needs. The PDF library
 * choice is an implementation decision behind this seam; the default is a
 * lightweight browser `<embed>`, and a maintained PDF engine can swap in later
 * without touching the renderer. Injected so tests use a deterministic fake
 * (jsdom cannot render PDFs).
 */
export interface PdfEngine {
  /** Render `url` into `container`, showing `page` (one-based). */
  render(container: HTMLElement, url: string, page: number): void;
  /** Navigate the already-rendered document to `page`. */
  goToPage(page: number): void;
  /** Total page count, or a lower-bound default when the engine cannot know it. */
  totalPages(): number;
}

/**
 * Default engine: embeds the intact PDF via a browser `<embed>` element and uses
 * the `#page=` fragment for navigation. Page count is unknown to a bare embed,
 * so {@link totalPages} reports 1 (best-effort); the real page count arrives when
 * a maintained PDF library replaces this.
 */
export class HtmlPdfEngine implements PdfEngine {
  private container: HTMLElement | null = null;
  private url = "";
  private page = 1;

  render(container: HTMLElement, url: string, page: number): void {
    this.container = container;
    this.url = url;
    this.page = page;
    this.paint();
  }

  goToPage(page: number): void {
    this.page = page;
    this.paint();
  }

  totalPages(): number {
    return 1;
  }

  private paint(): void {
    if (!this.container) return;
    this.container.innerHTML = "";
    const embed = document.createElement("embed");
    embed.setAttribute("type", "application/pdf");
    embed.setAttribute("src", `${this.url}#page=${this.page}`);
    embed.style.width = "100%";
    embed.style.height = "100%";
    this.container.appendChild(embed);
  }
}

/** A factory so each mounted PDF gets its own engine instance. */
export type PdfEngineFactory = () => PdfEngine;

const PdfEngineContext = createContext<PdfEngineFactory | null>(null);

export const PdfEngineProvider = PdfEngineContext.Provider;

/** Returns a stable {@link PdfEngine} for one mounted PDF (injected factory or default). */
export function usePdfEngine(): PdfEngine {
  const factory = useContext(PdfEngineContext);
  const ref = useRef<PdfEngine | null>(null);
  if (ref.current === null) ref.current = factory ? factory() : new HtmlPdfEngine();
  return ref.current;
}
