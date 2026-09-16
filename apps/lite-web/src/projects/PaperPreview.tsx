import {useEffect, useRef, useState} from "react";
import type {Artifact} from "../api/artifacts";
import {downloadPaper, paperHTML, readPaperLayout} from "../api/paperLayout";

export function PaperPreview({artifact, heading="纸面原型", exportable=true}:{artifact:Artifact; heading?:string; exportable?:boolean}) {
  const layout=readPaperLayout(artifact.payload.paperLayout);
  const container=useRef<HTMLDivElement>(null);
  const [width,setWidth]=useState(0);
  const [fit,setFit]=useState(true);
  const [checked,setChecked]=useState(false);
  const [overflow,setOverflow]=useState(false);
  useEffect(()=>{
    const node=container.current;if(!node)return;
    const resize=new ResizeObserver(()=>setWidth(node.clientWidth));resize.observe(node);setWidth(node.clientWidth);
    return()=>resize.disconnect();
  },[artifact.id,Boolean(layout)]);
  if(!layout)return null;
  const scale=fit&&width>0?Math.min(1,width/794,718/1123):1;
  return <section className="my-5 rounded-xl border border-mk-border bg-mk-paper p-4">
    <header className="mb-3 flex flex-wrap items-center justify-between gap-3"><div><h3 className="font-semibold">{heading}</h3><p className="mt-1 text-mk-small text-mk-muted">A4 单页 · 图形、剪裁与记录区域</p></div><div className="flex gap-2"><button type="button" aria-pressed={fit} onClick={()=>setFit(true)} className="rounded-full border border-mk-border px-3 py-1.5 text-mk-small">查看全页</button><button type="button" aria-pressed={!fit} onClick={()=>setFit(false)} className="rounded-full border border-mk-border px-3 py-1.5 text-mk-small">放大检查</button></div></header>
    <div ref={container} className="overflow-auto rounded border border-mk-border bg-white" style={{height:Math.ceil(1123*scale)+2,maxHeight:720}}><div style={{width:794*scale,height:1123*scale,position:"relative"}}><iframe title={heading === "纸面原型" ? "A4纸面原型预览" : `${heading}纸面预览`} sandbox="allow-same-origin" srcDoc={paperHTML(layout)} style={{width:794,height:1123,border:0,position:"absolute",transform:`scale(${scale})`,transformOrigin:"top left"}} onLoad={async e=>{
      const doc=e.currentTarget.contentDocument;if(!doc)return;
      await doc.fonts.ready;
      const elements=doc.querySelectorAll<SVGGraphicsElement>("#paper-elements > *");
      const outside=[...elements].some(el=>{const b=el.getBBox();return b.x<11||b.y<34||b.x+b.width>199||b.y+b.height>278;});
      setOverflow(outside);setChecked(elements.length===layout.elements.length);
    }}/></div></div>
    {overflow&&<p role="alert" className="mt-3 text-red-700">图形或文字超出页面范围，请修改后再打印。</p>}
    <p className="mt-3 text-mk-small text-mk-muted">请先放大检查文字与剪裁间距。打印时选择 A4 竖向、100% 实际大小，关闭页眉页脚。可移动部件需要打印、剪下后使用；请先试印一张。</p>
    {exportable&&<button type="button" disabled={!checked||overflow} onClick={()=>downloadPaper(layout)} className="mt-3 rounded-full bg-mk-accent-500 px-4 py-2 text-mk-small text-white disabled:opacity-40">导出 A4 图形打印版</button>}
  </section>;
}
