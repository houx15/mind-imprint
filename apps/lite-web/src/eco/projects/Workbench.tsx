import { useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowUp, Check, Globe, Info } from "lucide-react";
import { useEco } from "../store";
import {
  asRows,
  asText,
  cardById,
  cardProgress,
  cardsForTrack,
  filledRows,
  motiveOf,
} from "../data/cards";
import { METHODS, PBL_COVENANT } from "../data/method";
import { trackById } from "../data/projects";
import { go } from "../route";
import type { CardEntry, Project, ThreadItem } from "../data/types";
import { Bold, Btn, Empty, Field, Panel, Sys, cx } from "../ui";
import { CardSurface } from "./CardSurface";

/**
 * 项目工作台 — the agentic room.
 *
 * ## What this replaced, and why
 * v1 was a plan with checkboxes. The AI's whole contribution arrived in one
 * burst at the start; after that the student ticked boxes alone. Nothing was
 * agentic about it in any sense that matters.
 *
 * The workbench is a **running conversation that summons instruments**. Left:
 * the thread — 印记's turns, hers, and the 工具卡 it puts on the table, inline,
 * with the reason attached. Right: the rail — every card in the track, done
 * and undone, plus the method 印记 is working from and the covenant that says
 * who does what. The unit of progress is a card that came back, not a box
 * that got ticked.
 *
 * ## Three rules the layout enforces
 * 1. **A card never opens itself.** 印记 offers; she presses 打开. The
 *    invitation carries the reason (铁律②, and Intent Preview).
 * 2. **现在不做 is a real answer.** Declining puts the card in the rail and
 *    ends the subject. 印记 does not ask again.
 * 3. **The covenant is visible, not implied.** 「印记可以动手，但不能替你判断」
 *    sits in the rail for the whole project, because in a PROJECT (unlike in
 *    her writing) the AI genuinely may build things, and the line between
 *    building and deciding is the only thing holding.
 */
export function Workbench({ id, cardId }: { id: string; cardId?: string }) {
  const { state } = useEco();
  const project = state.projects.find((p) => p.id === id);

  if (!project) {
    return (
      <div className="mx-auto max-w-[720px] px-8 py-16">
        <Empty
          title="找不到这个项目"
          body="它可能是在另一个标签页里建的——这个原型的数据只活在当前标签页。"
          action={<Btn onClick={() => go({ name: "projects" })}>回项目页</Btn>}
        />
      </div>
    );
  }

  if (cardId) return <CardSurface project={project} cardId={cardId} />;
  return <Room project={project} />;
}

