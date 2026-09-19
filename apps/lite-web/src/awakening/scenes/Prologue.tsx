import { useEffect, useRef, useState } from "react";

import {
  ARCHIVE,
  ARCHIVE_CONCLUSION,
  ARCHIVE_EVIDENCE,
  ARCHIVE_NEXT,
  ARCHIVE_OPTIONS,
  ARCHIVE_QUESTION,
  ARCHIVE_SYSTEMS,
  BOOT_CONTINUE,
  BOOT_LINES,
  BOOT_SKIP,
  DECK,
  DECK_CARDS,
  OBSERVER,
  OBSERVER_DONE,
  OBSERVER_QUESTIONS,
  REJOIN,
  WARNING,
  WORLD,
  WORLD_CHOICES,
} from "../content";
import { IMAGES } from "../assets";
import { Bubble, Choice, Dim, Eyebrow, Ghost, Meter, Panel, Orbit, Primary, Stage } from "../ui";

/**
 * scenes/Prologue —— 开场到重新决定那七屏。
 *
 * 这一段**一次模型调用都不发**。一个学生走完剧情、读完档案、翻完三张底牌，
 * 到这里为止不花钱。所以它可以做得长，也应该做得长：它的任务是让她愿意在
 * 后面那八问上认真写字。
 */

/* ── 1 · 开场 ───────────────────────────────────────────────────────────── */

/**
 * 打字机。
 *
 * 两条规矩：**点一下先把当前这句打完**，再点才推进下一句（参考设计里
 * 「CLICK 跳过打字 / 下一句」就是这个）；`prefers-reduced-motion` 下整句
 * 直接出现，不逐字。
 */
function useTypewriter(text: string, enabled: boolean) {
  const [shown, setShown] = useState(enabled ? "" : text);
  const doneRef = useRef(!enabled);

  useEffect(() => {
    if (!enabled) {
      setShown(text);
      doneRef.current = true;
      return;
    }
    setShown("");
    doneRef.current = false;
    let i = 0;
    const id = window.setInterval(() => {
      i += 1;
      setShown(text.slice(0, i));
      if (i >= text.length) {
        doneRef.current = true;
        window.clearInterval(id);
      }
    }, 38);
    return () => window.clearInterval(id);
  }, [text, enabled]);

  return {
    shown,
    get done() {
      return doneRef.current;
    },
    finish: () => {
      doneRef.current = true;
      setShown(text);
    },
  };
}

export function BootScene({ onDone }: { onDone: () => void }) {
  const reduced =
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
  const [i, setI] = useState(0);
  const line = BOOT_LINES[i] ?? "";
  const tw = useTypewriter(line, !reduced);

  const advance = () => {
    if (!tw.done) {
      tw.finish();
      return;
    }
    if (i + 1 < BOOT_LINES.length) {
      setI(i + 1);
      return;
    }
    onDone();
  };

  return (
    <Stage>
      <div className="flex flex-col items-center gap-10 pt-6 text-center">
        <Orbit />
        <div className="w-full">
          <Eyebrow>AWAKENING_PROTOCOL // MEMORY FRAGMENT</Eyebrow>
          <p className="mt-5 min-h-[5.5rem] text-[19px] leading-[1.9]">
            {tw.shown}
            <span className="ml-0.5 inline-block w-[2px] animate-pulse bg-current align-middle" style={{ height: "1.1em" }} />
          </p>
          <div className="awk-dim mt-2 font-mono text-[11px]">
            {String(i + 1).padStart(3, "0")} / {String(BOOT_LINES.length).padStart(3, "0")}
          </div>
        </div>
        <div className="flex items-center gap-4">
          <Ghost onClick={onDone}>{BOOT_SKIP}</Ghost>
          <Primary onClick={advance}>{BOOT_CONTINUE}</Primary>
        </div>
      </div>
    </Stage>
  );
}

/* ── 2 · 序章 ───────────────────────────────────────────────────────────── */

