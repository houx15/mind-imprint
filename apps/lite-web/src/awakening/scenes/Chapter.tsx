import { useState } from "react";

import { IMAGES } from "../assets";
import {
  ARCHIVE,
  ARCHIVE_CONCLUSION,
  ARCHIVE_EVIDENCE,
  ARCHIVE_NEXT,
  ARCHIVE_OPTIONS,
  ARCHIVE_QUESTION,
  ARCHIVE_SYSTEMS,
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

/**
 * 序章之后、能量线索之前的那六屏。
 *
 * 它们共用主线那套舱内 HUD（awakening.css 的「主线」一节），各自的版式照参考
 * 设计：城市剪影 + 对话面板、警示圆环、档案两栏 + 三条证据、三张会翻面的牌、
 * 重新决定的浅色牌堆、观察者的两栏。
 *
 * 🚨 这些屏**没有任何模型调用**，也不该有 —— 它们是固定剧情。真正花钱的只有
 * 终端那八问和最后的两次 compose。把剧情做成模型生成会让每个学生看到的世界观
 * 都不一样，而世界观恰恰是这份设计里最该稳定的东西。
 */

/* ── 序章 01：城市 + 决定 ─────────────────────────────────────────────── */

/** 六栋楼的高度和位置。写死是对的：它是一张画，不是数据。 */
const TOWERS = [
  { left: "4%", height: "46%" },
  { left: "17%", height: "68%" },
  { left: "31%", height: "38%" },
  { left: "46%", height: "76%" },
  { left: "62%", height: "52%" },
  { left: "78%", height: "64%" },
];

export function WorldScene({ onChoose }: { onChoose: (route: "joined" | "observer") => void }) {
  return (
    <section className="awk-screen" aria-label="序章：做出你的选择">
      <div className="awk-chapter">
        <div className="awk-city-panel">
          <div className="awk-city" aria-hidden="true">
            {TOWERS.map((t) => (
              <div
                key={t.left}
                className="awk-tower"
                style={{ left: t.left, height: t.height }}
              />
            ))}
          </div>
          <div className="awk-city-copy">
            <div className="awk-chapter-number">{WORLD.chapter}</div>
            <h2 className="awk-h2">{WORLD.title}</h2>
            <p className="awk-p">{WORLD.lead}</p>
          </div>
        </div>

        <div className="awk-dialog-panel">
          <div className="awk-speaker">
            <div className="awk-avatar">{WORLD.speakerMark}</div>
            <div className="awk-speaker-meta">
              <strong>{WORLD.speaker}</strong>
              <span>{WORLD.speakerState}</span>
            </div>
          </div>

          {WORLD.body.map((line) => (
            <p key={line} className="awk-p">
              {line}
            </p>
          ))}
          <div className="awk-highlight">{WORLD.highlight}</div>
          <p className="awk-p">{WORLD.prompt}</p>

          <div className="awk-choice-stack">
            {WORLD_CHOICES.map((c) => (
              <button
                key={c.key}
                type="button"
                className="awk-option"
                onClick={() => onChoose(c.key)}
              >
                <span className="awk-option-index">{c.index}</span>
                <span className="awk-option-copy">
                  <strong>{c.title}</strong>
                  <span>{c.body}</span>
                </span>
                <span className="awk-option-arrow">›</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

/* ── 提醒 ─────────────────────────────────────────────────────────────── */

export function WarningScene({ onNext }: { onNext: () => void }) {
  return (
    <section className="awk-screen" aria-label="印记的提醒">
      <div className="awk-wrap awk-two-col">
        <div>
          <div className="awk-eyebrow">{WARNING.eyebrow}</div>
          <h2 className="awk-h2">{WARNING.title}</h2>

          <div className="awk-panel" style={{ marginTop: 16 }}>
            <div className="awk-speaker">
              <div className="awk-avatar">印</div>
              <div className="awk-speaker-meta">
                <strong>{WARNING.speaker}</strong>
                <span>正在提醒你</span>
              </div>
            </div>
            {WARNING.body.map((line) => (
              <p key={line} className="awk-p">
                {line}
              </p>
            ))}
          </div>

          {WARNING.steps.map((s) => (
            <div key={s.index} className="awk-risk">
              <b>{s.index}</b>
              <span>{s.text}</span>
            </div>
          ))}

          <div className="awk-actions">
            <button type="button" className="awk-btn awk-btn--primary" onClick={onNext}>
              {WARNING.action}
            </button>
          </div>
        </div>

        <div className="awk-warning-signal" aria-hidden="true">
          <div className="awk-warning-core">!</div>
          <div className="awk-warning-caption">IMPRINT ALERT · THINK FOR YOURSELF</div>
        </div>
      </div>
    </section>
  );
}

/* ── 历史档案 ─────────────────────────────────────────────────────────── */

export function ArchiveScene({
  attempts,
  onAttempt,
  onNext,
}: {
  attempts: number;
  onAttempt: () => void;
  onNext: () => void;
}) {
  const [open, setOpen] = useState<string>("");
  const [picked, setPicked] = useState<string>("");

  const chosen = ARCHIVE_OPTIONS.find((o) => o.key === picked);
  const solved = chosen?.correct === true;

  return (
    <section className="awk-screen" aria-label="历史档案：认知让步">
      <div className="awk-wrap awk-two-col">
        <div>
          <div className="awk-eyebrow">{ARCHIVE.eyebrow}</div>
          <h2 className="awk-h2">{ARCHIVE.title}</h2>
          <p className="awk-p">{ARCHIVE.lead}</p>

          <div className="awk-panel" style={{ marginTop: 16 }}>
            <div className="awk-speaker">
              <div className="awk-avatar">印</div>
              <div className="awk-speaker-meta">
                <strong>{ARCHIVE.speaker}</strong>
                <span>正在调取旧档案</span>
              </div>
            </div>
            {ARCHIVE.intro.map((line) => (
              <p key={line} className="awk-p">
                {line}
              </p>
            ))}
          </div>

          {ARCHIVE_EVIDENCE.map((e) => {
            const isOpen = open === e.id;
            return (
              <div key={e.id} className="awk-evidence">
                <button
                  type="button"
                  className="awk-evidence-toggle"
                  aria-expanded={isOpen}
                  onClick={() => setOpen(isOpen ? "" : e.id)}
                >
                  <span className="awk-evidence-mark">{e.index}</span>
                  <span>
                    <strong>{e.title}</strong>
                    <small>{e.hint}</small>
                  </span>
                  <em>{isOpen ? "▾" : "›"}</em>
                </button>
                {isOpen ? (
                  <div className="awk-evidence-note">
                    <strong>印记助手：</strong>
                    {e.reply}
                  </div>
                ) : null}
              </div>
            );
          })}

          <div className="awk-grid-3">
            {ARCHIVE_SYSTEMS.map((sys) => (
              <div key={sys.id} className="awk-system-card">
                <b>{sys.name}</b>
                <span>{sys.body}</span>
              </div>
            ))}
          </div>
          <p className="awk-p">{ARCHIVE_CONCLUSION}</p>
        </div>

        <div>
          <figure className="awk-archive-figure" style={{ margin: 0 }}>
            <div className="awk-archive-meta">
              <span className="awk-tag">ARCHIVE NO. 01</span>
              <span className="awk-tag">认知风险记录</span>
              <span className="awk-tag awk-tag--risk">危险等级 · 高</span>
            </div>
            <img
              src={IMAGES.archiveWarning}
              alt="旧时代教育警示图：一个孩子把象征主动思考的大脑丢进回收箱，随后瘫坐在椅子上，身旁的 AI 替他完成作业、选择答案、安排日程"
            />
            <figcaption className="awk-archive-caption">
              <span>AWAKENING ALLIANCE ARCHIVE</span>
              <span>RECOVERED · 2026</span>
            </figcaption>
          </figure>

          <div className="awk-panel" style={{ marginTop: 14 }}>
            <h3 className="awk-h3">{ARCHIVE_QUESTION.label}</h3>
            <p className="awk-p">{ARCHIVE_QUESTION.ask}</p>
            <div role="group" aria-label="档案确认题选项">
              {ARCHIVE_OPTIONS.map((o) => {
                const isPicked = picked === o.key;
                const tone = !isPicked
                  ? ""
                  : o.correct
                    ? " awk-option--good"
                    : " awk-option--bad";
                return (
                  <button
                    key={o.key}
                    type="button"
                    className={`awk-option${tone}`}
                    onClick={() => {
                      setPicked(o.key);
                      // 每答一次都记一笔 —— 她试了几次是过程数据（铁律④）。
                      onAttempt();
                    }}
                  >
                    <span className="awk-option-copy">
                      <strong>{o.label}</strong>
                    </span>
                  </button>
                );
              })}
            </div>
            <p className="awk-hint">{chosen ? chosen.reply : ARCHIVE_QUESTION.hint}</p>

            <div className="awk-foot">
              <span className="awk-helper">
                {solved ? `已作答 ${attempts} 次` : "答对之后继续"}
              </span>
              <button
                type="button"
                className="awk-btn awk-btn--primary"
                disabled={!solved}
                onClick={onNext}
              >
                {ARCHIVE_NEXT}
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

/* ── 三张底牌 ─────────────────────────────────────────────────────────── */

export function DeckScene({ onDone }: { onDone: () => void }) {
  /** 已经翻开几张。round === DECK_CARDS.length 表示三张都开了。 */
  const [round, setRound] = useState(0);
  const [answer, setAnswer] = useState("");

  const card = DECK_CARDS[round];
  const finished = round >= DECK_CARDS.length;

  return (
    <section className="awk-screen" aria-label="生成式 AI 的底牌">
      <div className="awk-wrap">
        <div className="awk-head">
          <div>
            <div className="awk-eyebrow">{DECK.eyebrow}</div>
            <h2 className="awk-h2">{DECK.title}</h2>
            <p className="awk-p">{DECK.lead}</p>
          </div>
          <div className="awk-deck-counter">
            ROUND {String(Math.min(round + (finished ? 0 : 1), DECK_CARDS.length)).padStart(2, "0")}{" "}
            / {String(DECK_CARDS.length).padStart(2, "0")}
            <br />
            <span>{finished ? "三张已翻开" : "牌组已启动"}</span>
          </div>
        </div>

        {/* 三张牌一直在，翻开过的正面朝上 —— 她能看见自己的进度。 */}
        <div className="awk-deck-cards" aria-label="三张底牌">
          {DECK_CARDS.map((c, i) => (
            <div
              key={c.id}
              className={`awk-deck-card${i < round ? " awk-deck-card--open" : ""}`}
            >
              <div className="awk-deck-card-inner">
                <div className="awk-deck-back">翻开底牌</div>
                <div className="awk-deck-face">
                  <b>{c.mark}</b>
                  <span>{c.face.headline}</span>
                </div>
              </div>
            </div>
          ))}
        </div>

        {finished ? (
          <div className="awk-panel">
            <div className="awk-eyebrow">DECK COMPLETE</div>
            <h3 className="awk-h3" style={{ marginTop: 8 }}>
              {DECK.done}
            </h3>
            <div className="awk-grid-3">
              {DECK_CARDS.map((c) => (
                <div key={c.id} className="awk-system-card">
                  <b>
                    {c.mark} · {c.face.headline}
                  </b>
                  <span>{c.face.body}</span>
                </div>
              ))}
            </div>
            <div className="awk-foot">
              <span className="awk-helper">下一步：回到觉醒协议，重新回答。</span>
              <button type="button" className="awk-btn awk-btn--primary" onClick={onDone}>
                {DECK.toRejoin}
              </button>
            </div>
          </div>
        ) : card ? (
          <div className="awk-panel">
            <div className="awk-meter" aria-label="牌组进度">
              {DECK_CARDS.map((c, i) => (
                <span key={c.id} className={i <= round ? "on" : undefined} />
              ))}
            </div>
            <h3 className="awk-h3" style={{ marginTop: 12 }}>
              {card.experiment.ask}
            </h3>
            <div role="group" aria-label="这一轮的选项">
              {card.experiment.options.map((o) => (
                <button
                  key={o.key}
                  type="button"
                  className={`awk-option${answer === o.key ? " awk-option--selected" : ""}`}
                  onClick={() => setAnswer(o.key)}
                >
                  <span className="awk-option-copy">
                    <strong>{o.label}</strong>
                    <span>{o.body}</span>
                  </span>
                </button>
              ))}
            </div>
            {answer ? <div className="awk-guide">{card.experiment.reply[answer]}</div> : null}

            <div className="awk-foot">
              <span className="awk-helper">
                {answer ? "看完这一句，翻开这张底牌。" : "先完成这一轮实验。"}
              </span>
              <button
                type="button"
                className="awk-btn awk-btn--primary"
                disabled={!answer}
                onClick={() => {
                  setRound(round + 1);
                  setAnswer("");
                }}
              >
                {round === 0 ? DECK.start : DECK.next}
              </button>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}

/* ── 重新决定 ─────────────────────────────────────────────────────────── */

export function RejoinScene({ onJoin, onStay }: { onJoin: () => void; onStay: () => void }) {
  return (
    <section className="awk-screen" aria-label="重新决定是否加入">
      <div className="awk-wrap awk-two-col">
        <div>
          <div className="awk-eyebrow">{REJOIN.eyebrow}</div>
          <h2 className="awk-h2">
            {REJOIN.title}
            <br />
            {REJOIN.lead}
          </h2>

          <div className="awk-panel" style={{ marginTop: 16 }}>
            <h3 className="awk-h3">你的下一步</h3>
            <p className="awk-p">{REJOIN.body}</p>
            <div className="awk-actions">
              <button type="button" className="awk-btn awk-btn--primary" onClick={onJoin}>
                {REJOIN.join}
              </button>
              <button type="button" className="awk-btn awk-btn--ghost" onClick={onStay}>
                {REJOIN.stay}
              </button>
            </div>
          </div>
        </div>

        {/* 三张已经翻开的牌。浅色，像摆在桌上的实物。 */}
        <div className="awk-rejoin-stack" aria-label="已翻开的三张底牌">
          {DECK_CARDS.map((c) => (
            <article key={c.id} className="awk-rejoin-card">
              <b>{c.mark}</b>
              <strong>{c.face.headline}</strong>
              <span>{c.face.body}</span>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ── 观察者 ───────────────────────────────────────────────────────────── */

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
  const [kept, setKept] = useState(false);

  if (kept) {
    return (
      <section className="awk-screen" aria-label="已保留观察者身份">
        <div className="awk-wrap awk-center" style={{ paddingTop: "12vh" }}>
          <div className="awk-eyebrow">OBSERVER ROUTE</div>
          <h2 className="awk-h2">{OBSERVER_DONE.title}</h2>
          <p className="awk-p">{OBSERVER_DONE.body}</p>
          <div className="awk-actions" style={{ justifyContent: "center" }}>
            <button type="button" className="awk-btn awk-btn--primary" onClick={onLeave}>
              {OBSERVER_DONE.action}
            </button>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="awk-screen" aria-label="观察者路线">
      <div className="awk-wrap awk-two-col">
        <div>
          <div className="awk-eyebrow">{OBSERVER.eyebrow}</div>
          <h2 className="awk-h2">
            {OBSERVER.title}
            <br />
            {OBSERVER.lead}
          </h2>

          <div className="awk-panel" style={{ marginTop: 16 }}>
            <div className="awk-speaker">
              <div className="awk-avatar">印</div>
              <div className="awk-speaker-meta">
                <strong>印记</strong>
                <span>为你保留观察席</span>
              </div>
            </div>
            {OBSERVER.body.map((line) => (
              <p key={line} className="awk-p">
                {line}
              </p>
            ))}
          </div>

          <div role="group" aria-label="选择一个观察方向">
            {OBSERVER_QUESTIONS.map((q) => (
              <button
                key={q.key}
                type="button"
                className={`awk-option${question === q.key ? " awk-option--selected" : ""}`}
                onClick={() => onPick(q.key)}
              >
                <span className="awk-option-index">{q.mark}</span>
                <span className="awk-option-copy">
                  <strong>{q.label}</strong>
                </span>
              </button>
            ))}
          </div>

          <div className="awk-foot">
            <span className="awk-helper">{question ? "方向已选定。" : OBSERVER.hint}</span>
            <div className="awk-actions" style={{ marginTop: 0 }}>
              <button type="button" className="awk-btn awk-btn--ghost" onClick={onRejoin}>
                {OBSERVER.rejoin}
              </button>
              <button
                type="button"
                className="awk-btn awk-btn--primary"
                disabled={!question}
                onClick={() => {
                  onConfirm();
                  setKept(true);
                }}
              >
                {OBSERVER.confirm}
              </button>
            </div>
          </div>
        </div>

        <aside className="awk-panel awk-panel--soft" aria-label="观察者任务卡">
          <div className="awk-eyebrow">OBSERVER BRIEF / 03 TESTS</div>
          <h3 className="awk-h3" style={{ marginTop: 8 }}>
            先看见，再决定
          </h3>
          {OBSERVER_QUESTIONS.map((q, i) => (
            <div key={q.key} className="awk-item">
              <b>{String(i + 1).padStart(2, "0")}</b>
              <span>
                <strong style={{ color: "var(--awk-ink)" }}>{q.mark}</strong> · {q.label}
              </span>
            </div>
          ))}
          <div className="awk-guide">别急着接受答案。找证据，留疑问，再做决定。</div>
        </aside>
      </div>
    </section>
  );
}
