import { useEffect, useRef, useState } from "react";

import { type AwakeningRun, postTurn } from "../../api/awakening";
import { IMAGES } from "../assets";
import { CHALLENGE, CHALLENGE_OPTIONS, LENS, TERMINAL } from "../content";
import { Bubble, Choice, Dim, Eyebrow, Meter, Panel, Primary, Stage } from "../ui";

/**
 * scenes/Terminal —— 八问，以及紧跟它的两屏选择。
 *
 * # 这一屏和别处不一样的地方
 *
 * 其它屏都在前端自己走，这一屏每一轮都要一次服务端往返：节点推进由服务端算，
 * 模型只负责接住她上一句、把当前这一问问出来。所以这里没有「下一题」按钮，
 * 只有发送。
 *
 * # 三件必须照实说的事
 *
 *  1. 模型没回上来时 `reply` 是空串。这时显示一句明确的失败，**不填一句像样
 *     的话**（memory: ai-errors-must-surface-never-fake）。她写的内容已经存下
 *     来了，再发一次就行。
 *  2. 她答得太薄时服务端会在**同一个节点**换个问法再问一次。这时步骤条不推进，
 *     因为它没推进。
 *  3. 她自己写的字不截断、不省略。终端里她说过的每一句都原样留在上面
 *     （memory: observation-tool-is-the-bug-2026-09-12）。
 */

interface Line {
  who: "her" | "imprint";
  text: string;
  /** 印记这一轮没回上来。 */
  failed?: boolean;
}

export function TerminalScene({
  run,
  onDone,
}: {
  run: AwakeningRun;
  onDone: () => void;
}) {
  // 已经发生过的轮次先铺上去 —— 她刷新过、或者昨天走到一半。
  const [lines, setLines] = useState<Line[]>(() => {
    const out: Line[] = [];
    for (const t of run.turns) {
      out.push({ who: "her", text: t.studentText });
      out.push({ who: "imprint", text: t.reply, failed: t.reply === "" });
    }
    return out;
  });
  const [node, setNode] = useState(run.nextNode);
  const [ask, setAsk] = useState(() => askFor(run));
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(run.nextNode >= TERMINAL.steps.length);
  const [error, setError] = useState("");
  const bottom = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [lines.length, busy]);

  const send = async () => {
    const value = text.trim();
    if (!value || busy) return;
    setBusy(true);
    setError("");
    setLines((l) => [...l, { who: "her", text: value }]);
    setText("");
    try {
      const res = await postTurn(run.id, value);
      setLines((l) => [...l, { who: "imprint", text: res.reply, failed: res.failed }]);
      setNode(res.nextNode);
      setAsk(res.ask);
      setDone(res.done);
    } catch (e: unknown) {
      // 报错照实说，带后台原话。
      setError(e instanceof Error && e.message ? e.message : "发送失败");
      setLines((l) => l.slice(0, -1));
      setText(value); // 她的字还给她，不丢
    } finally {
      setBusy(false);
    }
  };

  /* 这一屏是一整块聊天。会话吃掉全部剩余高度，输入钉在底上，
     「当前这一问」是会话里的最后一条消息 —— 不再飘在输入框上面。 */
  const step = Math.min(node + 1, TERMINAL.steps.length);
  const stateLabel = done ? "COMPLETE" : busy ? "THINKING" : "AWAITING INPUT";

  return (
    <section
      className="awk-term-screen"
      aria-label="兴趣信号诊断终端"
      style={{ ["--awk-term-plate" as string]: `url(${IMAGES.archiveBackdrop})` }}
    >
      <div className="awk-term">
        <header className="awk-term-head">
          <div className="awk-term-title">
            <b>印记 // {TERMINAL.eyebrow}</b>
            {/* 副标题说的是真状态：走到第几步、这一步问的是什么。 */}
            <small>
              {done
                ? "八问已完成"
                : `${TERMINAL.title} · 第 ${step} / ${TERMINAL.steps.length} 步 · ${
                    TERMINAL.steps[Math.min(node, TERMINAL.steps.length - 1)]
                  }`}
            </small>
          </div>
          <div className="awk-term-phase">
            <span>
              NODE {String(step).padStart(2, "0")} / {String(TERMINAL.steps.length).padStart(2, "0")}
            </span>
            <strong>{stateLabel}</strong>
          </div>
        </header>

        <div className="awk-term-log" role="log" aria-live="polite">
          {lines.map((l, i) =>
            l.who === "her" ? (
              <div key={i} className="awk-term-her">
                {l.text}
              </div>
            ) : l.failed ? (
              <div key={i} className="awk-term-failed">
                {TERMINAL.failed}
              </div>
            ) : (
              <div key={i} className="awk-term-imprint">
                <span className="awk-term-who">印记</span>
                {l.text}
              </div>
            ),
          )}

          {/* 当前这一问。会话的最后一条，亮一点。 */}
          {!done && !busy && ask ? (
            <div className="awk-term-imprint awk-term-ask">
              <span className="awk-term-who">印记</span>
              {ask}
            </div>
          ) : null}

          {busy ? (
            <div className="awk-term-busy">
              <span className="awk-dot" />
              印记正在回复
            </div>
          ) : null}

          {error ? (
            <div className="awk-term-failed">发送失败：{error}</div>
          ) : null}

          <div ref={bottom} />
        </div>

        {done ? (
          <div className="awk-term-done">
            <span>八个问题已经问完。</span>
            <button type="button" className="awk-term-send" onClick={onDone}>
              {TERMINAL.finish}
            </button>
          </div>
        ) : (
          <form
            className="awk-term-input"
            onSubmit={(e) => {
              e.preventDefault();
              void send();
            }}
          >
            <div className="awk-term-input-wrap">
              <span className="awk-term-prompt" aria-hidden="true">
                &gt;_
              </span>
              <textarea
                className="awk-term-field"
                rows={1}
                placeholder={TERMINAL.placeholder}
                value={text}
                disabled={busy}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    void send();
                  }
                }}
              />
              <span className="awk-term-hint">{TERMINAL.hint}</span>
            </div>
            <button
              type="submit"
              className="awk-term-send"
              disabled={busy || text.trim() === ""}
            >
              {TERMINAL.send}
            </button>
          </form>
        )}
      </div>
    </section>
  );
}

