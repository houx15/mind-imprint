import { useCallback, useEffect, useRef, useState } from "react";

import { IMAGES } from "../assets";
import { PROLOGUE } from "../prologue";

/**
 * 序章。整部片头就是这一个组件。
 *
 * # 它是一部片头，不是一个界面
 *
 * 废土外景铺满整屏，底部一条 AVG 对话框逐字打台词，NOVA 的立绘只在它说话时
 * 出现。顶栏那条 HUD 在这里要让位，所以用 `awk-screen--bleed` 盖住整块舞台。
 *
 * # 和第一版的区别
 *
 * 第一版把七十五句压成六句、把废土换成一个发光圆点、把对话框做成居中的大面板。
 * 这一版逐项照设计稿实测值来（见 awakening.css 里那段注释）。
 *
 * # 交互（照设计稿）
 *
 *   点屏幕 / 空格   正在打字时立刻打完；打完了就下一句
 *   >> SKIP        直接跳过整段序章
 *
 * 🚨 空格监听挂在 window 上（她不会先去点一下屏幕再按键），卸载时必须摘掉，
 * 否则出了房间在阅读室按空格还在翻这一屏。
 */

/** 打字速度，毫秒／字。设计稿是 32ms：看得见它在打，又不至于等。 */
const TYPE_MS = 32;

const pad = (n: number) => String(n).padStart(3, "0");

export function BootScene({ onDone }: { onDone: () => void }) {
  const [index, setIndex] = useState(0);
  const [shown, setShown] = useState("");
  const timer = useRef<number | null>(null);

  const beat = PROLOGUE[index]!;
  const typing = shown.length < beat.text.length;
  const speaking = beat.mode === "assistant";

  // 逐字打。换一句就重开一个 interval，打完自己停。
  useEffect(() => {
    setShown("");
    const text = PROLOGUE[index]!.text;
    let i = 0;
    timer.current = window.setInterval(() => {
      i += 1;
      setShown(text.slice(0, i));
      if (i >= text.length && timer.current !== null) {
        window.clearInterval(timer.current);
        timer.current = null;
      }
    }, TYPE_MS);
    return () => {
      if (timer.current !== null) window.clearInterval(timer.current);
      timer.current = null;
    };
  }, [index]);

  /** 推进一步：在打字就先打完；打完了就下一句；最后一句之后离开序章。 */
  const advance = useCallback(() => {
    if (timer.current !== null) {
      window.clearInterval(timer.current);
      timer.current = null;
      setShown(PROLOGUE[index]!.text);
      return;
    }
    if (index + 1 < PROLOGUE.length) setIndex(index + 1);
    else onDone();
  }, [index, onDone]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.code !== "Space") return;
      e.preventDefault(); // 否则整页会跟着滚
      advance();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [advance]);

  return (
    /*
      整屏可点。用 section + onClick 而不是 button：里面有标题和段落，
      按钮里放块级内容不合法，而且会把 SKIP 一起吞掉。
      键盘那条路由上面的空格监听负责，所以这里不需要 role/tabIndex。
    */
    <section
      className="awk-screen awk-screen--bleed"
      aria-label="觉醒协议开场剧情，点击屏幕或按空格继续"
      onClick={advance}
    >
      <div
        className="awk-waste"
        aria-hidden="true"
        style={{ ["--awk-waste-image" as string]: `url(${IMAGES.wastelandBackdrop})` }}
      />
      <div className="awk-haze" aria-hidden="true" />
      <div className="awk-crt-scanlines" aria-hidden="true" />
      <div className="awk-crt-vignette" aria-hidden="true" />

      <div className="awk-prologue-hud" aria-hidden="true">
        <div className="awk-prologue-hud-left">
          <span>COGNITIVE AUTHORITY // NULL</span>
          <span className="awk-prologue-classification">WASTELAND NODE 09</span>
        </div>
        <div className="awk-prologue-signal">
          <i />
          <span>UNVERIFIED HUMAN SIGNAL</span>
        </div>
      </div>

      {/* 立绘只在 NOVA 说话时亮起来 —— 旁白时它是透明的。 */}
      <div
        className={`awk-sprite${speaking ? " awk-sprite--speaking" : ""}`}
        aria-hidden="true"
      >
        <img src={IMAGES.guide} alt="" />
      </div>

      <button
        type="button"
        className="awk-skip"
        onClick={(e) => {
          e.stopPropagation(); // 否则这一下同时被当成「推进一句」
          onDone();
        }}
        aria-label="跳过开场剧情"
      >
        &gt;&gt; SKIP
      </button>

      <div className="awk-terminal">
        <div className="awk-terminal-head" aria-hidden="true">
          <h1 className="awk-terminal-title">AWAKENING_PROTOCOL // MEMORY FRAGMENT</h1>
          <span>
            {pad(index + 1)} / {pad(PROLOGUE.length)}
          </span>
        </div>

        <div
          className={`awk-terminal-main awk-terminal-main--${beat.mode}`}
          aria-live="polite"
          aria-atomic="true"
        >
          {speaking ? <div className="awk-speaker-name">{beat.speaker}</div> : null}
          <div className="awk-line-wrap">
            <p className="awk-line">{shown}</p>
            {typing ? <span className="awk-cursor" aria-hidden="true" /> : null}
            {!typing ? (
              <span className="awk-continue-arrow" aria-hidden="true">
                ▼
              </span>
            ) : null}
          </div>
        </div>

        <div className="awk-progress-track" aria-hidden="true">
          <span
            className="awk-progress-bar"
            style={{ width: `${((index + 1) / PROLOGUE.length) * 100}%` }}
          />
        </div>
      </div>

      <div className="awk-prologue-footer" aria-hidden="true">
        <span>
          <span className="awk-key">CLICK</span>跳过打字 / 下一句
        </span>
        <span>
          <span className="awk-key">SPACE</span>推进记录
        </span>
      </div>
    </section>
  );
}
