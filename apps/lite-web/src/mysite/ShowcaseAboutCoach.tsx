import { useRef, useState } from "react";
import { Send, Sparkles } from "lucide-react";
import { apiErrorText } from "../api/errorText";
import { chatShowcaseAbout, type ShowcaseAboutProposal } from "../api/showcase";
import type { ShowcaseConfig, ShowcaseAboutMessage } from "../site/showcaseTypes";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import { Says, errorMarkdown } from "../projects/Says";

export function ShowcaseAboutCoach({draft, available, disabled, onBusy, onConversation, onApply}: {
  draft: ShowcaseConfig;
  available: boolean;
  disabled: boolean;
  onBusy: (value:boolean) => void;
  onConversation: (messages:ShowcaseAboutMessage[]) => void;
  onApply: (proposal:ShowcaseAboutProposal) => void;
}) {
  const [input,setInput] = useState("");
  const [pending,setPending] = useState(false);
  const [error,setError] = useState("");
  const [proposal,setProposal] = useState<ShowcaseAboutProposal|null>(null);
  const lock = useRef(false);
  const messages = draft.aboutConversation ?? [];
  async function send() {
    if (!input.trim() || disabled || lock.current) return;
    const next = [...messages,{role:"user" as const,content:input.trim()}];
    if (next.length > 19 || next.reduce((total,message)=>total+Array.from(message.content).length,0)>18000) {setError("本轮对话已达到长度上限。请先应用或记录需要的内容，再开始新对话。");return;}
    lock.current = true; setPending(true); onBusy(true); setError("");
    try {
      const response = await chatShowcaseAbout(next, {name:draft.name,bio:draft.bio,interests:draft.interests,aboutLayout:draft.aboutLayout ?? "classic"});
      onConversation([...next,{role:"assistant",content:response.reply}]);
      setProposal(response.proposal ?? null); setInput("");
    } catch(err) {setError(`讨论失败：${apiErrorText(err)}`);}
    finally {lock.current=false;setPending(false);onBusy(false);}
  }
  if (!available) return <p className="showcase-editor-empty">介绍对话尚未开放，可以在下方编辑介绍与版式。</p>;
  return <section className="showcase-about-coach" aria-label="讨论个人介绍">
    <div className="showcase-coach-conversation" aria-live="polite">
      {messages.length === 0 && <div className="showcase-coach-message"><span>印记</span><p>你希望来到主页的人，先了解你的哪一面？可以从兴趣爱好、正在做的事，或一段自己的故事开始。</p></div>}
      {messages.map((message,index)=><div key={index} className={`showcase-coach-message ${message.role === "user" ? "is-student" : ""}`}><span>{message.role === "user" ? "我" : "印记"}</span><Says content={message.content}/></div>)}
      {pending && <p className="showcase-coach-pending">印记正在整理…</p>}
    </div>
    {messages.length === 0 && <div className="showcase-coach-starters">{["我的兴趣爱好","我正在做的事","我的一个故事"].map(label=><button type="button" key={label} disabled={disabled} onClick={()=>setInput(label+"：")}>{label}</button>)}</div>}
    <label className="showcase-field">与印记讨论<GrowingTextarea value={input} disabled={disabled} rows={3} maxLength={2000} onChange={e=>setInput(e.target.value)} placeholder="请写下想让访客了解的内容。"/></label>
    <button type="button" className="showcase-coach-send" disabled={disabled || !input.trim()} onClick={()=>void send()}><Send size={14}/>{pending ? "整理中" : "发送"}</button>
    {error && <div className="showcase-image-error" role="alert"><Says content={errorMarkdown(error)}/></div>}
    {proposal && <div className="showcase-about-proposal"><h3><Sparkles size={15}/>介绍建议</h3><strong>{proposal.name}</strong><p>{proposal.bio}</p><div className="showcase-proposal-keywords">{proposal.interests.map(word=><span key={word}>{word}</span>)}</div><p><b>建议版式：</b>{proposal.aboutLayout === "orbit" ? "照片与关键词" : "名字与简介"}</p><p>{proposal.reason}</p><button type="button" disabled={disabled} onClick={()=>onApply(proposal)}>应用到预览</button></div>}
    <p className="showcase-mode-help">发送时，当前介绍与这段对话会交给 AI 整理。对话随草稿保存，不随主页公开；建议应用后仍可修改。</p>
    {messages.length>0 && <button type="button" className="showcase-coach-reset" disabled={disabled} onClick={()=>{onConversation([]);setProposal(null);setError("");}}>清空对话</button>}
  </section>;
}
