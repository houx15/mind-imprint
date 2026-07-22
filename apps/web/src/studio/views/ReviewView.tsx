import { useState } from "react";
import type { GaugeFx, SelfScoreFx, PredictionFx, ReflectionFx, DeclarationFx } from "../state";

export type ReviewViewProps = {
  gauges: GaugeFx[];
  canFinish: boolean;
  finished: boolean;
  finishing: boolean;
  finishError: string | null;
  onFinish: () => void;
  // N2 Task 8: the three post-gauge cards below the 就绪度 grid.
  selfScore: SelfScoreFx;
  prediction: PredictionFx;
  reflection: ReflectionFx;
  onSelfScore: (body: { scores: { code: string; band: number }[] }) => void;
  onReflection: (body: { text: string }) => void;
  // N3f Task 9: the S6 AI 使用申报单 — four counters projected from data that
  // already exists, plus the student's own signature over them.
  declaration: DeclarationFx;
  onSignDeclaration: () => void;
};

const LEVEL_COLOR: Record<GaugeFx["level"], string> = {
  full: "#4C9A82",
  partial: "#D9A23D",
  empty: "#AEB4C2",
};

// Shared idiom for the three new cards below the gauge grid — verbatim the
// existing GaugeCard style (white, #ECEEF3 border, radius 14).
const CARD: React.CSSProperties = { background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "16px 18px" };
const CARD_TITLE: React.CSSProperties = { fontSize: 14.5, fontWeight: 800, color: "#1C2333" };
const CARD_SUB: React.CSSProperties = { fontSize: 12, color: "#8A92A3", margin: "4px 0 12px", lineHeight: 1.6 };
const BODY_LINE: React.CSSProperties = { fontSize: 13, color: "#4A5165", lineHeight: 1.7 };
const NOTE_LINE: React.CSSProperties = { fontSize: 12.5, color: "#6B7384", lineHeight: 1.7, marginTop: 4 };
const CHIP_BASE: React.CSSProperties = {
  fontSize: 12,
  fontWeight: 700,
  padding: "6px 13px",
  borderRadius: 999,
  border: "1px solid #E1E4ED",
  background: "#fff",
  color: "#6B7384",
  cursor: "pointer",
  fontFamily: "inherit",
};
const CHIP_ACTIVE: React.CSSProperties = { border: "1px solid #2A3B7A", background: "#EDEFF9", color: "#2A3B7A" };
const BADGE: React.CSSProperties = {
  flex: "none",
  fontSize: 11.5,
  fontWeight: 800,
  color: "#2A3B7A",
  background: "#EDEFF9",
  padding: "3px 10px",
  borderRadius: 999,
};

function GaugeCard({ g }: { g: GaugeFx }) {
  const color = LEVEL_COLOR[g.level];
  const lamps = Array.from({ length: g.total }, (_, i) => i < g.lit);
  return (
    <div
      data-testid={`gauge-${g.code}`}
      style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "14px 16px" }}
    >
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 800, color }}>{g.code}</span>
          <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{g.name}</span>
        </div>
        <span style={{ fontSize: 11.5, fontWeight: 800, color }}>
          {g.lit}/{g.total}
        </span>
      </div>
      <div style={{ display: "flex", gap: 5, margin: "11px 0 9px" }}>
        {lamps.map((lit, i) => (
          <span
            key={i}
            data-lamp={lit ? "lit" : "empty"}
            style={{ flex: 1, height: 6, borderRadius: 4, background: lit ? color : "#EEF0F5" }}
          />
        ))}
      </div>
      <div style={{ fontSize: 11.5, color: "#8A92A3", lineHeight: 1.55 }}>{g.note}</div>
    </div>
  );
}

// Prediction reveal: what the student called weakest at S0, held up against
// the actual weakest tables once the whole-draft review has run. `overlap`
// is a descriptive count, never a score (RL-3/5) — deliberately not styled
// as a grade.
function PredictionCard({ prediction }: { prediction: PredictionFx }) {
  const predictedNames = prediction.predicted.map((p) => p.name).join("、");
  const actualNames = prediction.actual.map((p) => p.name).join("、");
  return (
    <div style={CARD} data-testid="prediction-card">
      <div style={CARD_TITLE}>开头的预测 vs 实际</div>
      {prediction.predicted.length === 0 ? (
        <div style={{ ...BODY_LINE, marginTop: 8 }} data-testid="prediction-line-predicted">
          你在开头还没有预测最弱项
        </div>
      ) : (
        <div style={{ ...BODY_LINE, marginTop: 8 }} data-testid="prediction-line-predicted">
          开头你预测最弱的是 {predictedNames}
        </div>
      )}
      {prediction.revealed ? (
        <>
          <div style={BODY_LINE} data-testid="prediction-line-actual">
            跑完这轮，评分表上实际最弱的是 {actualNames}
          </div>
          <div style={NOTE_LINE} data-testid="prediction-line-overlap">
            你的预测和实际吻合 {prediction.overlap} 项
          </div>
        </>
      ) : (
        <div style={NOTE_LINE} data-testid="prediction-line-pending">
          跑完整稿体检后，这里会对照实际
        </div>
      )}
    </div>
  );
}

