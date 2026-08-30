import { useState } from "react";
import { ArrowLeft, Check, Globe, MessageCircle, Sparkles } from "lucide-react";
import { useEco } from "../store";
import { STEP_META, planHours, trackById } from "../data/projects";
import { go } from "../route";
import { Btn, Empty, Field, Panel, SectionHead, Sys, cx } from "../ui";
import { StepWorkspace } from "./StepWorkspace";

/**
 * 一个项目 — its route, and one step's workspace.
 *
 * ## The 动机卡 is pinned, not filed
 * Her three why-answers sit at the top of the project, permanently. Around
 * step three she will want to quit; the thing that gets her past it is reading
 * her own sentence about who this is for. Filing the motivation away in a tab
 * would defeat the reason we spent twenty minutes collecting it.
 *
 * ## Publish closes the loop
 * The last step publishes, which flips the project to `published` — and the
 * personal page renders published projects. That is the ecosystem's closing
 * edge: 世界 → 树 → 项目 → 主页.
 */
export function ProjectRoom({ id, stepId }: { id: string; stepId?: string }) {
  const { state, toggleStep, publishProject, openCoach } = useEco();
  const project = state.projects.find((p) => p.id === id);
  const [summary, setSummary] = useState("");

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

  if (stepId) {
    const step = project.steps.find((s) => s.id === stepId);
    if (!step) {
      return (
        <div className="mx-auto max-w-[720px] px-8 py-16">
          <Empty
            title="找不到这一步"
            body="它可能被删掉了。"
            action={<Btn onClick={() => go({ name: "project", id })}>回项目</Btn>}
          />
        </div>
      );
    }
    return <StepWorkspace project={project} step={step} />;
  }

  const t = trackById(project.track);
  const done = project.steps.filter((s) => s.done).length;
  const allDone = done === project.steps.length && project.steps.length > 0;
  const remaining = project.steps.filter((s) => !s.done);

  return (
    <div className="mx-auto max-w-[900px] px-8 py-8">
      <Btn variant="quiet" size="sm" iconStart={<ArrowLeft size={15} />} onClick={() => go({ name: "projects" })}>
        项目
      </Btn>

      <div className="mt-4 flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-mk-full" style={{ background: t.hue }} />
            <Sys>
              {t.label} · 开始于 {project.startedAt}
            </Sys>
          </div>
          <h1 className="mt-1.5 text-mk-display text-mk-ink">{project.title}</h1>
        </div>
        <div className="flex items-center gap-3">
          <div className="text-right">
            <Sys>进度</Sys>
            <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
              {done}/{project.steps.length}
            </span>
          </div>
          <Btn variant="outline" size="sm" iconStart={<MessageCircle size={15} />} onClick={() => openCoach("projects")}>
            问印记
          </Btn>
        </div>
      </div>

      {/* 动机卡 — pinned */}
      {project.motivation ? (
        <div
          className="mt-5 rounded-mk-lg border p-5"
          style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
        >
          <div className="flex items-center gap-2">
            <Sparkles size={15} strokeWidth={2} color="#C9962B" />
            <Sys className="!text-[#8A6320]">你当初说 · 动机卡</Sys>
          </div>
          <div className="mt-3 grid gap-4 sm:grid-cols-3">
            {[
              ["为谁做", project.motivation.who],
              ["不做的代价", project.motivation.cost],
              ["为什么是我", project.motivation.mine],
            ].map(([k, v]) => (
              <div key={k}>
                <p className="text-mk-small font-semibold text-[#8A6320]">{k}</p>
                <p className="mt-1 text-mk-body leading-[1.85] text-[#6B4D14]">{v || "—"}</p>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {/* progress bar */}
      <div className="mt-6 flex items-center gap-3">
        <div className="h-2 flex-1 overflow-hidden rounded-mk-full" style={{ background: "var(--mk-border)" }}>
          <div
            className="h-full rounded-mk-full transition-all duration-500 ease-mk"
            style={{ width: `${(done / project.steps.length) * 100}%`, background: t.hue }}
          />
        </div>
        <span className="font-mono text-mk-small text-mk-muted">
          还剩 {planHours(remaining)}
        </span>
      </div>

      {/* steps */}
      <div className="mt-7">
        <SectionHead index="路线 · PLAN" title="接下来做什么" sub="点开任何一步，进它自己的工作台。" />
        <ol className="space-y-2">
          {project.steps.map((s, i) => {
            const m = STEP_META[s.kind];
            const next = !s.done && project.steps.slice(0, i).every((x) => x.done);
            return (
              <li key={s.id}>
                <div
                  className={cx(
                    "flex items-start gap-3 rounded-mk-lg border p-4 transition-colors duration-[140ms]",
                    next ? "border-mk-accent-200 bg-mk-accent-50" : "border-mk-border bg-mk-surface",
                    s.done && "opacity-70",
                  )}
                >
                  <button
                    type="button"
                    onClick={() => toggleStep(project.id, s.id)}
                    className={cx(
                      "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-mk-full border transition-colors",
                      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                      s.done ? "border-mk-success bg-mk-success" : "border-mk-input-border hover:border-mk-accent",
                    )}
                    aria-label={s.done ? "标记为未完成" : "标记为完成"}
                  >
                    {s.done ? <Check size={14} strokeWidth={3} color="#fff" /> : null}
                  </button>

                  <button
                    type="button"
                    onClick={() => go({ name: "project", id: project.id, stepId: s.id })}
                    className="min-w-0 flex-1 text-left focus-visible:outline-none"
                  >
                    <span className="flex flex-wrap items-baseline gap-2">
                      <span
                        className={cx("text-mk-h3", s.done ? "text-mk-muted line-through" : "text-mk-ink")}
                      >
                        {s.title}
                      </span>
                      <span
                        className="rounded-mk-full px-2 py-0.5 text-[11px] font-medium"
                        style={{ background: `color-mix(in srgb, ${m.hue} 24%, transparent)`, color: "var(--mk-secondary)" }}
                      >
                        {m.glyph} {m.label}
                      </span>
                      <span className="font-mono text-[11px] text-mk-faint">{s.minutes} 分钟</span>
                      {next ? (
                        <span className="eco-mono rounded-mk-full bg-mk-accent px-2 py-0.5 text-white" style={{ letterSpacing: 0 }}>
                          下一步
                        </span>
                      ) : null}
                    </span>
                    <span className="mt-1 block text-mk-body leading-[1.8] text-mk-secondary">
                      <span className="text-mk-muted">为什么有这一步：</span>
                      {s.why}
                    </span>
                  </button>
                </div>
              </li>
            );
          })}
        </ol>
      </div>

      {/* publish */}
      {project.status !== "published" ? (
        <Panel className="mt-8 p-6">
          <Sys>做完了就发布</Sys>
          <h3 className="mt-1 text-mk-h1 text-mk-ink">把它放到我的主页上</h3>
          <p className="mt-2 max-w-[58ch] text-mk-body leading-[1.9] text-mk-secondary">
            {allDone
              ? "所有步骤都打勾了。写一段话说说你做出了什么——这段会出现在你的主页上。"
              : `还有 ${project.steps.length - done} 步没做完。你也可以现在就发布，把它标成「进行中」——原则 04 说：半成品可以发布，只要说清楚它是半成品。`}
          </p>
          <div className="mt-4">
            <Field
              label="做出了什么"
              hint="两三句话。写你真的做出了什么、遇到了什么、改了什么。"
              value={summary}
              onChange={setSummary}
              rows={4}
              placeholder="例如：我发了 87 份问卷，收回 61 份。最意外的是……"
            />
          </div>
          <Btn
            className="mt-4"
            iconStart={<Globe size={16} strokeWidth={1.9} />}
            disabled={summary.trim().length < 4}
            onClick={() => {
              publishProject(project.id, summary.trim());
              // Only send her to the page if there IS one. Publishing into a
              // homepage that hasn't been built yet used to land her on
              // 「这个主页还没建好」, which reads as the publish having failed.
              if (state.homepage.published) go({ name: "page", handle: "zhiyao" });
            }}
          >
            发布这个项目
          </Btn>
          {!state.homepage.published ? (
            <p className="mt-2 text-mk-small text-mk-muted">
              你的主页还没建好。这个项目会先挑好、等在那里——主页一发布，它就在上面了。
            </p>
          ) : null}
        </Panel>
      ) : (
        <Panel className="mt-8 p-6">
          <div className="flex items-center gap-2">
            <Globe size={16} strokeWidth={1.9} color="var(--mk-success)" />
            <Sys className="!text-mk-success">已发布</Sys>
          </div>
          <p className="mt-2 text-mk-body-lg leading-[1.9] text-mk-ink">{project.summary}</p>
          <Btn className="mt-4" variant="outline" onClick={() => go({ name: "page", handle: "zhiyao" })}>
            去我的主页看看
          </Btn>
        </Panel>
      )}
    </div>
  );
}
