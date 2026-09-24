import { createRoot } from "react-dom/client";
import { AccentProvider } from "@/ui/accent";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import { Annotate } from "@/primitives/annotate";
import "../../src/index.css";
import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";

document.body.classList.add("lite-student-theme");
for (const [step, value] of Object.entries(LITE_ACCENT_PRESETS[0]!.scale)) {
  document.body.style.setProperty(`--mk-theme-accent-${step}`, value);
}

/**
 * 「诗按诗的样子摆」的看图台。
 *
 * 产品负责人 2026-09-24：「for short poems, maybe we can have a better
 * display. it is always short and it is somekind of old poems need some
 * ancient vibe, maybe we can use a card-like format to show them.」
 *
 * 🚨 为什么必须在真浏览器里看：这块卡片全部是版式。字距、行高、那一列居没
 * 居正、卡片有多宽、段号收没收掉 —— 一条 `getByText` 全部照常通过。
 * AGENTS.md 那条：UI 用真浏览器看。
 *
 * 台上摆四屏，前两屏是**同一首诗**的改前改后：
 *
 *   1. 改前（没有 data-genre）——《江雪》一句一行粘进来，SplitBlocks 只在空行
 *      处切段，所以它**是一段**，段里带着换行。默认的 white-space 把换行折成
 *      空格，二十个字连成一行。这一屏是要拍下来的毛病本身。
 *   2. 改后（data-genre="poem"）—— 同一段文字，四行。
 *   3. 现代诗 —— 行长是作者排出来的。要证明这里是**整列居中**而不是逐行居中，
 *      逐行居中会把它排成另一首诗。
 *   4. 两段的七律 —— 不是每首诗都只有一段。
 */

/** 《江雪》柳宗元。一句一行、中间不空行 —— 服务端切出来是**一段**。 */
const JIANGXUE = [{ id: "b1", text: "千山鸟飞绝，\n万径人踪灭。\n孤舟蓑笠翁，\n独钓寒江雪。" }];

/** 现代诗：行长不齐，而且有作者自己排的缩进。 */
const MODERN = [
  {
    id: "b1",
    text: "我打江南走过\n那等在季节里的容颜如莲花的开落\n　　东风不来，三月的柳絮不飞",
  },
];

/** 两段的一首七律 —— 空行切出两段，卡片里是两组。 */
const SEVEN = [
  { id: "b1", text: "风急天高猿啸哀，\n渚清沙白鸟飞回。" },
  { id: "b2", text: "无边落木萧萧下，\n不尽长江滚滚来。" },
];

const EMPTY = { material_id: "m1", spans: [] };

/** 正文那一栏的真结构：`.mk-lite-room` → article-inner[data-genre] → 题目 + 那块纸。 */
function Article({
  label,
  title,
  genre,
  blocks,
}: {
  label: string;
  title: string;
  genre?: string;
  blocks: { id: string; text: string }[];
}) {
  return (
    <section style={{ marginBottom: 34 }}>
      <p data-case={label} style={{ fontSize: 12, color: "var(--mk-muted)", margin: "0 0 6px" }}>
        {label}
      </p>
      <div className="mk-lite-room" style={{ background: "var(--mk-paper)", padding: "18px 24px", borderRadius: 10 }}>
        <div className="mk-reading-room__article-inner" data-genre={genre}>
          <header className="mk-reading-room__article-header">
            <h2 style={{ fontSize: 19, margin: "0 0 4px", color: "var(--mk-ink)" }}>{title}</h2>
            <div className="mk-reading-room__article-meta">
              <span>{blocks.length} 段 · 课堂讨论材料</span>
            </div>
          </header>
          <div className={genre === "poem" ? "mk-poem-sheet" : undefined}>
            <Annotate blocks={blocks} state={EMPTY} activeSpanId={null} onSelectSpan={() => {}} />
          </div>
        </div>
      </div>
    </section>
  );
}

function Stage() {
  return (
    <div style={{ padding: 26, maxWidth: 940, margin: "0 auto" }}>
      <Article label="改前：没有 data-genre，换行折成空格" title="江雪" blocks={JIANGXUE} />
      <Article label="改后：data-genre=poem" title="江雪" genre="poem" blocks={JIANGXUE} />
      <Article label="现代诗：行长不齐，整列居中而不是逐行居中" title="错误" genre="poem" blocks={MODERN} />
      <Article label="两段的七律" title="登高（节选）" genre="poem" blocks={SEVEN} />
      <Article label="对照：说明文一个像素都不该变" title="城市为什么比郊区热？" blocks={[
        { id: "b1", text: "夏天的傍晚，如果你从市中心骑车回到郊区的家，会明显感到凉快下来。这不是错觉。" },
        { id: "b2", text: "沥青和混凝土白天吸收大量热量，入夜后慢慢放出来，于是城市的夜晚常常比周围的乡野高上好几度。" },
      ]} />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <AccentProvider initialAccent="teal" presets={LITE_ACCENT_PRESETS}>
    <div className="lite-student" style={{ background: "var(--mk-paper)", minHeight: "100vh" }}>
      <Stage />
    </div>
  </AccentProvider>,
);
