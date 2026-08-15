import { useEffect, useRef, useState } from "react";
import type { BlockRenderer, PdfBlock } from "../types";
import { usePdfEngine } from "../../media/pdfEngine";

/**
 * §9.3 / §17.9 — the isolated PDF viewer. It embeds the intact, teacher-confirmed
 * PDF through the injectable {@link PdfEngine} seam, honors `initialPage`, exposes
 * page navigation and a download action, and emits behavior Events
 * (`pdf.opened` / `pdf.pageChanged` / `pdf.downloaded`).
 *
 * It NEVER emits `block.completed`: opening, paging, or downloading a PDF is not
 * evidence of reading or understanding (§9.3). Learning evidence comes from a
 * separate assessment or interaction.
 */
export const PdfRenderer: BlockRenderer<PdfBlock> = ({ block, assetResolver, visible, emit }) => {
  const engine = usePdfEngine();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const url = assetResolver.resolve(block.source);
  const initialPage = block.initialPage ?? 1;
  const [page, setPage] = useState(initialPage);

  // First view: render at the initial page and record the open Event once.
  useEffect(() => {
    if (containerRef.current) engine.render(containerRef.current, url, initialPage);
    emit(block.id, "pdf.opened", { page: initialPage });
    // Mount-once open; block identity is stable for a mounted PdfRenderer.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const totalPages = engine.totalPages();

  const goTo = (next: number) => {
    const clamped = Math.max(1, Math.min(totalPages, next));
    if (clamped === page) return;
    setPage(clamped);
    engine.goToPage(clamped);
    emit(block.id, "pdf.pageChanged", { page: clamped });
  };

  return (
    <div
      data-block-id={block.id}
      data-block-type="pdf"
      hidden={!visible}
      aria-hidden={!visible}
      className="course-block course-block--pdf"
    >
      <div className="course-pdf__header">
        <span className="course-pdf__title">{block.title}</span>
        <a
          className="course-pdf__download"
          href={url}
          download
          onClick={() => emit(block.id, "pdf.downloaded", {})}
        >
          下载原文
        </a>
      </div>
      <div ref={containerRef} className="course-pdf__viewport" aria-label={block.title} />
      <div className="course-pdf__nav">
        <button type="button" className="course-pdf__prev" onClick={() => goTo(page - 1)} disabled={page <= 1}>
          上一页
        </button>
        <span className="course-pdf__page" data-pdf-page>
          {page}
        </span>
        <button type="button" className="course-pdf__next" onClick={() => goTo(page + 1)} disabled={page >= totalPages}>
          下一页
        </button>
      </div>
    </div>
  );
};
