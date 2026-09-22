import { useMemo, useState } from "react";

import type { EnergyProfile, TalentState } from "../../api/awakening";
import {
  ENERGY,
  ENERGY_DOMAINS,
  ENERGY_STAGES,
  GUIDES,
  NAVIGATOR,
  TALENT,
  TALENT_CARDS,
  TALENT_LANES,
  TALENT_PICK_COUNT,
} from "../content";
import { IMAGES } from "../assets";
import { Choice, Dim, Eyebrow, Ghost, Meter, Panel, Primary, Stage } from "../ui";

/**
 * scenes/Cards —— 能量卡牌、印记助手、天赋卡牌。
 *
 * 三屏都是**用手摆的板**：卡铺开，点一下就选中，不用下拉框、不用滑块
 * （memory: interaction-means-a-board-2026-09-04、no-native-form-controls）。
 * 工具一打开就铺满这块面板，所以这三屏用 `Stage wide`。
 */

/* ── 能量卡牌 ───────────────────────────────────────────────────────────── */

/**
 * 从四组选择算出她的能量方向。
 *
 * 纯函数，导出是为了测得到 —— 「同分时排序是否稳定」「一张都没选时给什么」
 * 这两件事读代码看不出来。
 */
export function scoreEnergy(picked: Record<string, string[]>): EnergyProfile {
  const score: Record<string, number> = {};
  for (const stage of ENERGY_STAGES) {
    for (const id of picked[stage.id] ?? []) {
      const card = stage.cards.find((c) => c.id === id);
      if (!card) continue;
      score[card.domain] = (score[card.domain] ?? 0) + 1;
    }
  }
  const domains = Object.entries(score)
    // 分数降序；同分按方向 id 排，所以两次算出来的顺序一样。
    .sort((a, b) => (b[1] - a[1]) || a[0].localeCompare(b[0]))
    .map(([id, n]) => ({
      id,
      name: ENERGY_DOMAINS[id]?.name ?? id,
      short: ENERGY_DOMAINS[id]?.short ?? "",
      score: n,
    }));
  // 一张都没选是合法的：这一屏没有必答要求。那时 focus 为空，
  // 终端的第一问就回到它的默认问法。
  const top = domains.slice(0, 2).map((d) => `${d.name}（${d.short}）`);
  return { focus: top.join("、"), domains };
}

export function EnergyScene({
  onDone,
  /** 走完那一下按钮上的字。默认是第一趟里的下一步（选印记助手）；从复访入口
   *  进来时调用方换成「返回入口」—— 按下去回哪里，上面就得写哪里。 */
  doneLabel = ENERGY.toNavigator,
}: {
  onDone: (profile: EnergyProfile) => void;
  doneLabel?: string;
}) {
  const [i, setI] = useState(0);
  const [picked, setPicked] = useState<Record<string, string[]>>({});
  const [result, setResult] = useState<EnergyProfile | null>(null);
  const stage = ENERGY_STAGES[i]!;
  const mine = picked[stage.id] ?? [];

  const toggle = (id: string) =>
    setPicked((p) => {
      const cur = p[stage.id] ?? [];
      return { ...p, [stage.id]: cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id] };
    });

  if (result) {
    return (
      <Stage wide>
        <Eyebrow>{ENERGY.eyebrow}</Eyebrow>
        <h1 className="mt-3 text-[28px] font-semibold">{ENERGY.resultTitle}</h1>
        <div className="mt-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {result.domains?.length ? (
            result.domains.map((d) => (
              <div key={d.id} className="awk-soft p-5">
                <div className="flex items-baseline justify-between gap-3">
                  <span className="font-semibold">{d.name}</span>
                  <span className="font-mono text-[13px]" style={{ color: "var(--cyan)" }}>
                    {d.score}
                  </span>
                </div>
                <div className="awk-dim mt-1 text-[13px]">{d.short}</div>
              </div>
            ))
          ) : (
            <p className="awk-dim text-[14px]">这一轮你没有选中任何一张。这不影响后面的探询。</p>
          )}
        </div>
        <div className="mt-8">
          <Primary onClick={() => onDone(result)}>{doneLabel}</Primary>
        </div>
      </Stage>
    );
  }

  return (
    <Stage wide>
      <div className="flex items-center justify-between gap-4">
        <div>
          <Eyebrow>{stage.code}</Eyebrow>
          <h1 className="mt-2 text-[26px] font-semibold">{ENERGY.title}</h1>
        </div>
        <span className="awk-energy-progress">第 {i + 1} / {ENERGY_STAGES.length} 组<Meter total={ENERGY_STAGES.length} done={i + 1} /></span>
      </div>
      <p className="awk-dim mt-2 text-[14px]">{ENERGY.lead}</p>

      <Panel className="mt-6 awk-energy-board">
        <p className="text-[16px] font-semibold leading-relaxed">{stage.ask}</p>
        <p className="awk-energy-hint">可多选 · 点击选中，再次点击取消</p>
        <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {stage.cards.map((c) => (
            <Choice
              key={c.id}
              title={c.label}
              selected={mine.includes(c.id)}
              onClick={() => toggle(c.id)}
            />
          ))}
        </div>
        <div className="mt-6 flex items-center gap-4">
          <Primary
            onClick={() => {
              if (i + 1 < ENERGY_STAGES.length) {
                setI(i + 1);
                return;
              }
              setResult(scoreEnergy(picked));
            }}
          >
            {i + 1 < ENERGY_STAGES.length ? ENERGY.next : ENERGY.finish}
          </Primary>
          <Dim>
            <span className="font-mono text-[12px]">已选 {mine.length} 张</span>
          </Dim>
        </div>
      </Panel>
    </Stage>
  );
}

