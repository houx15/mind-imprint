import { useEffect, useMemo, useState } from "react";
import { Check, Copy, Plus, Trash2 } from "lucide-react";
import { useEco } from "../store";
import {
  asList,
  asRows,
  asText,
  cardAnswered,
  cardById,
  filledRows,
} from "../data/cards";
import { methodById } from "../data/method";
import { variantsFor } from "../data/projects";
import { PAGE_EXAMPLES } from "../data/examples";
import type { CardRow, CardValue, Project } from "../data/types";
import { Btn, Empty, Panel, Sys, cx } from "../ui";
import { StepBanner, stepIdFor } from "./panels/StepBanner";

/**
 * 工具卡工作面 — the schema-driven card renderer.
 *
 * ## The contract this file exists to keep
 * **A new card is a new spec in `data/cards.ts`. It is never new code here.**
 * Everything below renders from `CardSpec.fields` using five primitives —
 * `text` / `textarea` / `choice` / `multi` / `rows`. If you find yourself
 * about to write `if (spec.id === "...")` in this file, stop: either the card
 * fits the primitives, or the primitives need a sixth member. Special-casing
 * one card is how schema-driven rendering quietly stops being true.
 *
 * The two `EXTRAS` below are the允许的例外 and they are deliberately not
 * fields: they are *reference material 印记 puts on the table beside the card*
 * (the three variants to choose from, the example pages to browse). They
 * render above the fields and write nothing. A card is still just its spec.
 *
 * ## Autosave, and why 提交 still exists
 * Every keystroke saves. 提交 is not "save" — it is **回灌**: the moment the
 * card goes back into the conversation and 印记 responds to what she actually
 * wrote. That is the loop, so it needs a deliberate act.
 */
