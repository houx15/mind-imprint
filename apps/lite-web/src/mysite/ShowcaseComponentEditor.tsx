import { useEffect, useRef, useState } from "react";
import { Loader2, Sparkles } from "lucide-react";
import { apiErrorText } from "../api/errorText";
import { generateShowcaseComponent, type GeneratedShowcaseComponent } from "../api/showcase";
import type { ShowcaseComponent } from "../site/showcaseTypes";
import type { ShowcaseConfig } from "../site/showcaseTypes";
import { ShowcaseComponentPreview } from "../site/ShowcaseComponents";
import { GrowingTextarea } from "../shared/GrowingTextarea";

const SVG_EXAMPLE = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 280"><rect width="600" height="280" rx="24" fill="#102b3f"/><circle cx="300" cy="135" r="66" fill="#98d9cd"/><ellipse cx="300" cy="135" rx="125" ry="22" fill="none" stroke="#f4cc7c" stroke-width="8" transform="rotate(-20 300 135)"/><text x="300" y="248" text-anchor="middle" fill="white" font-size="18">我的小星球</text></svg>`;
const HTML_EXAMPLE = `<canvas id="art" width="600" height="280" aria-label="点击画布种一朵花"></canvas><script>
const canvas = document.getElementById('art');
const ctx = canvas.getContext('2d');
ctx.fillStyle='#f4f1e6';ctx.fillRect(0,0,600,280);
ctx.fillStyle='#38584c';ctx.font='18px sans-serif';ctx.fillText('点击画布，种一朵花',24,36);
canvas.addEventListener('click',event=>{const r=canvas.getBoundingClientRect();const x=(event.clientX-r.left)*600/r.width,y=(event.clientY-r.top)*280/r.height;ctx.strokeStyle='#579475';ctx.beginPath();ctx.moveTo(x,y);ctx.lineTo(x,y+45);ctx.stroke();for(let i=0;i<6;i++){ctx.fillStyle='#e6a0aa';ctx.beginPath();ctx.arc(x+Math.cos(i*Math.PI/3)*10,y+Math.sin(i*Math.PI/3)*10,8,0,Math.PI*2);ctx.fill();}ctx.fillStyle='#eecb6e';ctx.beginPath();ctx.arc(x,y,6,0,Math.PI*2);ctx.fill();});
</script>`;
export function ShowcaseComponentEditor({components,onChange,disabled,onBusy,prompt,onPromptChange,style,palette,generationRequest,onRequestConsumed}:{components:ShowcaseComponent[];onChange:(items:ShowcaseComponent[])=>void;disabled:boolean;onBusy:(busy:boolean)=>void;prompt:string;onPromptChange:(prompt:string)=>void;style:ShowcaseConfig["style"];palette:ShowcaseConfig["palette"];generationRequest?:{id:number;prompt:string}|null;onRequestConsumed:()=>void}) {
  const [selected,setSelected]=useState<string>();
  const [error,setError]=useState("");
  const [preview,setPreview]=useState<ShowcaseComponent|null>(null);
  const [format,setFormat]=useState<"svg"|"html">("svg");
  const [candidate,setCandidate]=useState<GeneratedShowcaseComponent|null>(null);
  const [candidatePreview,setCandidatePreview]=useState(false);
  const [generating,setGenerating]=useState(false);
  const lock=useRef(false);
  const fileInput=useRef<HTMLInputElement>(null);
  const current=components.find(item=>item.id===selected)??components[0];
  const patch=(change:Partial<ShowcaseComponent>)=>{if(current)onChange(components.map(item=>item.id===current.id?{...item,...change}:item));};
  const add=(format:"svg"|"html",source:string,title:string,height=280,placement:ShowcaseComponent["placement"]="after-about")=>{
    if(components.length>=6){setError("最多添加 6 个组件");return;}
    if(new TextEncoder().encode(source).length>100*1024){setError("单个组件最大 100 KB");return;}
    const item:ShowcaseComponent={id:crypto.randomUUID(),title,format,source,height,placement,enabled:true};
    onChange([...components,item]);setSelected(item.id);setPreview(null);setError("");
  };
  async function upload(file:File){
    if(file.size>100*1024){setError("单个组件最大 100 KB");return;}
    const format=file.name.toLowerCase().endsWith('.svg')?'svg':/\.html?$/i.test(file.name)?'html':null;
    if(!format){setError("请选择 SVG 或 HTML 文件");return;}
    try{add(format,await file.text(),file.name.replace(/\.[^.]+$/,"").slice(0,80));}catch{setError("读取文件失败");}
  }
  async function generate(requestedPrompt=prompt,fromGuide=false){
    if(lock.current)return;
    if(components.length>=6){setError("最多添加 6 个组件。请先移除一个，再生成新组件。");return;}
    if(requestedPrompt.trim().length<4){setError("请描述想要的组件效果。");return;}
    const requestedFormat=fromGuide&&/canvas|画布/i.test(requestedPrompt)?"html":fromGuide&&/svg/i.test(requestedPrompt)?"svg":format;
    if(fromGuide)setFormat(requestedFormat);
    lock.current=true;setGenerating(true);onBusy(true);setError("");setCandidate(null);setCandidatePreview(false);
    try{setCandidate(await generateShowcaseComponent(requestedPrompt.trim(),requestedFormat,style,palette));}
    catch(err){setError(`组件生成失败：${apiErrorText(err)}`);}
    finally{lock.current=false;setGenerating(false);onBusy(false);}
  }
  useEffect(()=>{if(generationRequest){void generate(generationRequest.prompt,true);onRequestConsumed();}},[generationRequest?.id]);
  return <div className="showcase-component-editor">
    <div className="showcase-heading"><h2>描述你想要的元素</h2><p>印记可以生成 SVG 插画或 Canvas 互动组件。生成后先预览，再加入主页草稿。</p></div>
    <div className="showcase-mode-options"><button type="button" aria-pressed={format==="svg"} onClick={()=>{setFormat("svg");setCandidate(null);}}>SVG 插画</button><button type="button" aria-pressed={format==="html"} onClick={()=>{setFormat("html");setCandidate(null);}}>Canvas 互动</button></div>
    <label className="showcase-field mt-4">元素描述<GrowingTextarea value={prompt} onChange={e=>onPromptChange(e.target.value)} maxLength={1000} rows={3} placeholder={format==="svg"?"例如：一颗会轻轻闪烁的蓝色星球，和主页的海岸色调呼应。":"例如：点击画面时长出一朵花，颜色随点击位置变化。"}/></label>
    <button type="button" className="showcase-generate" disabled={disabled||generating||components.length>=6||prompt.trim().length<4} onClick={()=>void generate()}>{generating?<Loader2 size={15} className="animate-spin"/>:<Sparkles size={15}/>} {generating?"正在生成组件":"生成组件"}</button>
    {candidate&&<div className="showcase-component-candidate"><h3>{candidate.title}</h3><p>{candidate.explanation}</p><div className="showcase-component-actions"><button type="button" onClick={()=>setCandidatePreview(true)}>预览组件</button><button type="button" disabled={disabled||!candidatePreview} onClick={()=>{add(format,candidate.source,candidate.title,candidate.height,candidate.placement);setCandidate(null);setCandidatePreview(false);}}>加入主页草稿</button></div>{candidatePreview&&<ShowcaseComponentPreview component={{id:"candidate",title:candidate.title,format,source:candidate.source,height:candidate.height,placement:candidate.placement,enabled:true}}/>}</div>}
    <details className="showcase-component-manual"><summary>导入已有组件或查看示例</summary><div className="showcase-component-actions"><button type="button" disabled={disabled||components.length>=6} onClick={()=>fileInput.current?.click()}>导入文件</button><button type="button" disabled={disabled||components.length>=6} onClick={()=>add('svg',SVG_EXAMPLE,'我的小星球')}>SVG 示例</button><button type="button" disabled={disabled||components.length>=6} onClick={()=>add('html',HTML_EXAMPLE,'点击种花')}>Canvas 示例</button></div></details>
    <input type="file" hidden ref={fileInput} accept=".svg,.html,.htm" onChange={e=>{const file=e.target.files?.[0];e.target.value='';if(file)void upload(file);}}/>
    {error&&<p role="alert" className="showcase-image-error">{error}</p>}
    <div className="showcase-component-list">{components.map(item=><button type="button" key={item.id} aria-pressed={current?.id===item.id} onClick={()=>{setSelected(item.id);setPreview(null);}}>{item.title||'未命名组件'}{!item.enabled?' · 已隐藏':''}</button>)}</div>
    {current&&<>
      <label className="showcase-field">组件名称<input value={current.title} maxLength={80} onChange={e=>patch({title:e.target.value})}/></label>
      <details className="showcase-component-manual"><summary>调整组件代码</summary><label className="showcase-field">源码<textarea className="showcase-code-input" value={current.source} spellCheck={false} onChange={e=>{if(new TextEncoder().encode(e.target.value).length>100*1024){setError('单个组件最大 100 KB');return;}setError('');patch({source:e.target.value});}} rows={10}/></label></details>
      <label className="showcase-field">展示高度<select value={current.height} onChange={e=>patch({height:Number(e.target.value)})}>{[160,240,280,360,480,640,800].map(h=><option key={h} value={h}>{h}px</option>)}</select></label>
      <label className="showcase-field">展示位置<select value={current.placement} onChange={e=>patch({placement:e.target.value as ShowcaseComponent['placement']})}><option value="after-about">个人介绍之后</option><option value="after-works">作品之后</option></select></label>
      <label className="showcase-component-toggle"><input type="checkbox" checked={current.enabled} onChange={e=>patch({enabled:e.target.checked})}/>在主页展示</label>
      <div className="showcase-component-actions"><button type="button" onClick={()=>setPreview({...current})}>运行预览</button><button type="button" disabled={!preview} onClick={()=>setPreview(null)}>停止预览</button><button type="button" disabled={components.indexOf(current)===0} onClick={()=>{const next=[...components];const index=next.indexOf(current);[next[index-1],next[index]]=[next[index]!,next[index-1]!];onChange(next);}}>上移</button><button type="button" onClick={()=>{onChange(components.filter(item=>item.id!==current.id));setPreview(null);}}>移除</button></div>
      {preview&&<div className="showcase-component-preview"><ShowcaseComponentPreview component={preview}/></div>}
    </>}
    <p className="showcase-mode-help">每个组件最大 100 KB，最多 6 个。HTML 组件请将样式和脚本写在文件内，图片使用内嵌数据；组件不能访问平台账号与数据。保存草稿后仅自己可见，发布时分享已启用组件。</p>
  </div>;
}