export function WorldScene({ onChoose }: { onChoose: (route: "joined" | "observer") => void }) {
  return (
    <Stage>
      <Eyebrow>{WORLD.chapter}</Eyebrow>
      <h1 className="mt-3 text-[30px] font-semibold leading-tight">{WORLD.title}</h1>
      <p className="awk-dim mt-2 text-[15px]">{WORLD.lead}</p>

      <div className="mt-7">
        <Bubble speaker={WORLD.speaker}>
          {WORLD.body.map((p) => (
            <p key={p} className="mt-1 first:mt-0">
              {p}
            </p>
          ))}
        </Bubble>
      </div>

      <p className="awk-dim mt-7 text-[13px]">{WORLD.prompt}</p>
      <div className="mt-3 grid gap-3">
        {WORLD_CHOICES.map((c) => (
          <Choice
            key={c.key}
            index={c.index}
            title={c.title}
            body={c.body}
            onClick={() => onChoose(c.key)}
          />
        ))}
      </div>
    </Stage>
  );
}

/* ── 3 · 提醒 ───────────────────────────────────────────────────────────── */

export function WarningScene({ onNext }: { onNext: () => void }) {
  return (
    <Stage>
      <Eyebrow>{WARNING.eyebrow}</Eyebrow>
      <h1 className="mt-3 text-[28px] font-semibold leading-tight">{WARNING.title}</h1>

      <div className="mt-6">
        <Bubble speaker={WARNING.speaker}>
          {WARNING.body.map((p) => (
            <p key={p} className="mt-3 first:mt-0">
              {p}
            </p>
          ))}
        </Bubble>
      </div>

      <div className="awk-soft mt-6 p-5">
        {WARNING.steps.map((s) => (
          <div key={s.index} className="flex items-start gap-3 py-1.5">
            <span className="awk-index">{s.index}</span>
            <span className="text-[14px]">{s.text}</span>
          </div>
        ))}
      </div>

      <div className="mt-7">
        <Primary onClick={onNext}>{WARNING.action}</Primary>
      </div>
    </Stage>
  );
}

/* ── 4 · 历史档案 ───────────────────────────────────────────────────────── */

export function ArchiveScene({
  attempts,
  onAttempt,
  onNext,
}: {
  attempts: number;
  /** 她选错一次就加一。不减分，是过程数据（铁律④）。 */
  onAttempt: () => void;
  onNext: () => void;
}) {
  const [opened, setOpened] = useState<string[]>([]);
  const [picked, setPicked] = useState<string>("");
  const chosen = ARCHIVE_OPTIONS.find((o) => o.key === picked);
  const passed = chosen?.correct === true;
  const allOpened = opened.length === ARCHIVE_EVIDENCE.length;

  return (
    <Stage>
      <Eyebrow>{ARCHIVE.eyebrow}</Eyebrow>
      <h1 className="mt-3 text-[28px] font-semibold leading-tight">{ARCHIVE.title}</h1>
      <p className="awk-dim mt-2 text-[15px]">{ARCHIVE.lead}</p>

      <div className="mt-6">
        <Bubble speaker={ARCHIVE.speaker}>
          {ARCHIVE.intro.map((p) => (
            <p key={p} className="mt-2 first:mt-0">
              {p}
            </p>
          ))}
        </Bubble>
      </div>

      {/* 那张教育警示图。三条证据说的就是这张图里的三个部分，所以它必须
          在证据上面 —— 先看见，再读解释。加载失败就整块收起：一个碎图标
          比没有图糟。 */}
      <figure className="awk-soft mt-6 overflow-hidden">
        <img
          src={IMAGES.archiveWarning}
          alt="旧时代的教育警示图：一个孩子把主动思考丢进回收箱"
          loading="lazy"
          className="block w-full"
          onError={(e) => {
            const fig = e.currentTarget.closest("figure");
            if (fig) fig.style.display = "none";
          }}
        />
        <figcaption className="awk-dim px-4 py-3 font-mono text-[11px]">
          ARCHIVE NO. 01 · 认知风险记录 · 危险等级 高
        </figcaption>
      </figure>

      <div className="mt-6 grid gap-3">
        {ARCHIVE_EVIDENCE.map((e) => {
          const isOpen = opened.includes(e.id);
          return (
            <div key={e.id}>
              <Choice
                index={e.index}
                title={e.title}
                body={e.hint}
                selected={isOpen}
                onClick={() => setOpened((o) => (o.includes(e.id) ? o : [...o, e.id]))}
              />
              {isOpen ? (
                <div className="mt-2 pl-1">
                  <Bubble>{e.reply}</Bubble>
                </div>
              ) : null}
            </div>
          );
        })}
      </div>

      {allOpened ? (
        <>
          <div className="awk-soft mt-7 p-5">
            {ARCHIVE_SYSTEMS.map((s) => (
              <div key={s.id} className="border-b border-[var(--line)] py-3 last:border-0 last:pb-0 first:pt-0">
                <div className="font-semibold">{s.name}</div>
                <div className="awk-dim mt-1 text-[13px] leading-relaxed">{s.body}</div>
              </div>
            ))}
            <p className="mt-4 text-[14px] leading-relaxed">{ARCHIVE_CONCLUSION}</p>
          </div>

          <div className="mt-7">
            <div className="awk-eyebrow">{ARCHIVE_QUESTION.label}</div>
            <p className="mt-2 text-[15px] leading-relaxed">{ARCHIVE_QUESTION.ask}</p>
            <div className="mt-3 grid gap-2">
              {ARCHIVE_OPTIONS.map((o) => (
                <Choice
                  key={o.key}
                  title={o.label}
                  selected={picked === o.key && o.correct}
                  wrong={picked === o.key && !o.correct}
                  onClick={() => {
                    setPicked(o.key);
                    if (!o.correct) onAttempt();
                  }}
                  disabled={passed}
                />
              ))}
            </div>
            {chosen ? (
              <div className="mt-3">
                <Bubble>{chosen.reply}</Bubble>
              </div>
            ) : (
              <p className="awk-dim mt-3 text-[13px]">{ARCHIVE_QUESTION.hint}</p>
            )}
          </div>

          <div className="mt-7 flex items-center gap-4">
            <Primary onClick={onNext} disabled={!passed}>
              {ARCHIVE_NEXT}
            </Primary>
            {attempts > 0 ? (
              <Dim>
                <span className="font-mono text-[12px]">已尝试 {attempts + (passed ? 1 : 0)} 次</span>
              </Dim>
            ) : null}
          </div>
        </>
      ) : (
        <p className="awk-dim mt-6 text-[13px]">请打开三条证据。</p>
      )}
    </Stage>
  );
}