function SelfScoreCard({
  selfScore,
  onSelfScore,
}: {
  selfScore: SelfScoreFx;
  onSelfScore: (body: { scores: { code: string; band: number }[] }) => void;
}) {
  const scoredCount = selfScore.dims.filter((d) => d.band >= 0).length;
  return (
    <div style={CARD} data-testid="self-score-card">
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
        <div style={CARD_TITLE}>先自己评一评</div>
        <span style={BADGE} data-testid="self-score-badge">
          已评 {scoredCount}/{selfScore.dims.length}
        </span>
      </div>
      <div style={CARD_SUB}>对照上面的就绪度，给自己每一块打个档</div>
      {selfScore.dims.map((dim) => (
        <div
          key={dim.code}
          data-testid={`self-score-row-${dim.code}`}
          style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, padding: "8px 0" }}
        >
          <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{dim.name}</span>
          <div style={{ display: "flex", gap: 6, flex: "none" }}>
            {selfScore.bands.map((label, band) => {
              const active = dim.band === band;
              return (
                <button
                  key={band}
                  type="button"
                  data-testid={`self-score-chip-${dim.code}-${band}`}
                  data-active={active ? "true" : "false"}
                  onClick={() => onSelfScore({ scores: [{ code: dim.code, band }] })}
                  style={{ ...CHIP_BASE, ...(active ? CHIP_ACTIVE : {}) }}
                >
                  {label}
                </button>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

function ReflectionCard({
  reflection,
  onReflection,
}: {
  reflection: ReflectionFx;
  onReflection: (body: { text: string }) => void;
}) {
  const [text, setText] = useState(reflection.text);
  const canSubmit = [...text].length >= 20;
  return (
    <div style={CARD} data-testid="reflection-card">
      <div style={CARD_TITLE}>写一段研究回顾</div>
      <div style={CARD_SUB}>这段反思由你自己写——印记只提供问题，不代笔。</div>
      {reflection.prompts.length > 0 && (
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginBottom: 12 }}>
          {reflection.prompts.map((p) => (
            <span
              key={p}
              style={{
                fontSize: 11.5,
                color: "#6B7384",
                background: "#F7F8FC",
                border: "1px solid #EAECF2",
                borderRadius: 999,
                padding: "4px 11px",
              }}
            >
              {p}
            </span>
          ))}
        </div>
      )}
      <textarea
        data-testid="reflection-textarea"
        rows={4}
        value={text}
        onChange={(e) => setText(e.target.value)}
        style={{
          width: "100%",
          border: "1px solid #E1E4ED",
          borderRadius: 12,
          padding: "12px 14px",
          fontSize: 14,
          lineHeight: 1.7,
          color: "#1C2333",
          background: "#fff",
          outline: "none",
          resize: "vertical",
          fontFamily: "inherit",
        }}
      />
      <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 10 }}>
        <button
          type="button"
          data-testid="reflection-submit"
          disabled={!canSubmit}
          onClick={() => onReflection({ text })}
          style={{
            fontSize: 12.5,
            fontWeight: 700,
            color: "#fff",
            background: canSubmit ? "#2A3B7A" : "#B7BBCB",
            border: "none",
            borderRadius: 10,
            padding: "8px 16px",
            cursor: canSubmit ? "pointer" : "not-allowed",
            fontFamily: "inherit",
          }}
        >
          记下我的反思
        </button>
      </div>
    </div>
  );
}

// AI 使用申报单 (Task 9): a factual record of what happened, not a scoreboard
// — 铁律 2 means none of the four counters (including "AI 代写正文: 0 次")
// may read as praise or an achievement, so the styling below stays flat and
// descriptive at every state, unsigned or signed. Design binding:
// docs/design/思维印记_工作区.dc.html:1211-1223 (card/pill/grid) and
// :2179-2181 (the four labels/values, verbatim).
const DECL_ROW: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  gap: 8,
  background: "#fff",
  border: "1px solid #F0E6D2",
  borderRadius: 10,
  padding: "10px 13px",
};
const DECL_LABEL: React.CSSProperties = { fontSize: 12.5, color: "#5C4A22" };
const DECL_VALUE: React.CSSProperties = { fontSize: 13, fontWeight: 800, color: "#8A6520" };

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#8A6520" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

function DeclarationCard({
  declaration,
  onSignDeclaration,
}: {
  declaration: DeclarationFx;
  onSignDeclaration: () => void;
}) {
  const rows: { k: string; v: string }[] = [
    { k: "提问 / 追问", v: `${declaration.asks} 次` },
    { k: "三键处置（接受/改/拒）", v: `${declaration.dispositions} 次` },
    { k: "工具卡调用（自发/提示后）", v: `${declaration.cardsSpontaneous} / ${declaration.cardsPrompted}` },
    { k: "AI 代写正文", v: `${declaration.aiWrittenProse} 次` },
  ];
  return (
    <div style={{ background: "#FBF7EF", border: "1px solid #F0E6D2", borderRadius: 16, padding: "20px 22px" }} data-testid="declaration-card">
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginBottom: 12 }}>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#8A6520" }}>AI 使用申报单</div>
        <span
          data-testid="declaration-pill"
          style={{ fontSize: 11, fontWeight: 700, color: "#B8892F", background: "#F6ECD6", padding: "3px 10px", borderRadius: 999 }}
        >
          {declaration.signed ? "已签名" : "自动生成 · 待你签名"}
        </span>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 10 }}>
        {rows.map((r) => (
          <div key={r.k} style={DECL_ROW}>
            <span style={DECL_LABEL}>{r.k}</span>
            <span style={DECL_VALUE}>{r.v}</span>
          </div>
        ))}
      </div>
      {declaration.signed ? (
        <div style={{ marginTop: 14, fontSize: 12, color: "#8A6520" }} data-testid="declaration-signed-note">
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <CheckIcon />
            这份申报单已由你签署，记录已留存
          </span>
        </div>
      ) : (
        <div style={{ marginTop: 14, display: "flex", justifyContent: "flex-end" }}>
          <button
            type="button"
            data-testid="declaration-sign"
            onClick={onSignDeclaration}
            style={{
              fontSize: 12.5,
              fontWeight: 700,
              color: "#8A6520",
              background: "#fff",
              border: "1px solid #E4CFA0",
              borderRadius: 10,
              padding: "8px 16px",
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            确认签署
          </button>
        </div>
      )}
    </div>
  );
}

