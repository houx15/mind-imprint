import { useRef, useState } from "react";
import type { ShowcaseComponent } from "../site/showcaseTypes";
import { ShowcaseComponentPreview } from "../site/ShowcaseComponents";

const SVG_EXAMPLE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 280"><rect width="600" height="280" rx="24" fill="#102b3f"/><circle cx="300" cy="135" r="66" fill="#98d9cd"/><ellipse cx="300" cy="135" rx="125" ry="22" fill="none" stroke="#f4cc7c" stroke-width="8" transform="rotate(-20 300 135)"/><text x="300" y="248" text-anchor="middle" fill="white" font-size="18">我的小星球</text></svg>`;
const HTML_EXAMPLE = `<canvas id="art" width="600" height="280" aria-label="点击画布种一朵花"></canvas><script>
const canvas = document.getElementById('art');
const ctx = canvas.getContext('2d');
ctx.fillStyle='#f4f1e6';ctx.fillRect(0,0,600,280);
ctx.fillStyle='#38584c';ctx.font='18px sans-serif';ctx.fillText('点击画布，种一朵花',24,36);
canvas.addEventListener('click',event=>{const r=canvas.getBoundingClientRect();const x=(event.clientX-r.left)*600/r.width,y=(event.clientY-r.top)*280/r.height;ctx.strokeStyle='#579475';ctx.beginPath();ctx.moveTo(x,y);ctx.lineTo(x,y+45);ctx.stroke();for(let i=0;i<6;i++){ctx.fillStyle='#e6a0aa';ctx.beginPath();ctx.arc(x+Math.cos(i*Math.PI/3)*10,y+Math.sin(i*Math.PI/3)*10,8,0,Math.PI*2);ctx.fill();}ctx.fillStyle='#eecb6e';ctx.beginPath();ctx.arc(x,y,6,0,Math.PI*2);ctx.fill();});
</script>`;
export function ShowcaseComponentEditor({components,onChange,disabled}:{components:ShowcaseComponent[];onChange:(items:ShowcaseComponent[])=>void;disabled:boolean}) {
  const [selected,setSelected]=useState<string>();
  const [error,setError]=useState("");
  const [preview,setPreview]=useState<ShowcaseComponent|null>(null);
  const fileInput=useRef<HTMLInputElement>(null);
  const current=components.find(item=>item.id===selected)??components[0];
  const patch=(change:Partial<ShowcaseComponent>)=>{if(current)onChange(components.map(item=>item.id===current.id?{...item,...change}:item));};
  const add=(format:"svg"|"html",source:string,title:string)=>{
    if(components.length>=6){setError("最多添加 6 个组件");return;}
    if(new TextEncoder().encode(source).length>100*1024){setError("单个组件最大 100 KB");return;}
    const item:ShowcaseComponent={id:crypto.randomUUID(),title,format,source,height:280,placement:"after-about",enabled:true};
    onChange([...components,item]);setSelected(item.id);setPreview(null);setError("");
  };
  async function upload(file:File){
    if(file.size>100*1024){setError("单个组件最大 100 KB");return;}
    const format=file.name.toLowerCase().endsWith('.svg')?'svg':/\.html?$/i.test(file.name)?'html':null;
    if(!format){setError("请选择 SVG 或 HTML 文件");return;}
    try{add(format,await file.text(),file.name.replace(/\.[^.]+$/,"").slice(0,80));}catch{setError("读取文件失败");}
  }
  return <div className="showcase-component-editor">
    <div className="showcase-heading"><h2>创作组件</h2><p>导入自己制作的 SVG 插画或 HTML / Canvas 互动作品，也可以从示例开始修改。</p></div>
    <div className="showcase-component-actions"><button type="button" disabled={disabled||components.length>=6} onClick={()=>fileInput.current?.click()}>导入文件</button><button type="button" disabled={disabled||components.length>=6} onClick={()=>add('svg',SVG_EXAMPLE,'我的小星球')}>SVG 示例</button><button type="button" disabled={disabled||components.length>=6} onClick={()=>add('html',HTML_EXAMPLE,'点击种花')}>Canvas 示例</button></div>
    <input type="file" hidden ref={fileInput} accept=".svg,.html,.htm" onChange={e=>{const file=e.target.files?.[0];e.target.value='';if(file)void upload(file);}}/>
    {error&&<p role="alert" className="showcase-image-error">{error}</p>}
    <div className="showcase-component-list">{components.map(item=><button type="button" key={item.id} aria-pressed={current?.id===item.id} onClick={()=>{setSelected(item.id);setPreview(null);}}>{item.title||'未命名组件'}{!item.enabled?' · 已隐藏':''}</button>)}</div>
    {current&&<>
      <label className="showcase-field">组件名称<input value={current.title} maxLength={80} onChange={e=>patch({title:e.target.value})}/></label>
      <label className="showcase-field">源码<textarea className="showcase-code-input" value={current.source} spellCheck={false} onChange={e=>{if(new TextEncoder().encode(e.target.value).length>100*1024){setError('单个组件最大 100 KB');return;}setError('');patch({source:e.target.value});}} rows={10}/></label>
      <label className="showcase-field">展示高度<select value={current.height} onChange={e=>patch({height:Number(e.target.value)})}>{[160,240,280,360,480,640,800].map(h=><option key={h} value={h}>{h}px</option>)}</select></label>
      <label className="showcase-field">展示位置<select value={current.placement} onChange={e=>patch({placement:e.target.value as ShowcaseComponent['placement']})}><option value="after-about">个人介绍之后</option><option value="after-works">作品之后</option></select></label>
      <label className="showcase-component-toggle"><input type="checkbox" checked={current.enabled} onChange={e=>patch({enabled:e.target.checked})}/>在主页展示</label>
      <div className="showcase-component-actions"><button type="button" onClick={()=>setPreview({...current})}>运行预览</button><button type="button" disabled={!preview} onClick={()=>setPreview(null)}>停止预览</button><button type="button" disabled={components.indexOf(current)===0} onClick={()=>{const next=[...components];const index=next.indexOf(current);[next[index-1],next[index]]=[next[index]!,next[index-1]!];onChange(next);}}>上移</button><button type="button" onClick={()=>{onChange(components.filter(item=>item.id!==current.id));setPreview(null);}}>移除</button></div>
      {preview&&<div className="showcase-component-preview"><ShowcaseComponentPreview component={preview}/></div>}
    </>}
    <p className="showcase-mode-help">每个组件最大 100 KB，最多 6 个。HTML 组件请将样式和脚本写在文件内，图片使用内嵌数据；组件不能访问平台账号与数据。保存草稿后仅自己可见，发布时分享已启用组件。</p>
  </div>;
}
