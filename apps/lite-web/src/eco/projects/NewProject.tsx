import { useState } from "react";
import { ArrowLeft, ArrowRight, MessageCircle, Sparkles } from "lucide-react";
import { useEco } from "../store";
import { TRACKS, TRACK_PROPOSALS, trackById } from "../data/projects";
import { cardById, cardsForTrack } from "../data/cards";
import { KEYWORDS } from "../data/tree";
import { go } from "../route";
import type { TrackId } from "../data/types";
import { Btn, Field, Panel, SectionHead, Sys, cx } from "../ui";

/**
 * 开一个新项目 — the door, and only the door.
 *
 * ## What this screen used to do, and why it stopped
 * v1 made her answer the whole why-ladder and then reorder a generated
 * six-step plan BEFORE the project existed. Two problems: the hardest
 * questions in the whole product were asked by a form she had no relationship
 * with yet, and if she bailed halfway it all evaporated.
 *
 * Now the door asks for exactly two things — which kind of thing, and one
 * sentence about what she wants — and then opens the workbench, where 印记
 * greets her and puts 动机三问 on the table as a real card. The why still gets
 * the most attention in the product; it just happens INSIDE the project, where
 * the answers are kept and pinned.
 *
 * ## Two doors, always
 * Pick a track, or let 印记 propose from her tree. `TRACK_PROPOSALS` reads
 * real keywords — that is the payoff for having a keyword model at all: the
 * proposals come with evidence attached, not as generic prompts.
 */
