import { useEffect, useRef, useState, type ReactNode } from "react";
import { ArrowLeft, ArrowRight, Send, Sparkles } from "lucide-react";
import { apiErrorText } from "../api/errorText";
import { chatShowcaseGuide, type ShowcaseGuideDestination, type ShowcaseGuideProposal, type ShowcaseGuideStage } from "../api/showcase";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import type { ShowcaseConfig, ShowcaseAboutMessage } from "../site/showcaseTypes";
import type { ShowcaseInterestSnapshot } from "../site/ShowcaseInterestTree";
import type { ShowcaseBuildStep, ShowcaseGuidePhase } from "./showcaseGuideFlow";

export const GUIDE_STEPS: {id:ShowcaseBuildStep;title:string;message:string;next?:string}[] = [
  {id:"design",title:"风格",message:"先选你喜欢的风格。右侧会马上显示它的大致样子；你也可以继续调整版式、色调和字体。",next:"看开场"},
  {id:"hero",title:"开场",message:"接下来设计主页的第一屏。你可以修改欢迎文字，选择系统配图，或者描述一张想生成的图片。",next:"写介绍"},
  {id:"profile",title:"介绍",message:"现在介绍你自己。请填写愿意公开的名字、简介和关键词，再决定要不要展示兴趣树。",next:"选作品"},
  {id:"works",title:"作品",message:"请选择要展示的写作、阅读和项目成果。你还可以指定一两件重点作品，或用清晰的列表呈现。",next:"加元素"},
  {id:"components",title:"元素",message:"如果想增加自己的图形或互动效果，可以描述 SVG 插画或 Canvas 小组件。我会生成预览，加入主页由你决定。",next:"检查主页"},
  {id:"finish",title:"完成",message:"请检查右侧的文字、图片、介绍和作品。确认初版后，仍然可以随时回来修改。"},
];

const revisionChoices: {label:string;destination:ShowcaseGuideDestination;reply:string}[] = [
  {label:"调整整体风格",destination:"design",reply:"先看看整体风格。你想调整哪种感觉？"},
  {label:"修改开场",destination:"hero-text",reply:"我们来修改主页的开场文字。"},
  {label:"更换首图",destination:"hero-image",reply:"我们来挑选或生成新的首图。"},
  {label:"修改个人介绍",destination:"profile-content",reply:"我们来修改个人介绍。"},
  {label:"管理作品",destination:"works",reply:"我们来检查主页展示的作品。"},
  {label:"加入新元素",destination:"components",reply:"请描述你想加入的图形或互动效果。"},
];