/* ── 5 · AI 底牌 ────────────────────────────────────────────────────────── */

export function DeckScene({ onDone }: { onDone: () => void }) {
  const [i, setI] = useState(0);
  const [picked, setPicked] = useState<string>("");
  const [flipped, setFlipped] = useState<string[]>([]);
  const card = DECK_CARDS[i]!;
  const answered = picked !== "";
  const allDone = flipped.length === DECK_CARDS.length;

  const next = () => {
    setFlipped((f) => (f.includes(card.id) ? f : [...f, card.id]));
    setPicked("");
    if (i + 1 < DECK_CARDS.length) {
      setI(i + 1);
      return;
    }
  };

  if (allDone) {
    return (
      <Stage>
        <Eyebrow>{DECK.eyebrow}</Eyebrow>
        <h1 className="mt-3 text-[28px] font-semibold">{DECK.done}</h1>
        <div className="mt-6 grid gap-3 sm:grid-cols-3">
          {DECK_CARDS.map((c) => (
            <div key={c.id} className="awk-soft p-5">
              <div className="text-[28px] font-semibold" style={{ color: "var(--cyan)" }}>
                {c.mark}
              </div>
              <div className="mt-2 font-semibold">{c.face.headline}</div>
              <div className="awk-dim mt-2 text-[13px] leading-relaxed">{c.face.body}</div>
            </div>
          ))}
        </div>
        <div className="mt-8">
          <Primary onClick={onDone}>{DECK.toRejoin}</Primary>
        </div>
      </Stage>
    );
  }

  return (
    <Stage>
      <div className="flex items-center justify-between gap-4">
        <div>
          <Eyebrow>{DECK.eyebrow}</Eyebrow>
          <h1 className="mt-2 text-[26px] font-semibold">{DECK.title}</h1>
        </div>
        <Meter total={DECK_CARDS.length} done={flipped.length} />
      </div>
      <p className="awk-dim mt-2 text-[15px]">{DECK.lead}</p>

      <Panel className="mt-6">
        <div className="flex items-baseline gap-3">
          <span className="text-[34px] font-semibold" style={{ color: "var(--cyan)" }}>
            {card.mark}
          </span>
          <span className="awk-dim text-[13px]">{card.tease}</span>
        </div>

        <p className="mt-5 text-[15px] leading-relaxed">{card.experiment.ask}</p>

        <div className="mt-4 grid gap-2">
          {card.experiment.options.map((o) => (
            <Choice
              key={o.key}
              title={o.label}
              body={o.body}
              selected={picked === o.key}
              onClick={() => setPicked(o.key)}
              disabled={answered}
            />
          ))}
        </div>

        {answered ? (
          <>
            <div className="mt-4">
              <Bubble speaker="印记">{card.experiment.reply[picked]}</Bubble>
            </div>
            <div className="awk-soft mt-4 p-4">
              <div className="font-semibold">{card.face.headline}</div>
              <div className="awk-dim mt-1 text-[13px] leading-relaxed">{card.face.body}</div>
            </div>
            <div className="mt-5">
              <Primary onClick={next}>
                {i + 1 < DECK_CARDS.length ? DECK.next : DECK.done}
              </Primary>
            </div>
          </>
        ) : (
          <p className="awk-dim mt-4 text-[13px]">请选择一个答案。两个选项都能继续。</p>
        )}
      </Panel>
    </Stage>
  );
}

