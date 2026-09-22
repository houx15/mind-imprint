import { useState } from "react";
import type { ShowcaseCustomWork } from "../site/showcaseTypes";
export function ShowcaseLinkEditor({items,onChange}:{items:ShowcaseCustomWork[];onChange:(items:ShowcaseCustomWork[])=>void}) {
  const [error,setError]=useState('');
  const [expanded,setExpanded]=useState<string>();
  const update=(id:string,value:Partial<ShowcaseCustomWork>)=>onChange(items.map(item=>item.id===id?{...item,...value}:item));
  return <section className="showcase-field-group"><h3>外部作品链接</h3><p className="showcase-mode-help">添加自己制作的网站、作品集或公开项目链接，再在下方勾选展示。</p>
    {items.map(item=><div className="showcase-link-editor" key={item.id}><button type="button" aria-expanded={expanded===item.id} onClick={()=>setExpanded(expanded===item.id?undefined:item.id)}>{item.title||"未命名作品"}</button>{expanded===item.id && <>
      <label className="showcase-field">作品名称<input value={item.title} maxLength={80} onChange={e=>update(item.id,{title:e.target.value})}/></label>
      <label className="showcase-field">作品链接<input type="url" placeholder="https://" value={item.url} maxLength={2048} onChange={e=>update(item.id,{url:e.target.value})}/></label>
      <label className="showcase-field">简介<textarea rows={2} value={item.summary} maxLength={500} onChange={e=>update(item.id,{summary:e.target.value})}/></label>
      <label className="showcase-field">完成日期<input type="date" value={item.date??''} onChange={e=>update(item.id,{date:e.target.value})}/></label>
      <button type="button" onClick={()=>onChange(items.filter(other=>other.id!==item.id))}>移除链接</button>
    </>}</div>)}
    {error&&<p role="alert">{error}</p>}
    <button type="button" className="showcase-add-link" onClick={()=>{if(items.length>=100){setError('最多添加 100 个作品链接');return;}const id=crypto.randomUUID();onChange([...items,{id,title:'',summary:'',url:''}]);setExpanded(id);}}>添加作品链接</button>
  </section>;
}