export function CardSurface({
  project,
  cardId,
  onDone,
}: {
  project: Project;
  cardId: string;
  /** Close the stage. The card no longer owns a route of its own — it is one
   *  of four things the workbench panel can hold, so leaving it is the
   *  panel's business, not a navigation. */
  onDone: () => void;
}) {
  const { openCardEntry, saveCard, submitCard } = useEco();
  const spec = cardById(cardId);
  const entry = project.cards.find((c) => c.cardId === cardId);
  const [values, setValues] = useState<Record<string, CardValue>>(() => entry?.values ?? {});

  // Opening the surface IS accepting the invitation.
  useEffect(() => {
    openCardEntry(project.id, cardId);
    // Only when the identity of the card changes; `openCardEntry` is stable
    // per render of the provider and would otherwise re-fire every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project.id, cardId]);

  const done = entry?.status === "done";
  const answered = useMemo(() => (spec ? cardAnswered(spec, values) : false), [spec, values]);

  if (!spec) {
    return (
      <div className="px-6 py-10">
        <Empty title="找不到这张卡" body="它可能已经被换掉了。" action={<Btn onClick={onDone}>收起</Btn>} />
      </div>
    );
  }

  function set(fieldId: string, v: CardValue) {
    const next = { ...values, [fieldId]: v };
    setValues(next);
    saveCard(project.id, cardId, next);
  }

  const method = spec.method ? methodById(spec.method) : undefined;

  return (
    <div className="h-full overflow-y-auto px-6 py-5">
      {/* 你在哪一步 · 这一步你判断 · 填完会发生什么. Above the card head,
          because 「我在哪儿、为什么」 is the question she asks first. */}
      <StepBanner
        project={project}
        payoff={spec.payoff}
        stepId={stepIdFor(project, { kind: "card", id: cardId })}
      />

      {/* ── card head ─────────────────────────────────────────────────── */}
      <div
        className="rounded-mk-lg border p-5"
        style={{
          borderColor: `color-mix(in srgb, ${spec.hue} 46%, transparent)`,
          background: `color-mix(in srgb, ${spec.hue} 9%, var(--mk-surface))`,
        }}
      >
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span
            className="flex h-8 w-8 items-center justify-center rounded-mk-md text-[16px]"
            style={{ background: `color-mix(in srgb, ${spec.hue} 30%, transparent)` }}
          >
            {spec.glyph}
          </span>
          <h1 className="text-mk-h1 text-mk-ink">{spec.title}</h1>
          <span className="font-mono text-mk-small text-mk-muted">约 {spec.minutes} 分钟</span>
          {done ? (
            <span className="inline-flex items-center gap-1 rounded-mk-full bg-mk-success px-2.5 py-0.5 text-[11px] font-semibold text-white">
              <Check size={11} strokeWidth={3} /> 已提交
            </span>
          ) : null}
        </div>

        {/* Intent preview — 印记's stated reason for putting this card here.
            A card that appears without a reason is an ambush. */}
        <p className="mt-3.5 border-l-2 pl-3.5 text-mk-body-lg leading-[1.85] text-mk-ink"
           style={{ borderColor: spec.hue }}>
          {spec.reason}
        </p>

        <details className="mt-4">
          <summary className="cursor-pointer text-mk-small text-mk-secondary">
            为什么是这张卡
          </summary>
          <p className="mt-2 max-w-[64ch] text-mk-body leading-[1.9] text-mk-secondary">
            {spec.teaches}
          </p>
          {method ? (
            <div
              className="mt-3 rounded-mk-md p-3.5"
              style={{ background: "var(--mk-paper)", border: "1px solid var(--mk-border)" }}
            >
              <Sys>方法出处</Sys>
              <p className="mt-1 text-mk-body font-semibold text-mk-ink">{method.label}</p>
              <p className="text-mk-small text-mk-muted">{method.from}</p>
              <ul className="mt-2 space-y-1">
                {method.points.map((pt) => (
                  <li key={pt} className="flex gap-2 text-mk-small leading-[1.75] text-mk-secondary">
                    <span className="text-mk-faint">·</span>
                    {pt}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </details>
      </div>

      {/* ── reference material 印记 puts on the table ─────────────────── */}
      <Extras cardId={cardId} project={project} />

      {/* ── the fields ────────────────────────────────────────────────── */}
      <div className="mt-6 space-y-6">
        {spec.fields.map((f) => (
          <div key={f.id}>
            <label className="block text-mk-h3 text-mk-ink" htmlFor={`f-${f.id}`}>
              {f.label}
            </label>
            {f.hint ? (
              <p className="mt-1 max-w-[64ch] text-mk-small leading-[1.75] text-mk-muted">{f.hint}</p>
            ) : null}

            <div className="mt-2.5">
              {f.kind === "text" ? (
                <input
                  id={`f-${f.id}`}
                  value={asText(values[f.id])}
                  placeholder={f.placeholder}
                  onChange={(e) => set(f.id, e.target.value)}
                  className={INPUT}
                />
              ) : null}

              {f.kind === "textarea" ? (
                <textarea
                  id={`f-${f.id}`}
                  value={asText(values[f.id])}
                  placeholder={f.placeholder}
                  rows={4}
                  onChange={(e) => set(f.id, e.target.value)}
                  className={cx(INPUT, "resize-y leading-[1.85]")}
                />
              ) : null}

              {f.kind === "choice" ? (
                <div className="grid gap-2 sm:grid-cols-3">
                  {(f.options ?? []).map((o) => {
                    const on = asText(values[f.id]) === o.id;
                    return (
                      <button
                        key={o.id}
                        type="button"
                        onClick={() => set(f.id, o.id)}
                        className={cx(
                          "rounded-mk-md border p-3.5 text-left transition-colors duration-[120ms] ease-mk",
                          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                          on
                            ? "border-mk-accent bg-mk-accent-50"
                            : "border-mk-border bg-mk-surface hover:border-mk-accent-200",
                        )}
                      >
                        <span className="block text-mk-body font-semibold text-mk-ink">{o.label}</span>
                        {o.blurb ? (
                          <span className="mt-1 block text-mk-small leading-[1.7] text-mk-muted">
                            {o.blurb}
                          </span>
                        ) : null}
                      </button>
                    );
                  })}
                </div>
              ) : null}

              {f.kind === "multi" ? (
                <ul className="space-y-1.5">
                  {(f.options ?? []).map((o) => {
                    const list = asList(values[f.id]);
                    const on = list.includes(o.id);
                    return (
                      <li key={o.id}>
                        <button
                          type="button"
                          onClick={() =>
                            set(f.id, on ? list.filter((x) => x !== o.id) : [...list, o.id])
                          }
                          className="flex w-full items-start gap-2.5 rounded-mk-md border border-mk-border
                                     bg-mk-surface p-3 text-left transition-colors duration-[120ms]
                                     hover:border-mk-accent-200 focus-visible:outline-none
                                     focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                        >
                          <span
                            className={cx(
                              "mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-[3px] border",
                              on ? "border-mk-success bg-mk-success" : "border-mk-input-border",
                            )}
                          >
                            {on ? <Check size={11} strokeWidth={3} color="#fff" /> : null}
                          </span>
                          <span className="text-mk-body leading-[1.7] text-mk-ink">{o.label}</span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              ) : null}

              {f.kind === "rows" ? (
                <Rows
                  columns={f.columns ?? []}
                  rowsLabel={f.rowsLabel ?? "一条"}
                  min={f.min ?? 1}
                  rows={asRows(values[f.id])}
                  onChange={(rows) => set(f.id, rows)}
                />
              ) : null}
            </div>
          </div>
        ))}
      </div>

      {/* The 明确指令 card compiles into something she can take away. This is
          reading her own answers back, not a sixth primitive. */}
      {cardId === "command" ? <CompiledInstruction values={values} /> : null}

      {/* ── submit ────────────────────────────────────────────────────── */}
      <div className="mt-8 flex flex-wrap items-center gap-3">
        <Btn
          disabled={!answered}
          onClick={() => {
            submitCard(project.id, cardId, values);
            onDone();
          }}
        >
          {done ? "重新提交" : "提交给印记"}
        </Btn>
        <Btn variant="quiet" onClick={onDone}>
          先存着，收起
        </Btn>
        {!answered ? (
          <span className="text-mk-small text-mk-muted">
            还有必填的地方没写完。写了的部分已经自动存好了。
          </span>
        ) : null}
      </div>
    </div>
  );
}

const INPUT =
  "w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-3.5 py-2.5 text-mk-body " +
  "text-mk-ink placeholder:text-mk-faint focus:border-mk-accent focus:outline-none " +
  "focus:ring-2 focus:ring-mk-accent-200";

/** The repeatable group. One primitive, used by seven different cards. */
function Rows({
  columns,
  rowsLabel,
  min,
  rows,
  onChange,
}: {
  columns: { id: string; label: string; placeholder?: string; wide?: boolean }[];
  rowsLabel: string;
  min: number;
  rows: CardRow[];
  onChange: (rows: CardRow[]) => void;
}) {
  // Always show at least `min` slots: an empty list with an 「添加」 button
  // reads as optional, and these are the cards' actual content.
  const shown = rows.length >= min ? rows : [...rows, ...Array(min - rows.length).fill({})];
  const filled = filledRows(rows).length;

  function edit(i: number, colId: string, v: string) {
    const next = shown.map((r, j) => (j === i ? { ...r, [colId]: v } : { ...r }));
    onChange(next);
  }

  return (
    <div>
      <ul className="space-y-2.5">
        {shown.map((row, i) => (
          <li
            key={i}
            className="rounded-mk-md border border-mk-border bg-mk-surface p-3.5"
          >
            <div className="mb-2 flex items-center justify-between">
              <Sys>
                {rowsLabel} {String(i + 1).padStart(2, "0")}
              </Sys>
              {shown.length > min ? (
                <button
                  type="button"
                  onClick={() => onChange(shown.filter((_, j) => j !== i))}
                  className="rounded-mk-full p-1 text-mk-faint transition-colors hover:bg-mk-accent-50
                             hover:text-mk-secondary focus-visible:outline-none focus-visible:ring-2
                             focus-visible:ring-mk-accent-200"
                  aria-label="删掉这一条"
                >
                  <Trash2 size={13} strokeWidth={1.9} />
                </button>
              ) : null}
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              {columns.map((c) => (
                <div key={c.id} className={c.wide ? "sm:col-span-2" : undefined}>
                  <span className="mb-1 block text-mk-small text-mk-muted">{c.label}</span>
                  {c.wide ? (
                    <textarea
                      rows={2}
                      value={row[c.id] ?? ""}
                      placeholder={c.placeholder}
                      onChange={(e) => edit(i, c.id, e.target.value)}
                      className={cx(INPUT, "resize-y leading-[1.8]")}
                    />
                  ) : (
                    <input
                      value={row[c.id] ?? ""}
                      placeholder={c.placeholder}
                      onChange={(e) => edit(i, c.id, e.target.value)}
                      className={INPUT}
                    />
                  )}
                </div>
              ))}
            </div>
          </li>
        ))}
      </ul>
      <div className="mt-2.5 flex items-center gap-3">
        <Btn
          size="sm"
          variant="outline"
          iconStart={<Plus size={14} />}
          onClick={() => onChange([...shown, {}])}
        >
          再加一条
        </Btn>
        <span className="font-mono text-mk-small text-mk-faint">
          {filled} / {min} 起
        </span>
      </div>
    </div>
  );
}

/**
 * Reference material beside a card.
 *
 * Two cards need 印记 to put something on the table before she can answer:
 * 三个版本 needs the three versions, 参考搜集 needs somewhere to start looking.
 * They render as read-only material above the fields and write nothing.
 */
function Extras({ cardId, project }: { cardId: string; project: Project }) {
  if (cardId === "variants") {
    const options = variantsFor(project.track);
    return (
      <div className="mt-6">
        <Sys>印记做的三版 · 都不完全对</Sys>
        <div className="mt-2.5 grid gap-3 md:grid-cols-3">
          {options.map((o) => (
            <Panel key={o.key} className="p-4">
              <div className="flex items-baseline gap-2">
                <span className="font-mono text-mk-h3 text-mk-accent-700">{o.key}</span>
                <span className="text-mk-body font-semibold text-mk-ink">{o.name}</span>
              </div>
              <p className="mt-0.5 font-mono text-[11px] uppercase tracking-wider text-mk-faint">
                {o.shape}
              </p>
              <ul className="mt-2.5 space-y-1.5">
                {o.body.map((b) => (
                  <li key={b} className="text-mk-small leading-[1.8] text-mk-secondary">
                    {b}
                  </li>
                ))}
              </ul>
            </Panel>
          ))}
        </div>
      </div>
    );
  }

  if (cardId === "sweep") {
    return (
      <div className="mt-6">
        <Sys>没头绪的话，这几个是真实存在的个人页</Sys>
        <p className="mt-1 max-w-[64ch] text-mk-small leading-[1.75] text-mk-muted">
          别照抄。看的时候只做一件事：找出你想拿走的那一处，然后用一句话把它写下来。
        </p>
        <div className="mt-2.5 grid gap-2.5 md:grid-cols-2">
          {PAGE_EXAMPLES.slice(0, 4).map((e) => (
            <div
              key={e.id}
              className="flex items-start gap-3 rounded-mk-md border border-mk-border bg-mk-surface p-3.5"
            >
              <span
                className="mt-1 h-10 w-10 shrink-0 rounded-mk-sm"
                style={{ background: `linear-gradient(140deg, ${e.swatch[0]}, ${e.swatch[1]})` }}
              />
              <div className="min-w-0">
                <p className="text-mk-body font-semibold text-mk-ink">{e.name}</p>
                <p className="text-mk-small text-mk-muted">{e.who}</p>
                <p className="mt-1.5 text-mk-small leading-[1.75] text-mk-secondary">{e.steal}</p>
                <p className="mt-1 break-all font-mono text-[11px] text-mk-faint">{e.url}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return null;
}

/**
 * The compiled instruction.
 *
 * This is the payoff of the 明确指令 card: her three answers assembled into a
 * prompt she can paste into ANY AI tool. It always ends with the same line —
 * appended here, not typed by her — because that line is the product's
 * position, not her preference.
 */
function CompiledInstruction({ values }: { values: Record<string, CardValue> }) {
  const [copied, setCopied] = useState(false);
  const like = asText(values.like).trim();
  const structure = asText(values.structure).trim();
  const mode = asText(values.mode);
  if (!like && !structure && !mode) return null;

  const modeLine =
    mode === "ask"
      ? "你一次问我一个问题，我回答，你只负责把我的话整理清楚。不要连着问两个。"
      : mode === "propose"
        ? "你给我两三个空的骨架让我挑，不要把内容填上——填空是我的事。"
        : mode === "tidy"
          ? "我先写，你只指出重复的地方、含糊的地方和缺例子的地方。不要给我可以直接抄的替代句。"
          : "（还没选配合方式）";

  const text = [
    "【我要它像谁】",
    like || "（还没写）",
    "",
    "【内容的结构】按这个顺序，不要多加也不要少：",
    structure || "（还没写）",
    "",
    "【我们怎么合作】",
    modeLine,
    "",
    "最后一条，最重要：正文是我写的。你可以问我、可以给我骨架、可以帮我挑出啰嗦的地方，但不要替我写出可以直接抄的句子。",
  ].join("\n");

  return (
    <div className="mt-7">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <Sys>你的指令 · 拿去用</Sys>
          <p className="mt-0.5 text-mk-small text-mk-muted">
            这段贴进任何一个 AI 工具都成立。你学的是下指令，不是学我们的按钮。
          </p>
        </div>
        <Btn
          size="sm"
          variant="outline"
          iconStart={<Copy size={14} />}
          onClick={() => {
            void navigator.clipboard?.writeText(text);
            setCopied(true);
            window.setTimeout(() => setCopied(false), 1800);
          }}
        >
          {copied ? "已复制" : "复制"}
        </Btn>
      </div>
      <pre
        className="mt-2.5 overflow-x-auto whitespace-pre-wrap rounded-mk-md p-4 text-mk-body leading-[1.9]"
        style={{
          background: "var(--mk-paper)",
          border: "1px solid var(--mk-border)",
          color: "var(--mk-ink)",
          fontFamily: "inherit",
        }}
      >
        {text}
      </pre>
    </div>
  );
}