function Room({ project }: { project: Project }) {
  const { sayInProject, publishProject } = useEco();
  const [draft, setDraft] = useState("");
  const [summary, setSummary] = useState("");
  const endRef = useRef<HTMLDivElement>(null);
  const t = trackById(project.track);
  const { done, total } = cardProgress(project.cards, project.track);
  const motive = motiveOf(project);
  const shipped = project.cards.find((c) => c.cardId === "ship" && c.status === "done");

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [project.thread.length]);

  return (
    <div className="flex min-h-full">
      {/* ── the conversation ────────────────────────────────────────── */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="border-b border-mk-border px-8 pb-4 pt-6">
          <Btn
            variant="quiet"
            size="sm"
            iconStart={<ArrowLeft size={15} />}
            onClick={() => go({ name: "projects" })}
          >
            项目
          </Btn>
          <div className="mt-2.5 flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="h-2 w-2 rounded-mk-full" style={{ background: t.hue }} />
                <Sys>
                  {t.label} · 开始于 {project.startedAt}
                </Sys>
              </div>
              <h1 className="mt-1 text-mk-display text-mk-ink">{project.title}</h1>
              {project.intent ? (
                <p className="mt-1.5 max-w-[64ch] text-mk-body leading-[1.8] text-mk-secondary">
                  <span className="text-mk-muted">你一开始说：</span>
                  {project.intent}
                </p>
              ) : null}
            </div>
            <div className="text-right">
              <Sys>工具卡</Sys>
              <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
                {done}/{total}
              </span>
            </div>
          </div>

          {motive ? (
            <div
              className="mt-4 rounded-mk-lg border p-4"
              style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
            >
              <Sys className="!text-[#8A6320]">你当初说 · 动机</Sys>
              <div className="mt-2 grid gap-3 sm:grid-cols-3">
                {[
                  ["为谁做", motive.who],
                  ["不做的代价", motive.cost],
                  ["为什么是我", motive.mine],
                ].map(([k, v]) => (
                  <div key={k}>
                    <p className="text-mk-small font-semibold text-[#8A6320]">{k}</p>
                    <p className="mt-0.5 text-mk-small leading-[1.8] text-[#6B4D14]">{v || "—"}</p>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
        </header>

        <div className="min-h-0 flex-1 px-8 py-6">
          <ol className="mx-auto max-w-[720px] space-y-5">
            {project.thread.map((item) => (
              <li key={item.id}>
                <Turn item={item} project={project} />
              </li>
            ))}
          </ol>

          {/* publish, once she has filed 发布前检查 */}
          {shipped && project.status !== "published" ? (
            <Panel className="mx-auto mt-8 max-w-[720px] p-6">
              <Sys>最后一步</Sys>
              <h3 className="mt-1 text-mk-h1 text-mk-ink">把它放到我的主页上</h3>
              <p className="mt-2 max-w-[58ch] text-mk-body leading-[1.9] text-mk-secondary">
                你在发布前检查里写的那段可以直接用，也可以在这儿改。
              </p>
              <div className="mt-4">
                <Field
                  label="做出了什么"
                  hint="两三句。写你真的做出了什么、遇到了什么、改了什么。"
                  value={summary || asText(shipped.values.made)}
                  onChange={setSummary}
                  rows={4}
                />
              </div>
              <Btn
                className="mt-4"
                iconStart={<Globe size={16} strokeWidth={1.9} />}
                disabled={(summary || asText(shipped.values.made)).trim().length < 4}
                onClick={() =>
                  publishProject(project.id, (summary || asText(shipped.values.made)).trim())
                }
              >
                发布这个项目
              </Btn>
            </Panel>
          ) : null}

          {project.status === "published" ? (
            <Panel className="mx-auto mt-8 max-w-[720px] p-6">
              <div className="flex items-center gap-2">
                <Globe size={16} strokeWidth={1.9} color="var(--mk-success)" />
                <Sys className="!text-mk-success">已发布</Sys>
              </div>
              <p className="mt-2 text-mk-body-lg leading-[1.9] text-mk-ink">{project.summary}</p>
              <Btn
                className="mt-4"
                variant="outline"
                onClick={() => go({ name: "page", handle: "zhiyao" })}
              >
                去我的主页看看
              </Btn>
            </Panel>
          ) : null}

          <div ref={endRef} />
        </div>

        {/* composer */}
        <div className="sticky bottom-0 border-t border-mk-border bg-mk-surface px-8 py-4">
          <form
            className="mx-auto flex max-w-[720px] items-end gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const text = draft.trim();
              if (!text) return;
              sayInProject(project.id, text);
              setDraft("");
            }}
          >
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  const text = draft.trim();
                  if (!text) return;
                  sayInProject(project.id, text);
                  setDraft("");
                }
              }}
              rows={1}
              placeholder="卡住了、想到别的、或者想反驳印记——都可以直接说"
              className="max-h-[140px] min-h-[44px] flex-1 resize-none rounded-mk-lg border border-mk-input-border
                         bg-mk-paper px-3.5 py-3 text-mk-body text-mk-ink placeholder:text-mk-faint
                         focus:border-mk-accent focus:outline-none focus:ring-2 focus:ring-mk-accent-200"
            />
            <Btn type="submit" disabled={!draft.trim()} iconStart={<ArrowUp size={16} />}>
              说
            </Btn>
          </form>
        </div>
      </div>

      {/* ── the rail ────────────────────────────────────────────────── */}
      <Rail project={project} />
    </div>
  );
}

