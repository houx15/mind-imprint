import { useId, useLayoutEffect, useRef, useState } from "react";
import { ArrowRight, BookOpenText, ChevronDown, ChevronUp } from "lucide-react";
import { Button, Icon } from "@/ui";
import type { WritingPrompt } from "../api/writingPrompts";
import "./prompt-library.css";

/** Shared by the library and landing recommendations. Original prompt text
 * stays expandable; choosing a card still uses its existing server ID. */
export function PromptCard({ prompt, busy, onStart, recommended = false }: {
  prompt: WritingPrompt;
  busy: boolean;
  onStart: (id: string) => void;
  recommended?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const textId = useId();
  const textRef = useRef<HTMLParagraphElement>(null);
  const [overflows, setOverflows] = useState(false);
  // Measure actual lines: English, Chinese and narrow cards wrap differently.
  useLayoutEffect(() => {
    const text = textRef.current;
    if (!text || open) return;
    const measure = () => setOverflows(text.scrollHeight > text.clientHeight + 1);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(text);
    return () => observer.disconnect();
  }, [prompt.text, open]);

  return (
    <article className={`writing-library-card${recommended ? " is-recommended" : ""}`}>
      <header className="writing-library-card__header">
        <span className="writing-library-card__mark" aria-hidden="true"><Icon icon={BookOpenText} size={21} /></span>
        <div className="writing-library-card__identity">
          <h3>{prompt.category}</h3>
          <span>{[prompt.year, prompt.type].filter(Boolean).join(" · ")}</span>
        </div>
        {recommended && <span className="writing-library-card__recommend">推荐</span>}
      </header>
      <div className="writing-library-card__body">
        <div className="writing-library-card__kind"><span>{prompt.taskType}</span><span>{prompt.diffLabel}</span></div>
        <p ref={textRef} id={textId} className={`writing-library-card__text${open ? " is-open" : ""}`} lang={prompt.lang}>{prompt.text}</p>
        {(overflows || open) && <button type="button" className="writing-library-card__expand" aria-expanded={open} aria-controls={textId} onClick={() => setOpen(!open)}>
          {open ? "收起" : "展开全文"}<Icon icon={open ? ChevronUp : ChevronDown} size={14} />
        </button>}
        {prompt.topics.length > 0 && <ul className="writing-library-card__topics" aria-label="话题">{prompt.topics.map(t => <li key={t}>{t}</li>)}</ul>}
      </div>
      <div className="writing-library-card__bottom">
        {(prompt.wordLimit || prompt.minutes || prompt.fullScore) ? <dl className="writing-library-card__requirements">
          {prompt.wordLimit && <div><dt>字数要求</dt><dd>{prompt.wordLimit}</dd></div>}
          {prompt.minutes ? <div><dt>时间</dt><dd>{prompt.minutes} 分钟</dd></div> : null}
          {prompt.fullScore ? <div><dt>满分</dt><dd>{prompt.fullScore} 分</dd></div> : null}
        </dl> : null}
        {prompt.source && <p className="writing-library-card__source">{prompt.source}</p>}
        <Button size="sm" variant={recommended ? "primary" : "secondary"} loading={busy} onClick={() => onStart(prompt.id)} iconEnd={<Icon icon={ArrowRight} size={15} />}>用这道题写</Button>
      </div>
    </article>
  );
}