/** 第一问。服务端已经算好了（第二趟起会提到她树上已有的词）。 */
function askFor(run: AwakeningRun): string {
  return run.openingAsk || "";
}

/* ── 三层追问 ───────────────────────────────────────────────────────────── */

export function LensScene({
  choice,
  onPick,
  onConfirm,
}: {
  choice: string;
  onPick: (key: string) => void;
  onConfirm: () => void;
}) {
  return (
    <Stage>
      <Eyebrow>{LENS.eyebrow}</Eyebrow>
      <h1 className="mt-3 text-[26px] font-semibold">{LENS.title}</h1>
      <p className="awk-dim mt-2 text-[15px] leading-relaxed">{LENS.lead}</p>

      <div className="mt-6 grid gap-3">
        {LENS.layers.map((l) => (
          <Choice
            key={l.id}
            index={l.code.slice(0, 2)}
            title={l.title}
            body={l.body}
            selected={choice === l.id}
            onClick={() => onPick(l.id)}
          />
        ))}
      </div>
      {choice === "" ? <p className="awk-dim mt-3 text-[13px]">{LENS.hint}</p> : null}

      <div className="mt-7">
        <Primary disabled={choice === ""} onClick={onConfirm}>
          {LENS.confirm}
        </Primary>
      </div>
    </Stage>
  );
}

/* ── 下一步 ─────────────────────────────────────────────────────────────── */

export function ChallengeScene({
  choice,
  onPick,
  onConfirm,
}: {
  choice: string;
  onPick: (key: string) => void;
  onConfirm: () => void;
}) {
  return (
    <Stage>
      <Eyebrow>{CHALLENGE.eyebrow}</Eyebrow>
      <h1 className="mt-3 text-[26px] font-semibold">{CHALLENGE.title}</h1>
      <p className="awk-dim mt-2 text-[15px] leading-relaxed">{CHALLENGE.lead}</p>

      <div className="mt-6 grid gap-3">
        {CHALLENGE_OPTIONS.map((o) => (
          <Choice
            key={o.key}
            index={o.index}
            title={o.title}
            body={o.body}
            selected={choice === o.key}
            onClick={() => onPick(o.key)}
          />
        ))}
      </div>
      {choice === "" ? <p className="awk-dim mt-3 text-[13px]">{CHALLENGE.hint}</p> : null}

      <div className="mt-7">
        <Primary disabled={choice === ""} onClick={onConfirm}>
          {CHALLENGE.confirm}
        </Primary>
      </div>
    </Stage>
  );
}