/** One turn in the thread: a line of speech, or a card on the table. */
function Turn({ item, project }: { item: ThreadItem; project: Project }) {
  const { openCardEntry, skipCard } = useEco();

  if (item.kind === "say") {
    const coach = item.role === "coach";
    return (
      <div className={cx("flex gap-3", coach ? "" : "flex-row-reverse")}>
        <span
          className={cx(
            "mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold",
            coach ? "text-white" : "text-mk-secondary",
          )}
          style={{
            background: coach
              ? "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))"
              : "var(--mk-border)",
          }}
        >
          {coach ? "印" : "我"}
        </span>
        <div
          className={cx(
            "max-w-[86%] whitespace-pre-wrap rounded-mk-lg px-4 py-3 text-mk-body-lg leading-[1.9]",
            coach ? "text-mk-ink" : "text-mk-ink",
          )}
          style={{
            background: coach ? "var(--mk-surface)" : "var(--mk-accent-50)",
            border: `1px solid ${coach ? "var(--mk-border)" : "var(--mk-accent-200)"}`,
          }}
        >
          {/* 印记's scripted copy carries `**bold**` to name the method in a
              sentence. Printing the asterisks is the failure mode this shared
              component exists to prevent. */}
          <Bold text={item.text} />
        </div>
      </div>
    );
  }

  const spec = cardById(item.cardId);
  const entry = project.cards.find((c) => c.cardId === item.cardId);
  if (!spec) return null;
  const done = entry?.status === "done";

  return (
    <div className="flex gap-3">
      <span
        className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold text-white"
        style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
      >
        印
      </span>
      <div
        className="eco-in min-w-0 flex-1 rounded-mk-lg border p-5"
        style={{
          borderColor: `color-mix(in srgb, ${spec.hue} 48%, transparent)`,
          background: `color-mix(in srgb, ${spec.hue} 9%, var(--mk-surface))`,
        }}
      >
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <span className="text-[15px]">{spec.glyph}</span>
          <Sys>工具卡</Sys>
          <span className="text-mk-h3 text-mk-ink">{spec.title}</span>
          <span className="font-mono text-mk-small text-mk-muted">约 {spec.minutes} 分钟</span>
          {done ? (
            <span className="inline-flex items-center gap-1 rounded-mk-full bg-mk-success px-2 py-0.5 text-[11px] font-semibold text-white">
              <Check size={10} strokeWidth={3} /> 已提交
            </span>
          ) : null}
        </div>
        <p className="mt-2.5 text-mk-body-lg leading-[1.9] text-mk-ink">{spec.reason}</p>
        <div className="mt-3.5 flex flex-wrap gap-2">
          <Btn
            size="sm"
            onClick={() => {
              openCardEntry(project.id, item.cardId);
              go({ name: "project", id: project.id, cardId: item.cardId });
            }}
          >
            {done ? "回去看看我写的" : "打开这张卡"}
          </Btn>
          {!done ? (
            <Btn size="sm" variant="quiet" onClick={() => skipCard(project.id, item.cardId)}>
              现在不做
            </Btn>
          ) : null}
        </div>
      </div>
    </div>
  );
}

/**
 * The rail — 材料 · 方法 · 分工.
 *
 * 材料 lists every card in the track, so she can always see what is coming and
 * open anything out of order. A done card shows a one-line trace of what she
 * actually put in it, because a rail of grey titles is a table of contents and
 * a rail of her own sentences is a body of work.
 */
