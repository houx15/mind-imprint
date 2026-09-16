import { Says, errorMarkdown } from "../../Says";
import type { SiteRef } from "../../../api/siteRefs";
import type { HeroBrief } from "../../../api/creativeDirection";
import { GrowingTextarea } from "../../../shared/GrowingTextarea";
const input = "w-full rounded-mk-lg border border-mk-border bg-mk-surface p-4 text-mk-body";
const chip = "rounded-mk-lg border border-mk-border px-4 py-3 text-left text-mk-body aria-pressed:border-mk-accent-500 aria-pressed:bg-mk-accent-50";
export function HeroBriefCard({ hero, onChange, onRefine, busy, references = [], referenceError = "" }: { references?: SiteRef[]; referenceError?: string; hero: HeroBrief; onChange: (patch: Partial<HeroBrief>) => void; onRefine: () => void; busy: boolean }) {
 return <div className="space-y-5">
  <h3 className="text-mk-h2">构思主页的第一幕</h3>
  <div className="rounded-mk-lg bg-mk-accent-50 p-4 text-mk-body leading-relaxed"><strong>Hero 是打开主页最先看到的区域。</strong><p className="mt-2">它可以是一张画、一个场景，或可以互动的画面，帮助访客了解你是谁、这里有什么，以及接下来可以做什么。</p></div>
  {(references.length > 0 || referenceError) && <details className="rounded-mk-lg border border-mk-border p-4">
    <summary className="cursor-pointer text-mk-body font-semibold">参考自己的灵感记录 · {references.length}</summary>
    <p className="my-3 text-mk-small text-mk-secondary">请想一想：准备借鉴哪种做法，为自己的读者改变什么？请把决定写进下面的画面或互动描述。收藏的参考不会自动加入制作要求。</p>
    {referenceError && <div role="alert" className="text-mk-small text-mk-danger"><Says content={errorMarkdown(`灵感记录读取失败：${referenceError}`)} /></div>}
    <div className="space-y-3">{references.map(reference => <section key={reference.id} className="rounded-mk-md bg-mk-surface p-3">
      <a href={reference.url} target="_blank" rel="noreferrer" className="block break-all text-mk-small underline">{reference.title || reference.url}</a>
      <p className="mt-2 whitespace-pre-wrap text-mk-small">{reference.sheSaid || "尚未记录观察或设计取舍。请实际体验后再判断，也可以继续自己的构思。"}</p>
    </section>)}</div>
  </details>}
  <label className="block text-mk-body font-semibold">第一眼看到的画面<GrowingTextarea aria-label="第一幕画面" value={hero.scene} maxLength={2000} onChange={e=>onChange({scene:e.target.value,prompt:""})} placeholder="请描述画面中的事物、位置和变化。" className={`${input} mt-2 font-normal`}/></label>
  <fieldset className="space-y-2"><legend className="mb-2 text-mk-body font-semibold">呈现方式</legend><div className="grid gap-2 sm:grid-cols-3">{([{mode:"image",title:"一张图片",hint:"先构思一幅完整画面"},{mode:"code",title:"代码效果",hint:"用动画或互动呈现"},{mode:"mixed",title:"图片与代码",hint:"在图片上加入互动"}] as const).map(option=><button key={option.mode} type="button" className={chip} aria-pressed={hero.mode===option.mode} onClick={()=>onChange({mode:option.mode,prompt:""})}><strong>{option.title}</strong><span className="mt-1 block text-mk-small text-mk-secondary">{option.hint}</span></button>)}</div></fieldset>
  {hero.mode!=="image"&&<label className="block text-mk-body font-semibold">访客可以做什么<span className="my-2 block text-mk-small font-normal text-mk-secondary">请描述鼠标移动、点击或触摸后发生什么。也可以只展示画面，不加操作。</span><GrowingTextarea aria-label="第一幕互动" value={hero.action} maxLength={1000} onChange={e=>onChange({action:e.target.value,prompt:""})} placeholder="例如：点击一颗星星，出现一件作品。" className={`${input} font-normal`}/></label>}
  <div className="border-t border-mk-border pt-5"><label className="block text-mk-body font-semibold">生成提示词<span className="my-2 block text-mk-small font-normal text-mk-secondary">提示词是交给 AI 的制作说明。请写下或完善画面、动作和呈现方式，也可以请 AI 帮你整理上面的想法。</span><GrowingTextarea aria-label="Hero生成提示词" value={hero.prompt} maxLength={5000} onChange={e=>onChange({prompt:e.target.value})} className={`${input} font-normal`} /></label><button type="button" disabled={busy||!hero.scene.trim()||!hero.mode} className={`${chip} mt-3 disabled:opacity-40`} onClick={onRefine}>{busy?"正在整理…":"请 AI 整理提示词"}</button><p className="mt-3 text-mk-small text-mk-muted">这里保存的是制作说明。请核对提示词是否准确表达你的想法，生成后再试用实际效果。</p></div>
 </div>;
}
