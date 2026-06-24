/**
 * InnerPartsTeaching — 内在小人 教学模块（emotional-alignment）
 * 3 chapters: energy | cast | closing
 * All interaction state is LOCAL to each chapter component; no envelope writes.
 * Ported from .superpowers/brainstorm/55230-1782285283/content/inner-parts-cast.html
 */
import { useState } from "react";
import type { TeachingModule } from "../types";
import { Battery } from "../assets/Battery";
import { INNER_PARTS } from "../assets/InnerPartsCast";

/* ────────────────────────────────────────────────────────────────
   Chapter 1: Energy — Battery + click/drag level 1-5
   ────────────────────────────────────────────────────────────── */

function EnergyChapter() {
  const [level, setLevel] = useState(2);
  const isLow = level <= 2;

  const copyForLevel = (l: number): string => {
    if (l === 1) return "1 格电……先承认这一点，这本身就是勇气";
    if (l === 2) return "有点没电，先承认这一点";
    if (l === 3) return "还行，带着这个状态继续";
    if (l === 4) return "状态不错，可以出发了";
    return "满格！今天能量充足";
  };

  return (
    <div className="space-y-5">
      <div>
        <h3 className="text-sm font-semibold text-[#1C2333] mb-0.5">今天的情绪电量</h3>
        <p className="text-xs text-[#9AA1B0] mb-4">先不聊任务，拨一下电量</p>
        <div className="flex items-center gap-4">
          <Battery level={level} max={5} size={60} />
          <span
            className="text-xs font-medium"
            style={{ color: isLow ? "#C2557A" : "#C9743C" }}
          >
            {level} / 5 格
          </span>
        </div>
        {/* Slider to set level */}
        <input
          type="range"
          min={1}
          max={5}
          step={1}
          value={level}
          onChange={(e) => setLevel(Number(e.target.value))}
          className="w-full mt-4 h-2 rounded-full cursor-pointer"
          style={{ accentColor: isLow ? "#C2557A" : "#C9743C" }}
          aria-label={`情绪电量：${level} / 5 格`}
        />
        <div className="flex justify-between text-[10px] text-[#9AA1B0] mt-1">
          {[1, 2, 3, 4, 5].map((n) => (
            <span key={n}>{n}</span>
          ))}
        </div>
      </div>

      <div
        className="rounded-xl p-4 text-sm"
        style={{
          background: isLow ? "#FBE7EF" : "#FBEBDD",
          border: `1px solid ${isLow ? "#F4C6D8" : "#ECD0B4"}`,
          color: isLow ? "#7A2E42" : "#7A4A1C",
        }}
      >
        「{copyForLevel(level)}」
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 2: Cast — 6 character cards; click to flip
   ────────────────────────────────────────────────────────────── */

function CastChapter() {
  const [flippedKey, setFlippedKey] = useState<string | null>(null);

  const handleCardClick = (key: string) => {
    setFlippedKey((prev) => (prev === key ? null : key));
  };

  return (
    <div className="space-y-4">
      <p className="text-xs text-[#6B7384] leading-relaxed">
        今天哪个小人最在场？点一下翻面，看它的保护意图。
      </p>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: 10,
        }}
      >
        {INNER_PARTS.map((part) => {
          const isFlipped = flippedKey === part.key;
          return (
            /* Card wrapper — a div, not a button, so it doesn't swallow child text into its own textContent match  */
            <div
              key={part.key}
              onClick={() => handleCardClick(part.key)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") handleCardClick(part.key);
              }}
              aria-pressed={isFlipped}
              aria-label={
                isFlipped
                  ? `${part.name}，点击收起`
                  : `${part.name}，点击看它在替你挡什么`
              }
              style={{
                border: isFlipped
                  ? "2px solid #2A3B7A"
                  : "2px solid #EAECF2",
                borderRadius: 14,
                background: isFlipped ? "#EBF0FF" : "#fff",
                padding: "11px 11px 12px",
                cursor: "pointer",
                textAlign: "center",
                transition: "border-color .15s, box-shadow .15s, transform .12s",
                boxShadow: isFlipped
                  ? "0 8px 22px rgba(42,59,122,.16)"
                  : undefined,
              }}
            >
              {!isFlipped ? (
                /* Front face */
                <>
                  <part.Avatar size={56} />
                  <div
                    style={{
                      fontSize: 12.5,
                      fontWeight: 700,
                      color: "#1C2333",
                      marginTop: 6,
                    }}
                  >
                    {part.name}
                  </div>
                  <div
                    style={{
                      fontSize: 10,
                      color: "#6B7384",
                      lineHeight: 1.4,
                      marginTop: 3,
                    }}
                  >
                    {part.quote}
                  </div>
                  <div
                    style={{
                      fontSize: 9.5,
                      color: "#2A3B7A",
                      marginTop: 7,
                      opacity: 0.8,
                    }}
                  >
                    点一下看背面 ↺
                  </div>
                </>
              ) : (
                /* Back face — reveals what this part protects */
                <>
                  <div
                    style={{
                      fontSize: 12.5,
                      fontWeight: 700,
                      color: "#2A3B7A",
                      marginBottom: 8,
                    }}
                  >
                    {part.name}
                  </div>
                  <p
                    style={{
                      fontSize: 11,
                      color: "#3A4256",
                      lineHeight: 1.5,
                      padding: "6px 4px",
                      margin: 0,
                    }}
                  >
                    {part.protects}
                  </p>
                  <div
                    style={{
                      fontSize: 9.5,
                      color: "#4C9A82",
                      marginTop: 8,
                    }}
                  >
                    它不是缺点，是一个想护住你的声音
                  </div>
                </>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Chapter 3: Closing — 安全岛 textarea + 3-min micro-action buttons
   ────────────────────────────────────────────────────────────── */

function ClosingChapter() {
  const [island, setIsland] = useState("");
  const [action, setAction] = useState<"try" | "done" | null>(null);

  return (
    <div className="space-y-5">
      {/* 安全岛 */}
      <div>
        <h3 className="text-sm font-semibold text-[#1C2333] mb-1">安全岛</h3>
        <p className="text-xs text-[#9AA1B0] mb-3">
          有什么想对自己说的？留空也 OK，今天到此也被尊重。
        </p>
        <textarea
          value={island}
          onChange={(e) => setIsland(e.target.value)}
          placeholder="想写就写，这里只有你自己看……（可以留空）"
          rows={3}
          style={{
            width: "100%",
            borderRadius: 12,
            border: "1px solid #EAECF2",
            padding: "10px 12px",
            fontSize: 13,
            color: "#1C2333",
            background: "#FAFAFA",
            resize: "vertical",
            outline: "none",
            boxSizing: "border-box",
          }}
          aria-label="安全岛，可选填内容"
        />
        {island.length > 0 && (
          <p className="text-xs text-[#4C9A82] mt-1.5">已记下。</p>
        )}
      </div>

      {/* 3-min micro-action */}
      <div>
        <h3 className="text-sm font-semibold text-[#1C2333] mb-1">3 分钟小行动</h3>
        <p className="text-xs text-[#9AA1B0] mb-3">两个选择都被尊重。</p>
        <div className="flex gap-3">
          <button
            onClick={() => setAction("try")}
            style={{
              flex: 1,
              borderRadius: 12,
              padding: "10px 12px",
              fontSize: 13,
              fontWeight: 600,
              cursor: "pointer",
              border: action === "try" ? "2px solid #4C9A82" : "2px solid #EAECF2",
              background: action === "try" ? "#E7F3EE" : "#fff",
              color: action === "try" ? "#2C5B4C" : "#3A4256",
              transition: "all .15s",
            }}
          >
            试一下 →
          </button>
          <button
            onClick={() => setAction("done")}
            style={{
              flex: 1,
              borderRadius: 12,
              padding: "10px 12px",
              fontSize: 13,
              fontWeight: 600,
              cursor: "pointer",
              border: action === "done" ? "2px solid #2A3B7A" : "2px solid #EAECF2",
              background: action === "done" ? "#EBF0FF" : "#fff",
              color: action === "done" ? "#2A3B7A" : "#3A4256",
              transition: "all .15s",
            }}
          >
            今天到这里
          </button>
        </div>

        {action === "try" && (
          <p className="text-xs text-[#4C9A82] mt-2 leading-relaxed">
            很好。哪怕只是打开那份文件，也算开始了。
          </p>
        )}
        {action === "done" && (
          <p className="text-xs text-[#2A3B7A] mt-2 leading-relaxed">
            今天到此也被尊重。认识这些小人，已经是一步了。
          </p>
        )}
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────────
   Module export
   ────────────────────────────────────────────────────────────── */

export const InnerPartsTeaching: TeachingModule = {
  cardId: "emotional-alignment",
  title: "内在小人 — 情感对齐",
  category: "情绪管理",
  chapters: [
    { key: "energy",  label: "今天的情绪电量",         Component: EnergyChapter },
    { key: "cast",    label: "认识六个内在小人",         Component: CastChapter },
    { key: "closing", label: "安全岛 & 小行动",          Component: ClosingChapter },
  ],
};