export function NewProject() {
  const { state, draftTrack, draftTitle, draftIntent, createProject, setNewProjectMode } = useEco();
  const d = state.draft;
  const [mode, setMode] = useState<"pick" | "talk">(state.newProjectMode);

  function start() {
    const id = createProject();
    go({ name: "project", id });
  }

  return (
    <div className="mx-auto max-w-[980px] px-8 py-8">
      <Btn
        variant="quiet"
        size="sm"
        iconStart={<ArrowLeft size={15} />}
        onClick={() => go({ name: "projects" })}
      >
        项目
      </Btn>

      <SectionHead
        index="新项目 · NEW"
        title="你想做点什么？"
        sub="先只回答两件事：做哪一类，和你想干嘛。剩下的进去以后和印记一起弄。"
      />

      {/* two doors */}
      <div className="mb-7 flex gap-1 rounded-mk-full p-1" style={{ background: "var(--mk-paper)" }}>
        {(
          [
            ["pick", "我知道要做什么"],
            ["talk", "让印记从我的树上提三个"],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            onClick={() => {
              setMode(k);
              setNewProjectMode(k);
            }}
            className={cx(
              "flex-1 rounded-mk-full px-4 py-2 text-mk-body transition-colors duration-[140ms]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
              mode === k ? "bg-mk-surface font-semibold text-mk-ink shadow-mk-xs" : "text-mk-muted",
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {mode === "talk" ? (
        <div className="mb-8">
          <div className="flex gap-3">
            <span
              className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-bold text-white"
              style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
            >
              印
            </span>
            <div className="rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3">
              <p className="text-mk-body-lg leading-[1.9] text-mk-ink">
                我看了你的树。有三个词已经有足够的东西撑起一个项目了——不是我随便挑的，每一个下面都写了理由。
              </p>
            </div>
          </div>

          <ul className="mt-4 grid gap-3 md:grid-cols-2">
            {TRACK_PROPOSALS.map((p, i) => {
              const t = trackById(p.track);
              const kw = KEYWORDS.find((k) => p.from.includes(k.text));
              return (
                <li key={p.title} className="eco-in" style={{ ["--i" as string]: i }}>
                  <button
                    type="button"
                    onClick={() => {
                      draftTrack(p.track, p.title);
                      draftIntent(p.why);
                      setMode("pick");
                    }}
                    className="h-full w-full rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left
                               transition-colors duration-[140ms] ease-mk hover:border-mk-accent-200
                               hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2
                               focus-visible:ring-mk-accent-200"
                  >
                    <span className="flex items-center gap-2">
                      <span className="h-2 w-2 rounded-mk-full" style={{ background: t.hue }} />
                      <Sys>{t.label}</Sys>
                    </span>
                    <span className="mt-1.5 block text-mk-h2 text-mk-ink">{p.title}</span>
                    <span className="mt-2 flex items-center gap-1.5">
                      <Sparkles size={13} strokeWidth={2} className="text-mk-accent-700" />
                      <span className="text-mk-small text-mk-secondary">
                        来自你的关键词「{kw?.text ?? p.from}」
                      </span>
                    </span>
                    <span className="mt-1.5 block text-mk-body leading-[1.8] text-mk-secondary">
                      {p.why}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>

          <p className="mt-4 text-mk-small text-mk-muted">
            都不想做也没关系——
            <button
              type="button"
              onClick={() => {
                setMode("pick");
                setNewProjectMode("pick");
              }}
              className="font-semibold text-mk-accent-700 underline underline-offset-2
                         focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              自己挑一类
            </button>
            。
          </p>
        </div>
      ) : null}

      {/* track grid */}
      <Sys className="mb-2.5 block">① 做哪一类</Sys>
      <ul className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
        {TRACKS.map((t, i) => {
          const on = d.track === t.id;
          return (
            <li key={t.id} className="eco-in" style={{ ["--i" as string]: i }}>
              <button
                type="button"
                onClick={() => draftTrack(t.id, d.title)}
                className={cx(
                  "h-full w-full rounded-mk-lg border p-5 text-left transition-all duration-[140ms] ease-mk",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                  on
                    ? "border-mk-accent bg-mk-accent-50"
                    : "border-mk-border bg-mk-surface hover:border-mk-accent-200",
                )}
              >
                <span className="flex items-center gap-2">
                  <span className="text-[17px]" style={{ color: t.hue }}>
                    {t.glyph}
                  </span>
                  <span className="text-mk-h3 text-mk-ink">{t.label}</span>
                </span>
                <span className="mt-1.5 block text-mk-body leading-[1.8] text-mk-secondary">
                  {t.blurb}
                </span>
                <span className="mt-2.5 block text-mk-small leading-[1.75] text-mk-muted">
                  <span className="text-mk-faint">做完你会有：</span>
                  {t.ends}
                </span>
                {on ? (
                  <span className="mt-3 block border-t border-mk-accent-200 pt-2.5">
                    <Sys className="!text-mk-accent-700">印记会陪你走这几张卡</Sys>
                    <span className="mt-1 block text-mk-small leading-[1.8] text-mk-secondary">
                      {cardsForTrack(t.id)
                        .map((c) => cardById(c)?.title ?? c)
                        .join(" · ")}
                    </span>
                  </span>
                ) : null}
              </button>
            </li>
          );
        })}
      </ul>

      {/* name + intent */}
      {d.track ? (
        <Panel className="eco-in mt-7 p-6">
          <Sys>② 你想干嘛</Sys>
          <p className="mt-1 max-w-[60ch] text-mk-body leading-[1.85] text-mk-secondary">
            现在写得糙没关系，进去第一张卡就是把它问清楚。
          </p>
          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <Field
              label="给它起个名字"
              hint="一句话，说清做什么。之后能改。"
              value={d.title}
              onChange={draftTitle}
              rows={2}
              placeholder={trackById(d.track).examples[0]}
            />
            <Field
              label="一句话说说你想干嘛"
              hint="这句会被原样留在项目里，不会被改写。"
              value={d.intent}
              onChange={draftIntent}
              rows={2}
              placeholder="例如：我想知道我们班的人到底给别人留多少时间。"
            />
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <Btn
              iconStart={<ArrowRight size={16} />}
              disabled={d.title.trim().length < 2}
              onClick={start}
            >
              开工
            </Btn>
            <span className="flex items-center gap-1.5 text-mk-small text-mk-muted">
              <MessageCircle size={13} strokeWidth={1.9} />
              进去以后印记会先问你三个问题：为谁做、不做会怎样、为什么是你。
            </span>
          </div>
        </Panel>
      ) : null}
    </div>
  );
}