/* ── 6 · 重新决定 ───────────────────────────────────────────────────────── */

export function RejoinScene({
  onJoin,
  onStay,
}: {
  onJoin: () => void;
  onStay: () => void;
}) {
  return (
    <Stage>
      <Eyebrow>{REJOIN.eyebrow}</Eyebrow>
      <h1 className="mt-3 text-[28px] font-semibold leading-tight">{REJOIN.title}</h1>
      <p className="awk-dim mt-2 text-[15px]">{REJOIN.lead}</p>

      <div className="awk-soft mt-6 grid gap-2 p-5 sm:grid-cols-3">
        {DECK_CARDS.map((c) => (
          <div key={c.id}>
            <div className="text-[20px] font-semibold" style={{ color: "var(--cyan)" }}>
              {c.mark}
            </div>
            <div className="awk-dim mt-1 text-[12px] leading-relaxed">{c.face.headline}</div>
          </div>
        ))}
      </div>

      <p className="mt-6 text-[15px] leading-relaxed">{REJOIN.body}</p>
      <div className="mt-6 flex flex-wrap items-center gap-4">
        <Primary onClick={onJoin}>{REJOIN.join}</Primary>
        <Ghost onClick={onStay}>{REJOIN.stay}</Ghost>
      </div>
    </Stage>
  );
}

/* ── 7 · 观察者 ─────────────────────────────────────────────────────────── */

export function ObserverScene({
  question,
  onPick,
  onConfirm,
  onRejoin,
  onLeave,
}: {
  question: string;
  onPick: (key: string) => void;
  onConfirm: () => void;
  onRejoin: () => void;
  onLeave: () => void;
}) {
  const [confirmed, setConfirmed] = useState(false);

  if (confirmed) {
    return (
      <Stage>
        <Eyebrow>{OBSERVER.eyebrow}</Eyebrow>
        <h1 className="mt-3 text-[26px] font-semibold">{OBSERVER_DONE.title}</h1>
        <p className="mt-3 text-[15px] leading-relaxed">{OBSERVER_DONE.body}</p>
        <div className="mt-7 flex flex-wrap gap-4">
          <Primary onClick={onLeave}>{OBSERVER_DONE.action}</Primary>
          <Ghost onClick={onRejoin}>{OBSERVER.rejoin}</Ghost>
        </div>
      </Stage>
    );
  }

  return (
    <Stage>
      <Eyebrow>{OBSERVER.eyebrow}</Eyebrow>
      <h1 className="mt-3 text-[28px] font-semibold leading-tight">{OBSERVER.title}</h1>
      <p className="awk-dim mt-2 text-[15px]">{OBSERVER.lead}</p>

      <div className="mt-6">
        <Bubble speaker="印记">
          {OBSERVER.body.map((p) => (
            <p key={p} className="mt-2 first:mt-0">
              {p}
            </p>
          ))}
        </Bubble>
      </div>

      <div className="mt-5 grid gap-2">
        {OBSERVER_QUESTIONS.map((q) => (
          <Choice
            key={q.key}
            index={q.mark}
            title={q.label}
            selected={question === q.key}
            onClick={() => onPick(q.key)}
          />
        ))}
      </div>
      {question === "" ? <p className="awk-dim mt-3 text-[13px]">{OBSERVER.hint}</p> : null}

      <div className="mt-7 flex flex-wrap items-center gap-4">
        <Primary
          disabled={question === ""}
          onClick={() => {
            onConfirm();
            setConfirmed(true);
          }}
        >
          {OBSERVER.confirm}
        </Primary>
        <Ghost onClick={onRejoin}>{OBSERVER.rejoin}</Ghost>
      </div>
    </Stage>
  );
}