/* ── 印记助手 ───────────────────────────────────────────────────────────── */

export function NavigatorScene({
  navigator,
  onPick,
  onConfirm,
}: {
  navigator: string;
  onPick: (id: string) => void;
  onConfirm: () => void;
}) {
  return (
    <Stage wide>
      <div className="flex flex-wrap items-center gap-6">
        {/* 印记的形象。装饰，所以 alt 为空、加载失败就不占位。 */}
        <img
          src={IMAGES.guide}
          alt=""
          loading="lazy"
          className="h-28 w-28 flex-none rounded-full object-cover"
          style={{ border: "1px solid var(--line)" }}
          onError={(e) => {
            e.currentTarget.style.display = "none";
          }}
        />
        <div className="min-w-0">
          <Eyebrow>{NAVIGATOR.eyebrow}</Eyebrow>
          <h1 className="mt-2 text-[28px] font-semibold">{NAVIGATOR.title}</h1>
          <p className="awk-dim mt-2 text-[15px]">{NAVIGATOR.lead}</p>
        </div>
      </div>

      <div className="mt-7 grid gap-4 md:grid-cols-3">
        {GUIDES.map((g) => {
          const on = navigator === g.id;
          return (
            <button
              key={g.id}
              type="button"
              className="awk-card awk-guide"
              aria-pressed={on ? "true" : "false"}
              onClick={() => onPick(g.id)}
              /* 颜色一直在，不只在选中时 —— 她是先看见三个颜色，才挑其中一个的。
                 进了终端之后整屏用的就是这个颜色。 */
              style={{
                ["--awk-guide" as string]: g.accent,
                ...(on
                  ? { borderColor: g.accent, boxShadow: `0 0 0 1px ${g.accent} inset` }
                  : {}),
              }}
            >
              <span className="block font-mono text-[11px] tracking-[0.18em]" style={{ color: g.accent }}>
                {g.id}
              </span>
              <span className="mt-2 block text-[18px] font-semibold">{g.zh}</span>
              <span className="awk-dim mt-1 block text-[12px]">{g.label}</span>
              <span className="mt-3 block text-[13px] leading-relaxed">{g.body}</span>
              <span className="awk-dim mt-3 block text-[13px] italic leading-relaxed">「{g.quote}」</span>
            </button>
          );
        })}
      </div>

      <div className="mt-8">
        <Primary disabled={navigator === ""} onClick={onConfirm}>
          {NAVIGATOR.confirm}
        </Primary>
      </div>
    </Stage>
  );
}

/* ── 天赋卡牌 ───────────────────────────────────────────────────────────── */

