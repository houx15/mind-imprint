import { useMemo, useState } from "react";
import type { ShowcaseComponent } from "./showcaseTypes";

// Student code runs in an opaque-origin frame. It cannot connect to the network,
// navigate the parent, read platform cookies, or load external scripts.
export function componentDocument(source: string): string {
  return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html,body{margin:0;min-height:100%;overflow-wrap:anywhere}*{box-sizing:border-box}canvas,svg,img{max-width:100%}</style></head><body>${source}</body></html>`;
}
export function ShowcaseComponentPreview({ component }: {component: ShowcaseComponent}) {
  const document = useMemo(()=>componentDocument(component.source),[component.source]);
  const svgURL = useMemo(()=>`data:image/svg+xml;charset=utf-8,${encodeURIComponent(component.source)}`,[component.source]);
  return component.format === "svg"
    ? <img src={svgURL} alt={component.title} style={{width:"100%",height:component.height,objectFit:"contain"}}/>
    : <iframe title={component.title} srcDoc={document} sandbox="allow-scripts" referrerPolicy="no-referrer" allow="camera 'none'; microphone 'none'; geolocation 'none'; payment 'none'" style={{width:"100%",height:component.height,border:0,display:"block"}}/>;
}
export function ShowcaseComponents({ components, placement, editing }: {components?:ShowcaseComponent[];placement:ShowcaseComponent["placement"];editing?:boolean}) {
  const [running,setRunning]=useState<Record<string,ShowcaseComponent>>({});
  const items=(components??[]).filter(item=>item.enabled && item.placement===placement);
  if(!items.length)return null;
  return <section className="showcase-custom-components" aria-label="创作组件">{items.map(item=><article key={item.id}><h3>{item.title}</h3>{editing && item.format === "html" ? <>{running[item.id] ? <ShowcaseComponentPreview component={running[item.id]!}/> : <p>互动组件 · 请运行预览</p>}<button type="button" onClick={()=>setRunning(current=>({...current,[item.id]:{...item}}))}>运行预览</button><button type="button" disabled={!running[item.id]} onClick={()=>setRunning(current=>{const next={...current};delete next[item.id];return next;})}>停止预览</button></> : <ShowcaseComponentPreview component={item}/>}</article>)}</section>;
}
