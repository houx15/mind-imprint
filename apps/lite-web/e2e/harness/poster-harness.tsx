import { useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { ReportPoster } from "../../src/reports/ReportPoster";
import { exportPoster, posterSize, fittingPixelRatio } from "../../src/reports/exportPoster";
import type { LiteReport } from "../../src/api/reports";
import { useLiteTheme } from "../../src/shared/useLiteTheme";
import { AccentProvider } from "@/ui";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import { toPng } from "html-to-image";
import "../../src/index.css";
import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";
import "../../src/reports/report-visuals.css";

/**
 * 阅读报告海报的看图台 —— 专为「导出的 PNG 是不是完整的」存在。
 *
 * 产品负责人 2026-09-23 第 2 条：「when export reading report png, the picture
 * seems to be truncated and is not complete.」
 *
 * 🚨 为什么必须在真浏览器里看：memory
 * `look-at-the-image-not-the-assertions-2026-09-22` —— 上一次我建了看图台、
 * 跑了五条走查、**没打开那张截图**，而图是裸的。这一次要验的东西本身就是
 * 一张图，jsdom 里 `scrollHeight` 恒等于 0，连量都量不出来。
 *
 * 🚨 主题照 LiteApp 的真做法装（AccentProvider + useLiteTheme），
 * 不手搓 `--mk-theme-accent-*`。同一条记录里的第二个坑。
 *
 * 台上摆两份报告：
 *   - `short` —— 一份普通长度的；
 *   - `long`  —— 金句、摘抄、收获都拉满，把海报撑到远超一屏。
 *     裁切是**长**的时候才发作的，短的那份怎么量都对。
 */

function moments(n: number) {
  return Array.from({ length: n }, (_, i) => ({
    quote: `第${i + 1}句金句：他心里的依据是「慢慢地倒了」——在他眼里，她是慢慢倒下去的，不是被撞得重重摔下。`,
    where: `第${i + 3}段`,
  }));
}

const BASE: LiteReport = {
  version: 1,
  kind: "reading",
  title: "一件小事",
  studentName: "林知遥",
  finishedAt: "2026-09-23T12:00:00Z",
  ordinal: 8,
  stats: [
    { key: "blocks", label: "读过的段落", value: 14, unit: "段" },
    { key: "notes", label: "批注", value: 6, unit: "条" },
    { key: "excerpts", label: "摘抄", value: 9, unit: "句" },
    { key: "minutes", label: "用时", value: 42, unit: "分钟" },
  ],
  moments: moments(3),
  keep: {
    label: "我的收获",
    text: "我原来以为车夫只是好心，读到第 12 段才发现，那一下让「我」看见了自己身上的小。",
    source: "student",
  },
  // 🚨 这四样她真的产出过，而 2026-09-23 之前**一节都不在导出的图里**。
  gains: ["读记叙文先看「谁在看」，叙述者自己也是被写的一个人。", "转折要有一处看得见的依据，不能只写「我突然明白了」。"],
  lensNotes: [{ lens: "证据链", quote: "我料定这老女人并没有伤", finding: "这是「我」的推断，不是事实。" }],
  notes: [{ quote: "须仰视才见", note: "把人放在低处，再让他抬头。" }],
  excerpts: [
    "刚近S门，忽而车把上带着一个人，慢慢地倒了。",
    "我料定这老女人并没有伤，别人也没有看见。",
    "独有这一件小事，却总是浮在我眼前，有时反更明显。",
  ],
  turningPoints: [],
  article: { sourceUrl: "https://example.com/yijianxiaoshi", host: "example.com", excerpt: "" },
  piece: "",
  prosePending: false,
};

const LONG: LiteReport = {
  ...BASE,
  title: "一件小事（长报告）",
  moments: moments(3),
  keep: {
    label: "我的收获",
    // 🚨 一段长到会把海报撑高的字 —— 裁切只有在这种时候才看得出来。
    text: Array.from(
      { length: 8 },
      (_, i) =>
        `第${i + 1}段想法：我原来以为车夫只是好心，读到第 12 段才发现，那一下让「我」看见了自己身上的小。` +
        `作者没有写「我很惭愧」，他写的是「须仰视才见」——把一个人放在低处，再让他抬头。`,
    ).join("\n\n"),
    source: "student",
  },
  stats: [
    { key: "blocks", label: "读过的段落", value: 14, unit: "段" },
    { key: "notes", label: "批注", value: 6, unit: "条" },
    { key: "excerpts", label: "摘抄", value: 9, unit: "句" },
    { key: "minutes", label: "用时", value: 42, unit: "分钟" },
  ],
};

/**
 * 🚨 一份**很长**的报告：高到超过 8192 CSS px。
 *
 * 这是唯一一种我能在台上稳定复现的裁切：`pixelRatio: 2` 把它画成 16384+ px，
 * 撞上 html-to-image 的 canvasDimensionLimit。它**不报错**，自己把整张图缩放
 * 到装得下为止 —— 于是导出来的图比例不对、字也糊了，而调用方什么都不知道。
 *
 * 她的阅读报告真的能长到这个高度：金句 + 一段很长的收获 + 摘抄，1080 宽的
 * 海报里都是 38px 的正文。
 */
const VERY_LONG: LiteReport = {
  ...BASE,
  title: "一件小事（超长报告）",
  keep: {
    label: "我的收获",
    text: Array.from(
      { length: 40 },
      (_, i) =>
        `第${i + 1}段想法：我原来以为车夫只是好心，读到第 12 段才发现，那一下让「我」看见了自己身上的小。` +
        `作者没有写「我很惭愧」，他写的是「须仰视才见」——把一个人放在低处，再让他抬头。`,
    ).join("\n\n"),
    source: "student",
  },
};

function Stage() {
  const { themeStyle } = useLiteTheme();
  const shortRef = useRef<HTMLDivElement>(null);
  const longRef = useRef<HTMLDivElement>(null);
  const hugeRef = useRef<HTMLDivElement>(null);
  const [out, setOut] = useState<string>("");

  /**
   * 量一遍 + 真的画一张，把结果写到 DOM 上给走查读。
   *
   * 画出来的那张 PNG 也挂到页面上 —— 这样截图里看见的就是**导出的那张图
   * 本身**，不是海报在页面上的样子。两者不一样正是这个 bug。
   */
  const refOf = (which: string) =>
    which === "short" ? shortRef : which === "huge" ? hugeRef : longRef;

  async function run(which: "short" | "long" | "huge") {
    const node = refOf(which).current;
    if (!node) return;
    const size = posterSize(node);
    const ratio = fittingPixelRatio(size.width, size.height);
    const dataUrl = await toPng(node, {
      pixelRatio: ratio,
      cacheBust: true,
      width: size.width,
      height: size.height,
      style: { overflow: "visible", width: `${size.width}px`, height: `${size.height}px` },
    });
    const img = new Image();
    await new Promise((r) => {
      img.onload = r;
      img.src = dataUrl;
    });
    setOut(
      JSON.stringify({
        which,
        clientHeight: node.clientHeight,
        scrollHeight: node.scrollHeight,
        measured: size,
        ratio,
        png: { width: img.naturalWidth, height: img.naturalHeight },
      }),
    );
    const slot = document.getElementById(`png-${which}`);
    if (slot) {
      slot.innerHTML = "";
      img.style.width = "360px";
      img.style.border = "1px solid #999";
      slot.appendChild(img);
    }
  }

  /**
   * 🚨 **老写法**，原样保留在台上：一个尺寸都不传，让 html-to-image 自己去量
   * `clientWidth/clientHeight`。
   *
   * 留着它不是为了好玩 —— 是为了能证明「修好了」。没有这一条，新写法画出来
   * 的图再完整，也说不清楚老写法到底有没有裁
   *（memory: observation-tool-is-the-bug-2026-09-12 的反面：
   *  先证明那个毛病真的在，再说自己修好了它）。
   */
  async function runOld(which: "short" | "long" | "huge") {
    const node = refOf(which).current;
    if (!node) return;
    const dataUrl = await toPng(node, { pixelRatio: 2, cacheBust: true });
    const img = new Image();
    await new Promise((r) => {
      img.onload = r;
      img.src = dataUrl;
    });
    setOut(
      JSON.stringify({
        which,
        how: "old",
        clientHeight: node.clientHeight,
        scrollHeight: node.scrollHeight,
        measured: posterSize(node),
        ratio: 2,
        png: { width: img.naturalWidth, height: img.naturalHeight },
      }),
    );
  }

  return (
    <div style={{ ...themeStyle, padding: 24, background: "var(--mk-paper)" }}>
      <div style={{ display: "flex", gap: 12, marginBottom: 16 }}>
        <button data-run="short" onClick={() => void run("short")}>
          导出短报告
        </button>
        <button data-run="long" onClick={() => void run("long")}>
          导出长报告
        </button>
        <button data-run="old-long" onClick={() => void runOld("long")}>
          老写法导出长报告
        </button>
        <button data-run="huge" onClick={() => void run("huge")}>
          新写法导出超长报告
        </button>
        <button data-run="old-huge" onClick={() => void runOld("huge")}>
          老写法导出超长报告
        </button>
        <button data-run="download" onClick={() => void exportPoster(longRef.current, "long.png")}>
          走一遍真的 exportPoster
        </button>
      </div>
      <pre data-out style={{ minHeight: 24, fontSize: 12 }}>{out}</pre>
      <div style={{ display: "flex", gap: 24 }}>
        <div id="png-short" data-png-short />
        <div id="png-long" data-png-long />
      </div>
      <ReportPoster ref={shortRef} report={BASE} />
      <ReportPoster ref={longRef} report={LONG} />
      <ReportPoster ref={hugeRef} report={VERY_LONG} />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <AccentProvider presets={LITE_ACCENT_PRESETS} initialAccent={LITE_ACCENT_PRESETS[0]!.id}>
    <Stage />
  </AccentProvider>,
);
