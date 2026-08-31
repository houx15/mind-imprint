import { useEffect, useState } from "react";
import { Check, Loader, Monitor, Play, Send, Smartphone } from "lucide-react";
import { useEco } from "../../store";
import { ARTIFACTS, artifactById } from "../../data/artifacts";
import { buildSite, siteTheme } from "../../data/site";
import { BuiltSite } from "../../site/BuiltSite";
import type { ArtifactOption, ArtifactSpec, Project } from "../../data/types";
import { Bold, Btn, Field, Sys, cx } from "../../ui";
import { StepBanner, stepIdFor } from "./StepBanner";
import { FormSurface } from "./FormSurface";

/**
 * 印记做的东西 — the stage where the AI hands work over.
 *
 * ## The shape of every one of these
 *   ① 印记 works, and the working is SHOWN (not a spinner — the actual lines
 *      of what it is doing, which is the only honest way to charge someone
 *      thirty seconds of waiting).
 *   ② It hands the thing over **with its guesses named**. `note` on every
 *      spec says what it assumed; a handover that hides its assumptions can
 *      only be accepted, never reviewed.
 *   ③ She judges. And 🚨 **nothing settles without a reason** — every 收下
 *      button on this screen is disabled until she has written why.
 *
 * That last rule is the entire point of the artifact layer. The moment a
 * 「就用这个」 works on its own, this becomes a machine that generates and a
 * student who approves, which is the thing the product exists not to be.
 *
 * ## Why the work is faked with a timer
 * This is a prototype with no backend (see `data/types.ts`). The lines are
 * real copy about real steps; the elapsed time is theatre. It is theatre worth
 * keeping: the plan claims 印记 开工 is the long step, and a build that
 * resolves instantly would quietly contradict the plan the student just
 * agreed to.
 */
/** Chinese, and descriptive. The English enum names leaked out of the data
 *  model onto the screen, where they said nothing to a student. */
const KIND_TAG: Record<string, string> = {
  options: "方案对比",
  draft: "内容初稿",
  build: "页面构建",
  form: "问卷调研",
};