export function ReviewView({
  gauges,
  canFinish,
  finished,
  finishing,
  finishError,
  onFinish,
  selfScore,
  prediction,
  reflection,
  onSelfScore,
  onReflection,
  declaration,
  onSignDeclaration,
}: ReviewViewProps) {
  const litSum = gauges.reduce((n, g) => n + g.lit, 0);
  const totalSum = gauges.reduce((n, g) => n + g.total, 0);
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" }}>
      <div style={{ maxWidth: 720, margin: "0 auto" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, marginBottom: 8 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 7 }}>
            <div style={{ fontSize: 17, fontWeight: 800, color: "#1C2333" }}>就绪度</div>
            <span
              title="就绪度显示你的草稿现在落在评分表的哪一格，用来定位下一步，不是预估分数"
              style={{
                width: 16, height: 16, borderRadius: "50%", background: "#EEF0F5", color: "#9AA1B0",
                fontSize: 11, fontWeight: 700, display: "flex", alignItems: "center",
                justifyContent: "center", cursor: "help",
              }}
            >
              ?
            </span>
          </div>
          <div style={{ fontSize: 13.5, fontWeight: 700, color: "#6B7384" }}>
            已点亮 {litSum}/{totalSum} 格
          </div>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 12, marginTop: 14 }}>
          {gauges.map((g) => (
            <GaugeCard key={g.code} g={g} />
          ))}
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 12, marginTop: 20 }}>
          <PredictionCard prediction={prediction} />
          <SelfScoreCard selfScore={selfScore} onSelfScore={onSelfScore} />
          <ReflectionCard reflection={reflection} onReflection={onReflection} />
          <DeclarationCard declaration={declaration} onSignDeclaration={onSignDeclaration} />
        </div>
        {finished ? (
          <div style={{ marginTop: 22, textAlign: "center", fontSize: 13.5, fontWeight: 700, color: "#4C9A82" }}>已归档 · 成长报告已生成</div>
        ) : canFinish ? (
          <div style={{ marginTop: 22, display: "flex", flexDirection: "column", alignItems: "center", gap: 10 }}>
            <button
              type="button"
              onClick={onFinish}
              disabled={finishing}
              style={{ display: "inline-flex", alignItems: "center", gap: 8, background: finishing ? "#9CB6A9" : "#4C9A82", color: "#fff", border: "none", fontSize: 14.5, fontWeight: 700, padding: "13px 26px", borderRadius: 12, cursor: finishing ? "default" : "pointer", fontFamily: "inherit" }}
            >
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              {finishing ? "正在归档…" : "完成任务 · 归档"}
            </button>
            {finishError && <div style={{ fontSize: 12.5, color: "#B0432E" }}>{finishError}</div>}
          </div>
        ) : null}
      </div>
    </div>
  );
}