function Rail({ project }: { project: Project }) {
  const { openCardEntry } = useEco();
  const [tab, setTab] = useState<"work" | "method">("work");
  const seq = cardsForTrack(project.track);

  return (
    <aside
      className="hidden w-[320px] shrink-0 border-l border-mk-border bg-mk-surface lg:block"
    >
      <div className="sticky top-0 max-h-screen overflow-y-auto px-5 py-6">
        <div className="mb-4 flex gap-1 rounded-mk-full p-1" style={{ background: "var(--mk-paper)" }}>
          {(
            [
              ["work", "材料"],
              ["method", "方法"],
            ] as const
          ).map(([k, label]) => (
            <button
              key={k}
              type="button"
              onClick={() => setTab(k)}
              className={cx(
                "flex-1 rounded-mk-full px-3 py-1.5 text-mk-small transition-colors duration-[120ms]",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                tab === k ? "bg-mk-surface font-semibold text-mk-ink shadow-mk-xs" : "text-mk-muted",
              )}
            >
              {label}
            </button>
          ))}
        </div>

        {tab === "work" ? (
          <ol className="space-y-1.5">
            {seq.map((cid, i) => {
              const spec = cardById(cid);
              const entry = project.cards.find((c) => c.cardId === cid);
              if (!spec) return null;
              const done = entry?.status === "done";
              const started = entry?.status === "open";
              const offered = Boolean(entry);
              return (
                <li key={cid}>
                  <button
                    type="button"
                    disabled={!offered}
                    onClick={() => {
                      openCardEntry(project.id, cid);
                      go({ name: "project", id: project.id, cardId: cid });
                    }}
                    className={cx(
                      "w-full rounded-mk-md border p-3 text-left transition-colors duration-[120ms]",
                      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                      offered
                        ? "border-mk-border bg-mk-surface hover:border-mk-accent-200 hover:bg-mk-accent-50"
                        : "cursor-default border-dashed border-mk-border bg-transparent opacity-55",
                    )}
                  >
                    <span className="flex items-center gap-2">
                      <span className="eco-mono w-5 shrink-0 text-mk-faint">
                        {String(i + 1).padStart(2, "0")}
                      </span>
                      <span
                        className={cx(
                          "min-w-0 flex-1 truncate text-mk-body",
                          done ? "font-semibold text-mk-ink" : "text-mk-secondary",
                        )}
                      >
                        {spec.title}
                      </span>
                      {done ? (
                        <Check size={13} strokeWidth={2.6} color="var(--mk-success)" />
                      ) : started ? (
                        <span className="eco-mono text-mk-accent-700" style={{ letterSpacing: 0 }}>
                          写着
                        </span>
                      ) : offered ? (
                        <span className="eco-mono text-mk-faint" style={{ letterSpacing: 0 }}>
                          待开
                        </span>
                      ) : null}
                    </span>
                    {done && entry ? (
                      <span className="mt-1.5 block pl-7 text-mk-small leading-[1.7] text-mk-muted">
                        {trace(entry)}
                      </span>
                    ) : null}
                  </button>
                </li>
              );
            })}
          </ol>
        ) : (
          <div className="space-y-4">
            <div
              className="rounded-mk-md border p-4"
              style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }}
            >
              <div className="flex items-center gap-1.5">
                <Info size={13} strokeWidth={2} className="text-mk-accent-700" />
                <Sys className="!text-mk-accent-700">{PBL_COVENANT.title}</Sys>
              </div>
              <p className="mt-2 text-mk-small font-semibold text-mk-ink">印记可以</p>
              <ul className="mt-1 space-y-1">
                {PBL_COVENANT.can.map((c) => (
                  <li key={c} className="text-mk-small leading-[1.75] text-mk-secondary">
                    · {c}
                  </li>
                ))}
              </ul>
              <p className="mt-2.5 text-mk-small font-semibold text-mk-ink">这些必须是你的</p>
              <ul className="mt-1 space-y-1">
                {PBL_COVENANT.must.map((c) => (
                  <li key={c} className="text-mk-small leading-[1.75] text-mk-secondary">
                    · {c}
                  </li>
                ))}
              </ul>
              <p className="mt-2.5 border-t border-mk-accent-200 pt-2.5 text-mk-small leading-[1.8] text-mk-ink">
                {PBL_COVENANT.line}
              </p>
            </div>

            {METHODS.map((m) => (
              <details key={m.id} className="rounded-mk-md border border-mk-border p-3.5">
                <summary className="cursor-pointer text-mk-body font-semibold text-mk-ink">
                  {m.label}
                </summary>
                <p className="mt-1 text-mk-small text-mk-muted">{m.from}</p>
                <ul className="mt-2 space-y-1">
                  {m.points.map((pt) => (
                    <li key={pt} className="text-mk-small leading-[1.75] text-mk-secondary">
                      · {pt}
                    </li>
                  ))}
                </ul>
                <p className="mt-2 text-mk-small leading-[1.75] text-mk-faint">{m.why}</p>
              </details>
            ))}
          </div>
        )}
      </div>
    </aside>
  );
}

/**
 * One line of what she actually put in a finished card.
 *
 * Generic on purpose — it reads the entry, not the card id, so a new card
 * needs no change here. Rows win over prose because a row count is the most
 * honest single number a card can report.
 */
function trace(entry: CardEntry): string {
  for (const v of Object.values(entry.values)) {
    const rows = filledRows(asRows(v));
    if (rows.length) return `${rows.length} 条`;
  }
  for (const v of Object.values(entry.values)) {
    const t = asText(v).trim();
    if (t.length > 4) return t.length > 30 ? `${t.slice(0, 30)}…` : t;
  }
  return "已提交";
}
