import { useEffect, useRef, useState } from "react";
import type { Artifact } from "../api/artifacts";
import { artifactHTML, downloadArtifact } from "../api/artifactExport";
import { readPrintLayout } from "../api/printLayout";

export function FoldoutPreview({artifact}: {artifact: Artifact}) {
  const [overflow, setOverflow] = useState<boolean | null>(null);
  const [fit, setFit] = useState(true);
  const [width, setWidth] = useState(0);
  const container = useRef<HTMLDivElement>(null);
  const layout = readPrintLayout(artifact.payload.printLayout);
  useEffect(() => {
    const node = container.current;
    if (!node) return;
    const observer = new ResizeObserver(() => setWidth(node.clientWidth));
    observer.observe(node);
    setWidth(node.clientWidth);
    return () => observer.disconnect();
  }, [artifact.id, Boolean(layout)]);
  const scale = fit && width > 0 ? Math.min(1, width / 1123) : 1;
  if (!layout) return null;
  return <section className="my-4 rounded-mk-md border border-mk-border p-3">
    <h3 className="font-semibold">折页预览</h3>
    <p className="my-2 text-mk-small">A4横向单面，六面依次排列，背面留白。请检查查找顺序，再放大检查文字。</p>
    <div className="mb-2 flex gap-3">
      <button type="button" aria-pressed={fit} className="underline" onClick={() => setFit(true)}>查看全页</button>
      <button type="button" aria-pressed={!fit} className="underline" onClick={() => setFit(false)}>放大文字</button>
    </div>
    <div ref={container} className="overflow-auto border bg-white" style={{height:Math.ceil(795 * scale) + 2,maxHeight:650}}>
    <div style={{width:1123 * scale,height:795 * scale,position:"relative"}}>
    <iframe title="折页打印预览" sandbox="allow-same-origin" srcDoc={artifactHTML(artifact)} className="border-0 bg-white" style={{width:1123,height:795,transform:`scale(${scale})`,transformOrigin:"top left",position:"absolute"}} onLoad={e => {
      const panels = e.currentTarget.contentDocument?.querySelectorAll<HTMLElement>(".content");
      setOverflow(!panels || panels.length !== 6 || [...panels].some(p => p.scrollHeight > p.clientHeight + 1 || p.scrollWidth > p.clientWidth + 1));
    }}/>
    </div></div>
    {overflow && <p role="alert" className="my-2 text-red-700">分面内容超出可打印范围，请缩短内容后重新生成。</p>}
    <p className="my-2 text-mk-small">打印设置：A4横向、单面、实际大小100%，关闭页眉页脚。沿虚线交替向前、向后折，第一面作封面。请先试印一张检查边缘和文字。</p>
    <button type="button" className="mt-2 underline disabled:opacity-40" disabled={overflow !== false} onClick={() => downloadArtifact(artifact,"html")}>导出折页打印版</button>
  </section>;
}