export function Make({ project, artifactId }: { project: Project; artifactId: string }) {
  const { runArtifact, artifactReady } = useEco();
  const spec = artifactById(artifactId);
  const st = project.artifacts[artifactId];
  const [line, setLine] = useState(0);

  const working = st?.status === "working";

  useEffect(() => {
    if (!working || !spec) return;
    setLine(0);
    let i = 0;
    const t = window.setInterval(() => {
      i += 1;
      if (i >= spec.steps.length) {
        window.clearInterval(t);
        artifactReady(project.id, artifactId);
        return;
      }
      setLine(i);
    }, 900);
    return () => window.clearInterval(t);
    // Restarting on every store change would reset the ticker forever; this
    // deliberately keys only on "did it start working".
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [working, artifactId, project.id]);

  if (!spec) {
    return <p className="p-6 text-mk-body text-mk-muted">找不到这一步要做的东西。</p>;
  }

  return (
    <div className="h-full overflow-y-auto px-6 py-5">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <Sys>印记的产出 · 需要你确认</Sys>
        <span className="eco-mono text-mk-faint">{KIND_TAG[spec.kind]}</span>
      </div>
      <h2 className="mb-4 mt-1 text-mk-h1 text-mk-ink">{spec.title}</h2>

      {/* 你在哪一步 · 这一步你判断 · 填完会发生什么 — above the work, always. */}
      <StepBanner
        project={project}
        payoff={spec.payoff}
        stepId={stepIdFor(project, { kind: "make", id: artifactId })}
      />

      {!st || st.status === "idle" ? (
        <Idle spec={spec} onRun={() => runArtifact(project.id, artifactId)} />
      ) : working ? (
        <Working spec={spec} line={line} />
      ) : spec.kind === "options" ? (
        <Options project={project} spec={spec} />
      ) : spec.kind === "draft" ? (
        <Draft project={project} spec={spec} />
      ) : spec.kind === "form" ? (
        <FormSurface project={project} spec={spec} />
      ) : (
        <Build project={project} spec={spec} />
      )}
    </div>
  );
}

function Idle({ spec, onRun }: { spec: ArtifactSpec; onRun: () => void }) {
  return (
    <div className="mt-4">
      <p className="max-w-[66ch] whitespace-pre-wrap text-mk-body-lg leading-[1.9] text-mk-ink">
        <Bold text={spec.note} />
      </p>
      <div
        className="mt-4 rounded-mk-lg border p-4"
        style={{ borderColor: "var(--mk-border)", background: "var(--mk-paper)" }}
      >
        <Sys>我接下来会做</Sys>
        <ul className="mt-2 space-y-1">
          {spec.steps.map((s) => (
            <li key={s} className="flex gap-2 text-mk-small leading-[1.75] text-mk-secondary">
              <span className="text-mk-faint">·</span>
              {s.replace(/在(.*)……/, "$1")}
            </li>
          ))}
        </ul>
      </div>
      <Btn className="mt-4" iconStart={<Play size={15} strokeWidth={2} />} onClick={onRun}>
        让印记开始
      </Btn>
    </div>
  );
}

function Working({ spec, line }: { spec: ArtifactSpec; line: number }) {
  return (
    <div className="mt-6">
      <ol className="space-y-2.5">
        {spec.steps.map((s, i) => {
          const done = i < line;
          const now = i === line;
          return (
            <li
              key={s}
              className="flex items-center gap-2.5 text-mk-body"
              style={{ opacity: i > line ? 0.34 : 1, transition: "opacity 260ms" }}
            >
              {done ? (
                <Check size={15} strokeWidth={2.6} color="var(--mk-success)" />
              ) : now ? (
                <Loader size={15} strokeWidth={2} className="eco-spin text-mk-accent-700" />
              ) : (
                <span className="h-[15px] w-[15px] rounded-mk-full border border-mk-border" />
              )}
              <span className={cx(done ? "text-mk-muted" : "text-mk-ink")}>{s}</span>
            </li>
          );
        })}
      </ol>
      <p className="mt-5 text-mk-small leading-[1.8] text-mk-muted">
        这一步由我完成。做完之后你只需要做一件事：查看，然后指出哪里不对。
      </p>
    </div>
  );
}

/* ── options: pick one, say why ─────────────────────────────────────────── */

function Options({ project, spec }: { project: Project; spec: ArtifactSpec }) {
  const { setArtifactChoice, setArtifactWhy, settleArtifact } = useEco();
  const st = project.artifacts[spec.id];
  const chosen = spec.options?.find((o) => o.id === st?.choice);
  const why = st?.why ?? "";
  const ready = Boolean(chosen) && why.trim().length >= 6;
  const rejected = spec.options?.filter((o) => o.id !== st?.choice) ?? [];

  return (
    <div className="mt-4">
      <p className="max-w-[66ch] whitespace-pre-wrap text-mk-body-lg leading-[1.9] text-mk-ink">
        <Bold text={spec.note} />
      </p>

      <ul className="mt-5 grid gap-4 md:grid-cols-3">
        {spec.options?.map((o) => (
          <li key={o.id}>
            <button
              type="button"
              onClick={() => setArtifactChoice(project.id, spec.id, o.id)}
              className={cx(
                "h-full w-full overflow-hidden rounded-mk-lg border text-left transition-all duration-[140ms]",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                st?.choice === o.id
                  ? "border-mk-accent shadow-mk-md"
                  : "border-mk-border hover:border-mk-accent-200",
              )}
            >
              <Miniature option={o} />
              <span className="block p-4">
                <span className="flex items-center gap-2">
                  <span className="eco-mono text-mk-faint">{o.id}</span>
                  <span className="text-mk-h3 text-mk-ink">{o.name}</span>
                </span>
                <span className="mt-0.5 block text-mk-small text-mk-muted">{o.tag}</span>
                <span className="mt-2 block space-y-1">
                  {o.bullets.map((b) => (
                    <span key={b} className="block text-mk-small leading-[1.75] text-mk-secondary">
                      · {b}
                    </span>
                  ))}
                </span>
              </span>
            </button>
          </li>
        ))}
      </ul>

      <div className="mt-5 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
        <p className="text-mk-body-lg leading-[1.9] text-mk-ink">{spec.ask}</p>
        <div className="mt-3.5">
          <Field
            label={
              chosen
                ? `为什么是「${chosen.name}」，为什么不是另外两版`
                : "先在上面挑一版"
            }
            hint={
              rejected.length === 2
                ? `另外两版是「${rejected[0]?.name}」和「${rejected[1]?.name}」。说说它们哪里不对。`
                : "理由写得越具体，我后面越不用回来问你。"
            }
            value={why}
            onChange={(v) => setArtifactWhy(project.id, spec.id, v)}
            rows={3}
            placeholder="例如：选 B。我要的是「密」，A 太空了，一屏只放一句话像在装深沉。C 好看但我没有那种照片。"
          />
        </div>
        <Btn
          className="mt-4"
          disabled={!ready}
          iconStart={<Check size={16} strokeWidth={2.4} />}
          onClick={() =>
            chosen &&
            settleArtifact(
              project.id,
              spec.id,
              `你选了「${chosen.name}」，理由是「${why.trim().slice(0, 40)}」。\n\n这条理由我记下了——后面每一步我都按它做，你发现我做的东西和它不符，直接说。`,
            )
          }
        >
          选择这一版
        </Btn>
        {!ready ? (
          <p className="mt-2 text-mk-small text-mk-muted">请选择一个方案并说明理由后继续。</p>
        ) : null}
      </div>
    </div>
  );
}

/** A real miniature of the option, not a swatch. An option she cannot see is
 *  an option she cannot judge — and three colour chips would make the choice
 *  cosmetic, which is exactly what this card is trying not to teach. */
function Miniature({ option }: { option: ArtifactOption }) {
  return (
    <span
      className="block px-4 py-5"
      style={{ background: option.paper, color: option.ink, fontFamily: option.font }}
    >
      <span className="block text-[15px] font-bold leading-tight">{option.name}</span>
      <span
        className="mt-2 block h-[3px] w-8 rounded-mk-full"
        style={{ background: option.accent }}
      />
      <span className="mt-2.5 block space-y-1">
        {[86, 74, 92, 60].map((w, i) => (
          <span
            key={w}
            className="block h-[5px] rounded-mk-full"
            style={{
              width: `${w}%`,
              background: option.ink,
              opacity: i === 0 ? 0.42 : 0.2,
            }}
          />
        ))}
      </span>
    </span>
  );
}

/* ── draft: 印记 wrote it, she rewrites it ──────────────────────────────── */

function Draft({ project, spec }: { project: Project; spec: ArtifactSpec }) {
  const { setArtifactBlock, settleArtifact } = useEco();
  const st = project.artifacts[spec.id];
  const edits = st?.blocks ?? {};
  const changed = (spec.blocks ?? []).filter(
    (b) => (edits[b.id] ?? b.text).trim() !== b.text.trim(),
  ).length;
  const total = spec.blocks?.length ?? 0;

  return (
    <div className="mt-4">
      <p className="max-w-[66ch] whitespace-pre-wrap text-mk-body-lg leading-[1.9] text-mk-ink">
        <Bold text={spec.note} />
      </p>

      <div className="mt-5 space-y-4">
        {spec.blocks?.map((b) => {
          const v = edits[b.id] ?? b.text;
          const touched = v.trim() !== b.text.trim();
          return (
            <div
              key={b.id}
              className="rounded-mk-lg border p-4"
              style={{
                borderColor: touched ? "var(--mk-success)" : "var(--mk-border)",
                background: "var(--mk-surface)",
              }}
            >
              <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
                <Sys>{b.label}</Sys>
                <span
                  className="eco-mono"
                  style={{ color: touched ? "var(--mk-success)" : "var(--mk-faint)" }}
                >
                  {touched ? "你改过" : "印记写的"}
                </span>
              </div>
              <p className="mt-1 text-mk-small leading-[1.7] text-mk-muted">{b.hint}</p>
              <textarea
                value={v}
                rows={2}
                onChange={(e) => setArtifactBlock(project.id, spec.id, b.id, e.target.value)}
                className="mt-2 w-full resize-y rounded-mk-md border border-mk-input-border bg-mk-paper p-3
                           text-mk-prose text-mk-ink outline-none transition-colors duration-[120ms]
                           focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
              />
            </div>
          );
        })}
      </div>

      <div className="mt-5 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
        <p className="text-mk-body-lg leading-[1.9] text-mk-ink">{spec.ask}</p>
        <p className="mt-2 text-mk-body text-mk-secondary">
          现在改过 <span className="font-mono tabular-nums text-mk-ink">{changed}</span> / {total} 块。
          {changed === 0
            ? "一处都不改的话，这一页上说话的人是我。"
            : changed < 3
              ? "还有几块是我的句子。"
              : "这样差不多是你在说话了。"}
        </p>
        <Btn
          className="mt-4"
          disabled={changed === 0}
          iconStart={<Check size={16} strokeWidth={2.4} />}
          onClick={() =>
            settleArtifact(
              project.id,
              spec.id,
              `内容定了：${total} 块里你改了 ${changed} 块。\n\n改过的那几句我原样用，一个字不动——那是你的话。`,
            )
          }
        >
          确认这一版内容
        </Btn>
        {changed === 0 ? (
          <p className="mt-2 text-mk-small text-mk-muted">
            至少修改一处。这一页上留着我的句子，读者读到的就是我。
          </p>
        ) : null}
      </div>
    </div>
  );
}

/* ── build: 印记 built it, she accepts or sends it back ─────────────────── */

function Build({ project, spec }: { project: Project; spec: ArtifactSpec }) {
  const { noteBuild, settleArtifact } = useEco();
  const st = project.artifacts[spec.id];
  const rounds = spec.rounds ?? [];
  const idx = Math.min(st?.round ?? 0, rounds.length - 1);
  const round = rounds[idx];
  const last = idx >= rounds.length - 1;
  const [note, setNote] = useState("");
  const [why, setWhy] = useState("");
  const paint = paintOf(project);

  if (!round) return null;

  return (
    <div className="mt-4">
      <p className="max-w-[66ch] whitespace-pre-wrap text-mk-body-lg leading-[1.9] text-mk-ink">
        <Bold text={spec.note} />
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1">
        <Sys>第 {idx + 1} 版</Sys>
        <span className="text-mk-small text-mk-secondary">{round.changed}</span>
      </div>

      {/* 🚨 The thing itself — the real site, not a sketch of it.
          This used to be four lines of text in a fake browser frame, and a
          project whose deliverable is never actually seen teaches that the
          deliverable does not matter. `BuiltSite` here is the same component
          that serves `/eco/p/:handle`, so what she signs off on IS what a
          visitor gets. `stage` carries 印记's own admissions into the pixels:
          at 第 1 版 the headline really is too big on a phone. */}
      {spec.id === "hp-build" ? (
        <SitePreview project={project} stage={(idx + 1) as 1 | 2 | 3} />
      ) : (
        /* Every other build — the garden map, the signs — is still described
           rather than rendered. Those deliverables live outside a browser. */
        <div
          className="mt-3 overflow-hidden rounded-mk-lg border border-mk-border shadow-mk-sm"
          style={{ background: paint.paper }}
        >
          <div
            className="flex items-center gap-1.5 px-3 py-2"
            style={{ borderBottom: `1px solid color-mix(in srgb, ${paint.ink} 12%, transparent)` }}
          >
            {["#E8695E", "#E5B94E", "#5FA97E"].map((c) => (
              <span key={c} className="h-2.5 w-2.5 rounded-mk-full" style={{ background: c }} />
            ))}
            <span className="eco-mono ml-2" style={{ color: paint.ink, opacity: 0.45 }}>
              预览
            </span>
          </div>
          <div className="px-7 py-8" style={{ color: paint.ink, fontFamily: paint.font }}>
            <p className="text-[22px] font-bold leading-[1.35]">{round.headline ?? spec.title}</p>
            <span
              className="mt-3 block h-[3px] w-10 rounded-mk-full"
              style={{ background: paint.accent }}
            />
            <ul className="mt-4 space-y-2">
              {(round.lines ?? []).map((l) => (
                <li key={l} className="text-[14px] leading-[1.85]" style={{ opacity: 0.86 }}>
                  {l}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {/* 印记 names what is wrong with its own work */}
      <div
        className="mt-4 rounded-mk-lg border p-4"
        style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
      >
        <Sys className="!text-[#8A6320]">我已知的问题</Sys>
        <ul className="mt-2 space-y-1.5">
          {round.admits.map((a) => (
            <li key={a} className="flex gap-2 text-mk-body leading-[1.8] text-[#6B4D14]">
              <span>·</span>
              {a}
            </li>
          ))}
        </ul>
      </div>

      {/* her turn */}
      <div className="mt-5 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
        <p className="text-mk-body-lg leading-[1.9] text-mk-ink">{spec.ask}</p>

        {!last ? (
          <>
            <div className="mt-3.5">
              <Field
                label="哪里不对"
                hint="具体到我能直接动手改。「第二屏的字太小」我能改；「感觉怪怪的」我只能瞎猜。"
                value={note}
                onChange={setNote}
                rows={2}
                placeholder="例如：手机上第一句话断成三行了，读起来很别扭。"
              />
            </div>
            <Btn
              className="mt-3"
              disabled={note.trim().length < 4}
              iconStart={<Send size={15} strokeWidth={2} />}
              onClick={() => {
                // `noteBuild` already puts it back into `working`, which is
                // what restarts the ticker above.
                noteBuild(project.id, spec.id, note.trim());
                setNote("");
              }}
            >
              请印记修改
            </Btn>
          </>
        ) : null}

        <div className={cx("border-mk-border", last ? "" : "mt-5 border-t pt-5")}>
          <Field
            label="这一版可以定稿了吗？请说明理由"
            hint="「什么程度算完成」是这一步里属于你的判断。"
            value={why}
            onChange={setWhy}
            rows={2}
            placeholder="例如：可以了。我要的是别人扫一眼知道我在想什么，现在第一句就说清楚了。剩下的图我以后自己换。"
          />
          <Btn
            className="mt-3"
            variant={last ? "primary" : "outline"}
            disabled={why.trim().length < 6}
            iconStart={<Check size={16} strokeWidth={2.4} />}
            onClick={() =>
              settleArtifact(
                project.id,
                spec.id,
                `你确认了第 ${idx + 1} 版。你写的是「${why.trim().slice(0, 40)}」。\n\n${
                  (st?.notes.length ?? 0) > 0
                    ? `过程中你提了 ${st?.notes.length} 条具体意见，每一条我都改了。这些意见会留在项目成果里。`
                    : "你一轮就定稿了。可以，只要那是你看过之后的判断。"
                }`,
              )
            }
          >
            确认这一版
          </Btn>
        </div>
      </div>
    </div>
  );
}

/**
 * What the preview is painted in.
 *
 * Read off whichever `options` artifact she already settled, so the built
 * thing genuinely looks like the version she picked two steps ago. If a
 * preview ignored her choice, the choice would have been a quiz question.
 */
/**
 * 网站预览 — the real page, in a frame.
 *
 * ## Why there is a 手机 toggle here and nowhere else
 * 印记's first round admits 「手机上第一屏那句话会断成三行」. If she cannot look
 * at the phone, that admission is a sentence she has to take on faith, and the
 * feedback she writes back is a guess. With the toggle it is something she
 * checks. The narrow layout is a PROP rather than a media query for the same
 * reason: a `md:` breakpoint inside this frame would read the real window and
 * lay the phone preview out as a desktop.
 */
function SitePreview({ project, stage }: { project: Project; stage: 1 | 2 | 3 }) {
  const { state } = useEco();
  const [phone, setPhone] = useState(false);
  const site = buildSite({ projects: state.projects, sections: state.homepage.sections });
  const theme = siteTheme(state.projects, state.homepage.style);

  return (
    <div className="mt-3 overflow-hidden rounded-mk-lg border border-mk-border shadow-mk-sm">
      <div className="flex items-center gap-2 border-b border-mk-border bg-mk-paper px-3 py-2">
        {["#E8695E", "#E5B94E", "#5FA97E"].map((c) => (
          <span key={c} className="h-2.5 w-2.5 rounded-mk-full" style={{ background: c }} />
        ))}
        <span
          className="ml-1.5 truncate rounded-mk-full bg-mk-surface px-2.5 py-0.5 text-[11px] text-mk-muted"
          style={{ fontFamily: 'ui-monospace,"SF Mono",monospace' }}
        >
          {site.domain}
        </span>
        <span className="ml-auto flex items-center gap-1">
          {[
            { on: !phone, label: "电脑", icon: <Monitor size={13} strokeWidth={1.9} /> },
            { on: phone, label: "手机", icon: <Smartphone size={13} strokeWidth={1.9} /> },
          ].map((t) => (
            <button
              key={t.label}
              type="button"
              aria-pressed={t.on}
              onClick={() => setPhone(t.label === "手机")}
              className={cx(
                "flex items-center gap-1 rounded-mk-sm px-2 py-1 text-[12px] transition-colors",
                "duration-[120ms] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                t.on ? "bg-mk-surface text-mk-ink" : "text-mk-muted hover:text-mk-secondary",
              )}
            >
              {t.icon}
              {t.label}
            </button>
          ))}
        </span>
      </div>

      <div
        // 🚨 `items-start`: without it the phone frame is a stretched flex
        // item, and its `overflow-hidden` (there for the rounded corners) then
        // CLIPS the page at the frame's height — the preview showed the first
        // screen and nothing below it could ever be reached.
        className={cx(
          "max-h-[520px] overflow-y-auto",
          phone && "flex items-start justify-center bg-mk-paper py-5",
        )}
      >
        <div className={cx(phone && "w-[390px] overflow-hidden rounded-mk-md shadow-mk-md")}>
          <BuiltSite site={site} theme={theme} stage={stage} narrow={phone} />
        </div>
      </div>
    </div>
  );
}

function paintOf(project: Project): {
  paper: string;
  ink: string;
  accent: string;
  font?: string;
} {
  for (const [id, st] of Object.entries(project.artifacts)) {
    const spec = ARTIFACTS[id];
    if (spec?.kind !== "options" || !st.choice) continue;
    const opt = spec.options?.find((o) => o.id === st.choice);
    if (opt) return { paper: opt.paper, ink: opt.ink, accent: opt.accent, font: opt.font };
  }
  return { paper: "#FBF8F4", ink: "#33302E", accent: "#EA5140" };
}
