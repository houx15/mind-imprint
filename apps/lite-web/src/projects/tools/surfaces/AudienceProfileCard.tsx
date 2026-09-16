import { useState } from "react";
import type { AudienceBoard } from "../../../api/audienceBoard";
import { AudiencePortrait } from "./AudiencePortrait";

const ages = ["12岁及以下", "13至15岁", "16至18岁", "19至30岁", "31至50岁", "51岁及以上", "不确定"];
const suggestions: Record<string, string[]> = {
  父母: ["我的成长", "学习成果", "作品集"],
  老师: ["学习成果", "作品集", "尝试与进步"],
  同学: ["酷炫的效果", "互动小游戏", "闯关式介绍", "共同兴趣"],
  陌生人: ["我是什么样的人", "我的故事", "我的独特之处"],
};
const offerings = ["作品照片", "制作过程", "学习经历", "活动记录", "参与方式"];

export function AudienceProfileCard({ board, index, total, onChange, onPrevious, onNext, onReview }: {
  board: AudienceBoard; index: number; total: number;
  onChange: (patch: Partial<AudienceBoard>) => void;
  onPrevious: () => void; onNext: () => void; onReview: () => void;
}) {
  const [touchStart, setTouchStart] = useState<{ x: number; y: number } | null>(null);
  const keywordProps = (field: "hobbies" | "interests" | "offerings") => ({
    input: board.keywordDrafts?.[field] ?? { text: "", editing: null },
    onUpdate: (values: string[], input: { text: string; editing: string | null }) => onChange({
      [field]: values, keywordDrafts: { ...board.keywordDrafts, [field]: input },
    }),
  });
  const ready = !Object.values(board.keywordDrafts ?? {}).some(draft => draft && (draft.text.trim() || draft.editing !== null)) && Boolean(board.person.trim() && board.ageRange && board.interests.length && board.offerings.length);
  return <section className="mx-auto w-full max-w-[760px]" aria-label={`${board.role}人物卡`}>
    <div className="mb-4 flex items-center justify-between text-mk-small text-mk-muted"><span>读者人物板</span><div className="flex items-center gap-3"><button type="button" aria-label="上一张人物卡" disabled={index === 0} onClick={onPrevious} className="rounded-full border border-mk-border px-3 py-1.5 disabled:opacity-30">←</button><span>{index + 1} / {total}</span><button type="button" aria-label="下一张人物卡" disabled={index === total - 1} onClick={onNext} className="rounded-full border border-mk-border px-3 py-1.5 disabled:opacity-30">→</button></div></div>
    <article className="overflow-hidden rounded-[24px] border border-mk-border bg-mk-surface shadow-sm"
      onTouchStart={event => {
        const touch = event.touches[0];
        setTouchStart(touch && !(event.target as HTMLElement).closest("input,textarea,select,button") ? { x: touch.clientX, y: touch.clientY } : null);
      }}
      onTouchEnd={event => {
        const end = event.changedTouches[0];
        if (touchStart && end && Math.abs(end.clientX - touchStart.x) > 70 && Math.abs(end.clientX - touchStart.x) > Math.abs(end.clientY - touchStart.y)) {
          if (end.clientX < touchStart.x && index < total - 1) onNext();
          else if (end.clientX > touchStart.x && index > 0) onPrevious();
        }
        setTouchStart(null);
      }}>
      <div className="grid grid-cols-[minmax(90px,0.65fr)_minmax(0,1.35fr)] items-start gap-4 bg-mk-accent-50 p-5 sm:gap-6 sm:p-7">
        <div className="flex flex-col items-center rounded-[20px] bg-white/70 p-3">
          <AudiencePortrait role={board.role} className="w-full max-w-[150px]" />
          <h3 className="mt-2 text-mk-h2 text-mk-ink">{board.role}</h3>
        </div>
        <div className="flex min-w-0 flex-col justify-center gap-4">
          <label className="text-mk-small font-semibold text-mk-secondary">具体人物
            <input value={board.person} maxLength={200} onChange={e => onChange({ person: e.target.value })} placeholder="例如：美术老师" className="mt-2 block w-full rounded-xl border border-mk-border bg-mk-surface px-3 py-2.5 text-mk-body font-normal text-mk-ink" />
          </label>
          <label className="text-mk-small font-semibold text-mk-secondary">大致年龄
            <select value={board.ageRange} onChange={e => onChange({ ageRange: e.target.value })} className="mt-2 block w-full rounded-xl border border-mk-border bg-mk-surface px-3 py-2.5 text-mk-body font-normal text-mk-ink"><option value="">请选择</option>{ages.map(age => <option key={age}>{age}</option>)}</select>
          </label>
          <KeywordSection title="兴趣爱好" hint="请填写这位读者平时喜欢的事物；不了解时可以留空。" placeholder="例如：动漫、科幻、篮球" values={board.hobbies ?? []} suggestions={[]} {...keywordProps("hobbies")} />
        </div>
      </div>
      <div className="space-y-6 border-t border-mk-border p-5 sm:p-7">
        <KeywordSection title="他可能对主页的哪些内容感兴趣" hint="以下是参考选项，请根据具体的人选择或补充。" values={board.interests} suggestions={suggestions[board.role] ?? ["我的作品", "我的故事", "互动小游戏"]} {...keywordProps("interests")} />
        <div className="border-t border-mk-border" />
        <KeywordSection title="我想向这个人展示什么" hint="这些内容会用于规划你的作品。" values={board.offerings} suggestions={offerings} {...keywordProps("offerings")} />
      </div>
    </article>
    <div className="mt-5 flex items-center justify-between gap-3">
      <button type="button" disabled={index === 0} onClick={onPrevious} className="rounded-full border border-mk-border px-4 py-2.5 text-mk-body disabled:opacity-30">← 上一张</button>
      <div className="flex gap-1.5" aria-hidden="true">{Array.from({ length: total }, (_, i) => <span key={i} className={`h-2 rounded-full ${i === index ? "w-6 bg-mk-accent-500" : "w-2 bg-mk-border"}`} />)}</div>
      {index < total - 1 ? <button type="button" onClick={onNext} className="rounded-full bg-mk-accent-500 px-4 py-2.5 text-mk-body font-semibold text-white">下一张 →</button> : <button type="button" disabled={!ready} onClick={onReview} className="rounded-full bg-mk-accent-500 px-4 py-2.5 text-mk-body font-semibold text-white disabled:opacity-40">查看人物板总结</button>}
    </div>
  </section>;
}

