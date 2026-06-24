/**
 * SiftCraapTeaching — 5-chapter interactive teaching module for SIFT×CRAAP.
 * All interaction state is LOCAL to each chapter component; no envelope writes.
 * Ported from .superpowers/brainstorm/55230-1782285283/content/sift-interactions.html
 */
import { useState } from "react";
import type { TeachingModule } from "../types";
import { StopIcon, InvestigateIcon, FindIcon, TraceIcon, CraapIcon } from "../assets/SiftIcons";
import { Radar } from "../assets/Radar";
import { SourceChip } from "../assets/SourceChip";

/* ────────────────────────────────────────────────────────────────
   Shared mini-layout primitives (no envelope, no store)
   ────────────────────────────────────────────────────────────── */

function ChapterLayout({ icon, iconBg, children }: {
  icon: React.ReactNode;
  iconBg: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex gap-4 items-start">
      <div
        className="rounded-2xl flex-none flex items-center justify-center"
        style={{ width: 80, height: 80, background: iconBg }}
      >
        {icon}
      </div>
      <div className="flex-1 min-w-0">{children}</div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 1: Stop — breathing circle + reflective input
   ────────────────────────────────────────────────────────────── */

function StopChapter() {
  const [text, setText] = useState("");

  return (
    <div className="space-y-5">
      <ChapterLayout icon={<StopIcon size={50} />} iconBg="#FBEBDD">
        <p className="text-sm text-[#6B7384] leading-relaxed mb-3">
          在采信任何信息之前，先深呼一口气。停下来，问自己一个问题。
        </p>
        <div
          className="rounded-xl p-3 text-sm text-[#3A4256]"
          style={{ border: "1px dashed #C9743C" }}
        >
          <span className="font-medium">你打算用这条信息<strong>说明什么？</strong></span>
          <input
            type="text"
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="输入你的想法…"
            className="block w-full mt-2 bg-transparent border-none outline-none text-sm placeholder:text-[#9AA1B0] text-[#1C2333]"
            aria-label="输入你打算用这条信息说明什么"
          />
        </div>
        {text.length > 0 && (
          <p className="mt-2 text-xs text-[#4C9A82]">
            很好！带着这个目的去核查信息来源。
          </p>
        )}
      </ChapterLayout>

      {/* Breathing circle animation */}
      <div className="flex justify-center py-2">
        <div className="breathing-circle" aria-hidden="true" />
        <style>{`
          .breathing-circle {
            width: 64px; height: 64px;
            border-radius: 50%;
            background: radial-gradient(circle, #F2C9A6 40%, #FBEBDD 100%);
            border: 2px solid #B5632F;
            animation: breathe 3.5s ease-in-out infinite;
          }
          @keyframes breathe {
            0%, 100% { transform: scale(1); opacity: 0.7; }
            50% { transform: scale(1.28); opacity: 1; }
          }
        `}</style>
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 2: Investigate — click-to-classify source chips
   ────────────────────────────────────────────────────────────── */

type Verdict = "ok" | "q" | "bad";

const SOURCES: { name: string; correctVerdict: Verdict; defaultVerdict: Verdict }[] = [
  { name: "NASA 官方报告", correctVerdict: "ok", defaultVerdict: "q" },
  { name: "某门户转载", correctVerdict: "q", defaultVerdict: "q" },
  { name: "养生号推文", correctVerdict: "bad", defaultVerdict: "q" },
  { name: "Nature Sustainability", correctVerdict: "ok", defaultVerdict: "q" },
  { name: "知乎匿名回答", correctVerdict: "bad", defaultVerdict: "q" },
];

const VERDICT_CYCLE: Verdict[] = ["q", "ok", "bad"];

function InvestigateChapter() {
  const [verdicts, setVerdicts] = useState<Verdict[]>(
    SOURCES.map((s) => s.defaultVerdict)
  );

  const cycleVerdict = (i: number) => {
    setVerdicts((prev) => {
      const next: Verdict[] = [...prev];
      const current: Verdict = prev[i] ?? "q";
      const idx = VERDICT_CYCLE.indexOf(current);
      next[i] = VERDICT_CYCLE[(idx + 1) % VERDICT_CYCLE.length] as Verdict;
      return next;
    });
  };

  const allCorrect = verdicts.every((v, i) => v === SOURCES[i]?.correctVerdict);

  return (
    <div className="space-y-4">
      <ChapterLayout icon={<InvestigateIcon size={50} />} iconBg="#EBF0FF">
        <p className="text-sm text-[#6B7384] leading-relaxed mb-3">
          点击每个来源，在 <strong>可信 ✓</strong> / <strong>存疑 ?</strong> / <strong>不可信 ✕</strong> 之间切换。
        </p>
        <div className="flex flex-wrap gap-2">
          {SOURCES.map((src, i) => (
            <button
              key={src.name}
              onClick={() => cycleVerdict(i)}
              className="cursor-pointer select-none transition-transform hover:scale-105 active:scale-95"
              aria-label={`${src.name}，当前：${verdicts[i] === "ok" ? "可信" : verdicts[i] === "q" ? "存疑" : "不可信"}，点击切换`}
            >
              <SourceChip name={src.name} verdict={verdicts[i] ?? "q"} />
            </button>
          ))}
        </div>
        {allCorrect && (
          <p className="mt-3 text-xs text-[#4C9A82] font-medium">
            全对！学术来源和官方机构可信度最高。
          </p>
        )}
      </ChapterLayout>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 3: Find — weak source vs Nature side-by-side reveal
   ────────────────────────────────────────────────────────────── */

function FindChapter() {
  const [revealed, setRevealed] = useState(false);

  return (
    <div className="space-y-4">
      <ChapterLayout icon={<FindIcon size={50} />} iconBg="#E7F3EE">
        <p className="text-sm text-[#6B7384] leading-relaxed mb-3">
          同一个话题，不同来源的说法差异很大。
        </p>
        <div className="flex gap-3">
          {/* Weak source */}
          <div
            className="flex-1 rounded-xl border p-3 text-xs text-[#3A4256]"
            style={{ borderColor: "#EAECF2", background: "#FAFAFA" }}
          >
            <div className="text-[10px] font-semibold text-[#C9743C] mb-1">普通博客</div>
            <p className="leading-relaxed">「中国是全球最可持续发展的国家」</p>
            <p className="text-[#9AA1B0] mt-1">无数据，无引用，无作者信息</p>
          </div>

          {/* Better source — revealed on click */}
          {revealed ? (
            <div
              className="flex-1 rounded-xl border-2 p-3 text-xs text-[#2C5B4C] transition-all duration-300"
              style={{ borderColor: "#4C9A82", background: "#E7F3EE" }}
            >
              <div className="text-[10px] font-semibold text-[#4C9A82] mb-1">Nature Sustainability ✓</div>
              <p className="leading-relaxed">按维度分析：可再生能源↑ / 人均碳排放↑ / 生物多样性↓</p>
              <p className="text-[#4C9A82] mt-1">同行评审，引用 40+ 研究</p>
            </div>
          ) : (
            <button
              onClick={() => setRevealed(true)}
              className="flex-1 rounded-xl border-2 border-dashed p-3 text-xs text-[#2A3B7A] hover:bg-[#EBF0FF] transition-colors flex flex-col items-center justify-center gap-1"
              style={{ borderColor: "#C9D2F0" }}
            >
              <span className="text-lg">🔍</span>
              <span className="font-medium">看更权威版本</span>
            </button>
          )}
        </div>
      </ChapterLayout>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 4: Trace — share-chain peel-back interaction
   ────────────────────────────────────────────────────────────── */

const CHAIN = [
  { label: "朋友圈转发", bg: "#EAECF2", color: "#6B7384" },
  { label: "公众号文章", bg: "#EAECF2", color: "#6B7384" },
  { label: "门户转载", bg: "#EAECF2", color: "#6B7384" },
  { label: "原始论文 ✓", bg: "#E7F3EE", color: "#4C9A82" },
];

function TraceChapter() {
  // peeled = how many layers have been removed from the front (0 = full chain shown)
  const [peeled, setPeeled] = useState(0);
  const atOrigin = peeled >= CHAIN.length - 1;

  const handlePeel = () => {
    if (!atOrigin) setPeeled((p) => p + 1);
  };

  const visibleChain = CHAIN.slice(peeled);

  return (
    <div className="space-y-4">
      <ChapterLayout icon={<TraceIcon size={50} />} iconBg="#FBE7EF">
        <p className="text-sm text-[#6B7384] leading-relaxed mb-3">
          顺着转发链溯源，找到最初的出处。点击剥去一层。
        </p>
        <div className="flex items-center flex-wrap gap-1 mb-3">
          {visibleChain.map((node, i) => (
            <span key={node.label} className="flex items-center gap-1">
              {i > 0 && <span className="text-[#9AA1B0] text-xs">→</span>}
              <span
                className="rounded-md px-2 py-1 text-xs font-medium"
                style={{ background: node.bg, color: node.color }}
              >
                {node.label}
              </span>
            </span>
          ))}
        </div>

        {!atOrigin ? (
          <button
            onClick={handlePeel}
            className="px-3 py-1.5 rounded-lg text-xs font-medium text-[#C2557A] hover:bg-[#FBE7EF] transition-colors"
            style={{ border: "1px solid #F4C6D8" }}
          >
            剥去一层转发 →
          </button>
        ) : (
          <p className="text-xs text-[#4C9A82] font-medium">
            到达原始来源！这才是可以引用的出处。
          </p>
        )}
      </ChapterLayout>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 5: CRAAP — 5 sliders driving the Radar (advanced)
   ────────────────────────────────────────────────────────────── */

const CRAAP_LABELS = ["时效", "相关", "权威", "准确", "目的"];
const CRAAP_DESCRIPTIONS = [
  "信息是否最新？",
  "与你的论点相关？",
  "来源权威可靠？",
  "内容准确可核实？",
  "发布目的是什么？",
];

function CraapChapter() {
  const [values, setValues] = useState<number[]>([3, 3, 3, 3, 3]);

  const setVal = (i: number, v: number) => {
    setValues((prev) => {
      const next = [...prev];
      next[i] = v;
      return next;
    });
  };

  const avg = values.reduce((a, b) => a + b, 0) / values.length;

  return (
    <div className="space-y-4">
      <div className="flex items-start gap-3 mb-1">
        <div
          className="rounded-2xl flex-none flex items-center justify-center"
          style={{ width: 88, height: 88, background: "#EBF0FF" }}
        >
          <CraapIcon size={58} />
        </div>
        <div className="flex-1">
          <p className="text-sm text-[#6B7384] leading-relaxed">
            用五个维度为你的来源「做体检」。每动一杆，右边的形状跟着变。
          </p>
          <p className="text-xs text-[#9AA1B0] mt-1">
            均分 {avg.toFixed(1)} / 5
            {avg >= 4 ? " — 高质量来源 ✓" : avg >= 2.5 ? " — 尚可，注意存疑点" : " — 建议换一个来源"}
          </p>
        </div>
      </div>

      <div className="flex gap-4 items-start">
        {/* Sliders */}
        <div className="flex-1 space-y-3">
          {CRAAP_LABELS.map((label, i) => (
            <div key={label} className="space-y-1">
              <div className="flex justify-between text-xs">
                <span className="font-medium text-[#3A4256]">{label}</span>
                <span className="text-[#9AA1B0]">{CRAAP_DESCRIPTIONS[i]}</span>
                <span className="font-semibold text-[#2A3B7A] w-4 text-right">{values[i]}</span>
              </div>
              <input
                type="range"
                min={0}
                max={5}
                step={1}
                value={values[i]}
                onChange={(e) => setVal(i, Number(e.target.value))}
                className="w-full h-1.5 rounded-full accent-[#2A3B7A] cursor-pointer"
                aria-label={`${label}：${values[i]} / 5`}
              />
            </div>
          ))}
        </div>

        {/* Live Radar */}
        <div className="flex-none">
          <Radar values={values} labels={CRAAP_LABELS} />
        </div>
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Module export + registry registration
   ────────────────────────────────────────────────────────────── */

export const SiftCraapTeaching: TeachingModule = {
  cardId: "sift_craap",
  title: "SIFT×CRAAP 信息核查",
  category: "信息素养",
  chapters: [
    { key: "stop",        label: "先停一下",          Component: StopChapter },
    { key: "investigate", label: "查这是谁说的",       Component: InvestigateChapter },
    { key: "find",        label: "找更权威的版本",     Component: FindChapter },
    { key: "trace",       label: "溯到原始出处",       Component: TraceChapter },
    { key: "craap",       label: "给来源做体检",       badge: "★ 进阶", Component: CraapChapter },
  ],
};