export function ShowcaseDesignGuide({phase,stage,draft,interestTree,disabled,available,onBusy,onConversation,onApply,onStart,onStep,onNavigate,onComplete,onComponentIdea,children}:{
  phase:ShowcaseGuidePhase;stage:ShowcaseGuideStage;draft:ShowcaseConfig;interestTree?:ShowcaseInterestSnapshot;
  disabled:boolean;available:boolean;onBusy:(busy:boolean)=>void;
  onConversation:(messages:ShowcaseAboutMessage[])=>void;onApply:(proposal:ShowcaseGuideProposal)=>void;
  onStart:()=>void;onStep:(stage:ShowcaseGuideStage)=>void;onNavigate:(destination:ShowcaseGuideDestination)=>void;onComplete:()=>void;
  onComponentIdea:(prompt:string)=>void;children:ReactNode;
}) {
  const [input,setInput]=useState("");
  const [pending,setPending]=useState(false);
  const [error,setError]=useState("");
  const [proposal,setProposal]=useState<ShowcaseGuideProposal|null>(null);
  const lock=useRef(false);
  const conversation=useRef<HTMLDivElement>(null);
  const shownStage=useRef<ShowcaseGuideStage|null>(null);
  const messages=draft.guideConversation??[];
  const index=GUIDE_STEPS.findIndex(step=>step.id===stage);
  const step=index>=0?GUIDE_STEPS[index]:undefined;
  useEffect(()=>{setProposal(null);setError("");setInput("");},[stage,phase]);
  useEffect(()=>{
    const el=conversation.current;
    if(!el)return;
    if(shownStage.current!==stage){shownStage.current=stage;el.scrollTop=0;return;}
    const last=el.querySelector<HTMLElement>(".showcase-guide-message:last-of-type");
    if(last)el.scrollTop=last.offsetTop-el.offsetTop-12;
  },[messages.length,stage,pending]);

  async function send(value=input){
    const content=value.trim();
    if(!content||disabled||(!available&&stage!=="components")||lock.current||phase==="welcome")return;
    if(messages.length>=58){setError("对话已达到长度上限。请保存当前成果，之后可以继续修改主页。");return;}
    const next=[...messages,{role:"user" as const,content:`【${step?.title??"修改需求"}】${content}`}];
    if(stage==="components"){
      onConversation([...next,{role:"assistant",content:"我会根据你的描述生成组件。请在下方预览，再决定是否加入主页。"}]);
      onComponentIdea(content);setInput("");setError("");return;
    }
    lock.current=true;setPending(true);onBusy(true);setError("");
    try{
      const result=await chatShowcaseGuide(stage,next,draft);
      onConversation([...next,{role:"assistant",content:result.reply}]);
      setProposal(result.proposal??null);setInput("");
      if(stage==="revise"&&result.navigateTo)onNavigate(result.navigateTo);
    }catch(err){setError(`讨论失败：${apiErrorText(err)}`);}
    finally{lock.current=false;setPending(false);onBusy(false);}
  }
  function chooseRevision(choice:(typeof revisionChoices)[number]){
    onConversation([...messages,{role:"user",content:`【修改需求】${choice.label}`},{role:"assistant",content:choice.reply}]);
    onNavigate(choice.destination);
  }

  if(phase==="welcome")return <section className="showcase-guide showcase-guide-welcome" aria-label="主页设计向导">
    <div className="showcase-guide-heading"><span><Sparkles size={15}/>印记 · 主页设计向导</span></div>
    <div className="showcase-guide-welcome-body"><div className="showcase-guide-welcome-art" aria-hidden="true"><span/><i/><b/></div><p className="showcase-guide-kicker">个人作品空间</p><h2>我们来搭建一个属于你的个人作品空间吧。</h2><p>你可以和我一起决定它的风格、开场、介绍和作品展示。每做一步，右侧都会显示变化。</p><strong>准备好了吗？</strong><button type="button" disabled={disabled} onClick={onStart}>开始制作 <ArrowRight size={17}/></button></div>
  </section>;

  return <section className="showcase-guide" aria-label="主页设计向导">
    <div className="showcase-guide-heading"><span><Sparkles size={15}/>印记 · 主页设计向导</span><strong>{step?`${index+1} / ${GUIDE_STEPS.length} · ${step.title}`:"修改主页"}</strong></div>
    <div ref={conversation} className="showcase-guide-messages" aria-live="polite">
      <div className="showcase-guide-message is-opening"><b>印记</b><p>{step?.message??"你已经有一个主页版本。想修改哪里？可以直接告诉我，我会打开相应的内容。"}</p>{stage==="profile"&&!!interestTree?.keywords.length&&<p className="showcase-guide-tree">兴趣树里还有：{interestTree.keywords.slice(0,8).join("、")}。你想展示哪些？</p>}{stage==="profile"&&<p>你也可以上传照片或生成头像。</p>}{stage==="works"&&<p>报告的公开状态仍在各自的报告页管理。</p>}</div>
      {stage==="revise"&&<div className="showcase-guide-revision-choices" aria-label="常见修改">{revisionChoices.map(choice=><button key={choice.destination} type="button" disabled={disabled} onClick={()=>chooseRevision(choice)}>{choice.label}<ArrowRight size={13}/></button>)}</div>}
      {messages.map((message,i)=><div className={`showcase-guide-message ${message.role==="user"?"is-student":""}`} key={`${i}-${message.role}`}><b>{message.role==="user"?"我":"印记"}</b><p>{message.content}</p></div>)}
      {pending&&<p className="showcase-guide-pending">印记正在整理建议…</p>}
      {proposal&&<div className="showcase-guide-proposal"><p>{proposal.reason}</p><button type="button" disabled={disabled} onClick={()=>{onApply(proposal);setProposal(null);}}>应用建议到预览</button></div>}
      {step&&<div className="showcase-guide-tools">{children}</div>}
    </div>
    <div className="showcase-guide-composer">
      <GrowingTextarea value={input} rows={2} maxLength={stage==="components"?1000:2000} disabled={disabled||(!available&&stage!=="components")} onChange={e=>setInput(e.target.value)} placeholder={stage==="components"?"例如：点击后会出现星星的夜空":"告诉印记你的想法或修改要求…"}/>
      <button type="button" aria-label={stage==="components"?"发送并生成组件":"发送给印记"} disabled={disabled||(!available&&stage!=="components")||!input.trim()} onClick={()=>void send()}><Send size={16}/></button>
    </div>
    {!available&&stage!=="components"&&<p className="showcase-guide-error">设计对话暂不可用，仍可使用制作工具。</p>}
    {error&&<p className="showcase-guide-error" role="alert">{error}</p>}
    <div className="showcase-guide-step-actions">{step ? <><button type="button" disabled={index===0||disabled} onClick={()=>onStep(GUIDE_STEPS[index-1]!.id)}><ArrowLeft size={14}/>上一步</button>{phase==="build"&&stage==="finish"?<button type="button" disabled={disabled} onClick={onComplete}>完成初版<ArrowRight size={14}/></button>:step.next&&phase==="build"?<button type="button" disabled={disabled} onClick={()=>onStep(GUIDE_STEPS[index+1]!.id)}>{step.next}<ArrowRight size={14}/></button>:<button type="button" disabled={disabled} onClick={()=>onStep("revise")}>继续修改其他部分<ArrowRight size={14}/></button>}</> : null}</div>
  </section>;
}