function KeywordSection({ title, hint, placeholder = "补充关键词", values, suggestions, input, onUpdate }: { title: string; hint: string; placeholder?: string; values: string[]; suggestions: string[]; input: { text: string; editing: string | null }; onUpdate: (values: string[], input: { text: string; editing: string | null }) => void }) {
  const { text: draft, editing } = input;
  const emptyInput = { text: "", editing: null };
  const save = () => {
    const word = draft.trim();
    if (!word || (values.length >= 12 && editing === null)) return;
    onUpdate(Array.from(new Set(editing === null ? [...values, word] : values.map(v => v === editing ? word : v))), emptyInput);
  };
  return <div>
    <h4 className="text-mk-body font-semibold text-mk-ink">{title}</h4>
    <p className="mt-1 text-mk-small text-mk-muted">{hint}</p>
    <div className="mt-3 flex flex-wrap gap-2">
      {values.map(value => <span key={value} className="inline-flex items-center overflow-hidden rounded-full border border-mk-accent-200 bg-mk-accent-50 text-mk-body text-mk-ink">
        <button type="button" aria-label={`修改关键词：${value}`} onClick={() => onUpdate(values, { text: value, editing: value })} className="px-3 py-2">{value}</button>
        <button type="button" aria-label={`删除关键词：${value}`} onClick={() => onUpdate(values.filter(v => v !== value), editing === value ? emptyInput : input)} className="border-l border-mk-accent-200 px-2.5 py-2 text-mk-muted">×</button>
      </span>)}
      {suggestions.filter(value => !values.includes(value)).map(value => <button type="button" key={value} disabled={values.length >= 12} onClick={() => onUpdate([...values, value], input)} className="rounded-full border border-dashed border-mk-border px-3 py-2 text-mk-small text-mk-secondary disabled:opacity-40">＋ {value}</button>)}
    </div>
    <form className="mt-3 flex gap-2" onSubmit={event => { event.preventDefault();save(); }}>
      <input aria-label={`${title}关键词`} value={draft} maxLength={300} onChange={e => onUpdate(values, { ...input, text: e.target.value })} placeholder={editing ? "修改关键词" : placeholder} className="min-w-0 flex-1 rounded-xl border border-mk-border bg-transparent px-3 py-2 text-mk-body" />
      <button disabled={!draft.trim() || (editing === null && values.length >= 12)} className="rounded-xl border border-mk-border px-4 py-2 text-mk-small font-semibold disabled:opacity-40">{editing ? "保存修改" : "添加"}</button>
      {(editing !== null || draft) && <button type="button" onClick={() => onUpdate(values, emptyInput)} className="text-mk-small text-mk-muted">取消</button>}
    </form>
    {(editing !== null || draft) && <p className="mt-2 text-mk-small text-mk-muted">输入会随人物板草稿保存。请添加或保存修改后再查看总结。</p>}
  </div>;
}
