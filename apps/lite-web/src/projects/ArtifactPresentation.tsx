import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Artifact } from "../api/artifacts";
import { artifactStatus } from "../api/artifactExport";
import { artifactSlides } from "./artifactSlides";

export function ArtifactPresentation({ artifact }: { artifact: Artifact }) {
  const [open, setOpen] = useState(false);
  const [index, setIndex] = useState(0);
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const slides = artifactSlides(artifact.payload.body ?? "");
  if (slides.length && (artifact.guessed.length || artifact.admits.length)) {
    slides.push({ title: "假设与局限", markdown: ["## 假设与局限",
      artifact.guessed.length ? "### AI 标注的假设\n" + artifact.guessed.map(x => `- ${x}`).join("\n") : "",
      artifact.admits.length ? "### 待核实与局限\n" + artifact.admits.map(x => `- ${x}`).join("\n") : "",
    ].filter(Boolean).join("\n\n") });
  }
  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);
  if (slides.length < 2) return null;
  const current = slides[Math.min(index, slides.length - 1)]!;
  const close = () => setOpen(false);
  return <>
    <button ref={trigger} type="button" className="rounded-full bg-mk-accent-500 px-5 py-2 font-semibold text-white" onClick={() => { setIndex(0); setOpen(true); }}>分步展示</button>
    {createPortal(<dialog ref={dialog} aria-label="成果分步展示" onCancel={close} onClose={() => { close(); trigger.current?.focus(); }}
      onKeyDown={e => {
        if (e.key === "ArrowRight") { e.preventDefault(); setIndex(n => Math.min(n + 1, slides.length - 1)); }
        if (e.key === "ArrowLeft") { e.preventDefault(); setIndex(n => Math.max(n - 1, 0)); }
      }} className="fixed inset-0 m-0 h-[100dvh] max-h-none w-screen max-w-none border-0 bg-mk-bg p-0 text-mk-ink backdrop:bg-black/50">
      <div className="flex h-full flex-col">
        <header className="flex shrink-0 items-center justify-between gap-4 border-b border-mk-border px-5 py-4 sm:px-10">
          <div className="min-w-0"><p className="truncate text-mk-small text-mk-secondary">{artifact.title}</p><p className="text-mk-small">{artifactStatus(artifact)}</p></div>
          <button type="button" className="shrink-0 rounded-full border border-mk-border px-4 py-2" onClick={close}>结束展示</button>
        </header>
        <main key={index} className="min-h-0 flex-1 overflow-y-auto px-6 py-8 sm:px-16 sm:py-12">
          <div className="mx-auto max-w-4xl">
            <p className="mb-7 text-sm font-semibold tracking-widest text-mk-accent-500" aria-live="polite">{String(index + 1).padStart(2, "0")} / {String(slides.length).padStart(2, "0")}</p>
            <article className="break-words text-lg leading-relaxed sm:text-2xl [&_h1]:mb-8 [&_h1]:text-3xl [&_h1]:font-semibold sm:[&_h1]:text-5xl [&_h2]:mb-8 [&_h2]:text-3xl [&_h2]:font-semibold sm:[&_h2]:text-4xl [&_h3]:mb-4 [&_h3]:mt-8 [&_h3]:font-semibold [&_p]:my-5 [&_ul]:list-disc [&_ul]:pl-7 [&_ol]:list-decimal [&_ol]:pl-7 [&_li]:my-3 [&_a]:underline [&_pre]:overflow-x-auto [&_table]:block [&_table]:overflow-x-auto [&_blockquote]:border-l-4 [&_blockquote]:border-mk-accent-500 [&_blockquote]:pl-5">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{current.markdown}</ReactMarkdown>
            </article>
          </div>
        </main>
        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-mk-border px-5 py-4 sm:px-10">
          <button type="button" disabled={index === 0} onClick={() => setIndex(n => n - 1)} className="rounded-full border border-mk-border px-5 py-2 disabled:opacity-30">上一页</button>
          <p className="hidden text-mk-small text-mk-secondary sm:block">← → 切换 · Esc 结束</p>
          <button type="button" onClick={() => index === slides.length - 1 ? close() : setIndex(n => n + 1)} className="rounded-full bg-mk-accent-500 px-5 py-2 font-semibold text-white">{index === slides.length - 1 ? "结束展示" : "下一页"}</button>
        </footer>
      </div>
    </dialog>, document.body)}
  </>;
}
