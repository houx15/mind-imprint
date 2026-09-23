import { useEffect, useRef, useState, type ReactNode } from "react";
import { ArrowLeft, ArrowRight, Send, Sparkles } from "lucide-react";
import { apiErrorText } from "../api/errorText";
import { chatShowcaseGuide, type ShowcaseGuideProposal, type ShowcaseGuideStage } from "../api/showcase";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import type { ShowcaseConfig, ShowcaseAboutMessage } from "../site/showcaseTypes";
import type { ShowcaseInterestSnapshot } from "../site/ShowcaseInterestTree";

export const GUIDE_STEPS: {id:ShowcaseGuideStage;title:string;message:string;next?:string}[] = [
  {id:"design",title:"风格",message:"先选你喜欢的风格、版式和色调。你也可以告诉我想让访客感受到什么，我会给出具体建议。",next:"看开场"},
  {id:"hero",title:"开场",message:"现在看主页的第一屏。刚选的风格有对应配图；如果你有更喜欢的画面，可以描述给我，再生成一张。标题和介绍文字也可以修改。",next:"写介绍"},
  {id:"profile",title:"介绍",message:"主页可以用名字与简介介绍你，也可以让照片和关键词围绕在一起。请先填写你愿意公开的基本信息，我们再决定怎么展示。",next:"选作品"},
  {id:"works",title:"作品",message:"你可以选择哪些阅读、写作和项目成果出现在主页，也可以决定作品的展示方式。作品管理有独立页面。",next:"加元素"},
  {id:"components",title:"元素",message:"你还可以加入自己想要的元素。描述一张 SVG 插画或一个 Canvas 互动效果，我来生成；先预览，再决定是否加入主页。",next:"检查主页"},
  {id:"finish",title:"完成",message:"这就是你目前的个人主页。请检查右侧预览，保存草稿。确认要给别人看时，再发布并复制链接。"},
];

export function ShowcaseDesignGuide({stage,draft,interestTree,disabled,available,onBusy,onConversation,onApply,onStep,onComponentIdea,children}:{
  stage:ShowcaseGuideStage;draft:ShowcaseConfig;interestTree?:ShowcaseInterestSnapshot;
  disabled:boolean;available:boolean;onBusy:(busy:boolean)=>void;
  onConversation:(messages:ShowcaseAboutMessage[])=>void;onApply:(proposal:ShowcaseGuideProposal)=>void;
  onStep:(stage:ShowcaseGuideStage)=>void;
  onComponentIdea:(prompt:string)=>void;
  children:ReactNode;
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
  const step=GUIDE_STEPS[index]!;
  const recent=messages.slice(-4);
  useEffect(()=>{setProposal(null);setError("");},[stage]);
  useEffect(()=>{
    const el=conversation.current;
    if(!el)return;
    if(shownStage.current!==stage){shownStage.current=stage;el.scrollTop=0;return;}
    if(messages.length>0){
      const last=el.querySelector<HTMLElement>(":scope > .showcase-guide-message:last-of-type");
      if(last)el.scrollTop=last.offsetTop-el.offsetTop-12;
    }
  },[messages.length,stage,pending]);

  async function send(){
    const value=input.trim();
    if(!value||disabled||(!available&&stage!=="components")||lock.current)return;
    if(messages.length>=58){setError("对话已达到长度上限。请保存当前成果，之后可以继续修改主页。");return;}
    const next=[...messages,{role:"user" as const,content:`【${step.title}】${value}`}];
    if(stage==="components"){
      onConversation([...next,{role:"assistant",content:"我会根据你的描述生成组件。请在下方预览，再决定是否加入主页。"}]);
      onComponentIdea(value);setInput("");setError("");return;
    }
    lock.current=true;setPending(true);onBusy(true);setError("");
    try{
      const result=await chatShowcaseGuide(stage,next,draft);
      onConversation([...next,{role:"assistant",content:result.reply}]);
      setProposal(result.proposal??null);setInput("");
    }catch(err){setError(`讨论失败：${apiErrorText(err)}`);}
    finally{lock.current=false;setPending(false);onBusy(false);}
  }
  return <section className="showcase-guide" aria-label="主页设计向导">
    <div className="showcase-guide-heading"><span><Sparkles size={15}/>印记 · 主页设计向导</span><strong>{index+1} / {GUIDE_STEPS.length} · {step.title}</strong></div>
    <div ref={conversation} className="showcase-guide-messages" aria-live="polite">
      <div className="showcase-guide-message"><b>印记</b><p>{step.message}</p>{stage==="profile"&&!!interestTree?.keywords.length&&<p className="showcase-guide-tree">你的兴趣树有：{interestTree.keywords.slice(0,8).join("、")}。你想在主页展示其中哪些？</p>}{stage==="profile"&&<p>介绍完成后，也可以上传照片或生成头像。</p>}{stage==="works"&&<p>报告的公开状态仍在各自的报告页管理。</p>}</div>
      {recent.map((message,i)=><div className={`showcase-guide-message ${message.role==="user"?"is-student":""}`} key={`${messages.length-recent.length+i}-${message.role}`}><b>{message.role==="user"?"我":"印记"}</b><p>{message.content}</p></div>)}
      {pending&&<p className="showcase-guide-pending">印记正在整理建议…</p>}
      {proposal&&<div className="showcase-guide-proposal"><p>{proposal.reason}</p><button type="button" disabled={disabled} onClick={()=>{onApply(proposal);setProposal(null);}}>应用建议到预览</button></div>}
      <div className="showcase-guide-tools">{children}</div>
    </div>
    <div className="showcase-guide-composer">
      <GrowingTextarea value={input} rows={2} maxLength={stage==="components"?1000:2000} disabled={disabled||(!available&&stage!=="components")} onChange={e=>setInput(e.target.value)} placeholder={stage==="components"?"例如：一片点击后会长出星星的夜空。":"告诉印记你想修改什么…"}/>
      <button type="button" aria-label={stage==="components"?"发送并生成组件":"发送给印记"} disabled={disabled||(!available&&stage!=="components")||!input.trim()} onClick={()=>void send()}><Send size={16}/></button>
    </div>
    {!available&&stage!=="components"&&<p className="showcase-guide-error">设计对话暂不可用，仍可使用下面的制作工具。</p>}
    {error&&<p className="showcase-guide-error" role="alert">{error}</p>}
    <div className="showcase-guide-step-actions"><button type="button" disabled={index===0||disabled} onClick={()=>onStep(GUIDE_STEPS[index-1]!.id)}><ArrowLeft size={14}/>上一步</button>{step.next&&<button type="button" disabled={disabled} onClick={()=>onStep(GUIDE_STEPS[index+1]!.id)}>{step.next}<ArrowRight size={14}/></button>}</div>
  </section>;
}