export function TalentScene({
  onDone,
}: {
  onDone: (talent: TalentState) => void;
}) {
  const [selected, setSelected] = useState<string[]>([]);
  const [lanes, setLanes] = useState<Record<string, string[]>>({
    energy: [],
    learned: [],
    latent: [],
  });
  const [sorting, setSorting] = useState(false);

  const placed = useMemo(
    () => new Set([...lanes.energy!, ...lanes.learned!, ...lanes.latent!]),
    [lanes],
  );
  const allPlaced = selected.every((id) => placed.has(id));

  const toggle = (id: string) =>
    setSelected((s) => {
      if (s.includes(id)) return s.filter((x) => x !== id);
      if (s.length >= TALENT_PICK_COUNT) return s;
      return [...s, id];
    });

  const place = (id: string, lane: string) =>
    setLanes((l) => {
      const cleared = Object.fromEntries(
        Object.entries(l).map(([k, v]) => [k, v.filter((x) => x !== id)]),
      ) as Record<string, string[]>;
      return { ...cleared, [lane]: [...(cleared[lane] ?? []), id] };
    });

  if (!sorting) {
    return (
      <Stage wide>
        <Eyebrow>{TALENT.eyebrow}</Eyebrow>
        <h1 className="mt-3 text-[28px] font-semibold">{TALENT.title}</h1>
        <p className="awk-dim mt-2 text-[15px] leading-relaxed">{TALENT.lead}</p>

        <div className="mt-7 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
          {TALENT_CARDS.map((c) => {
            const on = selected.includes(c.id);
            return (
              <button
                key={c.id}
                type="button"
                className="awk-card"
                aria-pressed={on ? "true" : "false"}
                onClick={() => toggle(c.id)}
                style={on ? { borderColor: c.accent, boxShadow: `0 0 0 1px ${c.accent} inset` } : undefined}
              >
                <span className="text-[24px] font-semibold" style={{ color: c.accent }}>
                  {c.mark}
                </span>
                <span className="mt-2 block font-semibold">{c.title}</span>
                <span className="awk-dim mt-1 block text-[12px] leading-relaxed">{c.body}</span>
                <span className="awk-dim mt-2 block font-mono text-[11px]">{c.axis}</span>
              </button>
            );
          })}
        </div>

        <div className="mt-7 flex items-center gap-4">
          <Primary disabled={selected.length !== TALENT_PICK_COUNT} onClick={() => setSorting(true)}>
            {TALENT.toSort}
          </Primary>
          <Dim>
            <span className="font-mono text-[12px]">
              已选 {selected.length} / {TALENT_PICK_COUNT} 张
            </span>
          </Dim>
        </div>
      </Stage>
    );
  }

  return (
    <Stage wide>
      <Eyebrow>CARD SORT / SELF-REFLECTION</Eyebrow>
      <h1 className="mt-3 text-[28px] font-semibold">{TALENT.sortTitle}</h1>
      <p className="awk-dim mt-2 text-[15px] leading-relaxed">{TALENT.sortLead}</p>

      <div className="mt-7 grid gap-4 lg:grid-cols-3">
        {TALENT_LANES.map((lane) => (
          <div key={lane.key} className="awk-soft p-5">
            <div className="font-semibold">{lane.label}</div>
            <div className="awk-dim mt-1 text-[12px] leading-relaxed">{lane.body}</div>
            <div className="mt-4 grid gap-2">
              {(lanes[lane.key] ?? []).map((id) => {
                const c = TALENT_CARDS.find((x) => x.id === id)!;
                return (
                  <div key={id} className="awk-card" style={{ borderColor: c.accent, cursor: "default" }}>
                    <span className="font-semibold">
                      <span style={{ color: c.accent }}>{c.mark}</span> {c.title}
                    </span>
                  </div>
                );
              })}
              {(lanes[lane.key] ?? []).length === 0 ? (
                <div className="awk-dim py-3 text-center text-[12px]">待放入</div>
              ) : null}
            </div>
          </div>
        ))}
      </div>

      <div className="mt-7">
        <div className="awk-eyebrow">待归类</div>
        <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {selected
            .filter((id) => !placed.has(id))
            .map((id) => {
              const c = TALENT_CARDS.find((x) => x.id === id)!;
              return (
                <div key={id} className="awk-soft p-4">
                  <div className="font-semibold">
                    <span style={{ color: c.accent }}>{c.mark}</span> {c.title}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {TALENT_LANES.map((lane) => (
                      <Ghost key={lane.key} onClick={() => place(id, lane.key)}>
                        {lane.label}
                      </Ghost>
                    ))}
                  </div>
                </div>
              );
            })}
          {allPlaced ? <p className="awk-dim text-[13px]">5 张都已归类。</p> : null}
        </div>
      </div>

      <div className="mt-8">
        <Primary
          disabled={!allPlaced}
          onClick={() =>
            onDone({
              selected,
              lanes: { energy: lanes.energy, learned: lanes.learned, latent: lanes.latent },
            })
          }
        >
          {TALENT.finish}
        </Primary>
      </div>
      <p className="awk-dim mt-4 text-[12px] leading-relaxed">{TALENT.disclaimer}</p>
    </Stage>
  );
}
