import { Says, errorMarkdown } from "./Says";
import { LandingHeader } from "../learning/LandingHeader";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  createProject,
  groupByStatus,
  listProjects,
  PROJECT_STATUSES,
  PROJECT_STATUS_LABELS,
  updateProject,
  type Project,
  type ProjectStatus,
} from "../api/projects";
import { navigate, projectPath } from "../routing";
import { getSite, startSiteProject, type SiteState } from "../api/site";
import { ProjectCard } from "./ProjectCard";
import { useZoneDrag } from "./tools/board/useZoneDrag";
import { DragGhost } from "./tools/board/DragGhost";
import { apiErrorText } from "../api/errorText";
import { AssignmentStrip } from "../inbox/AssignmentStrip";
import createHomepageIllustration from "../learning/assets/ideas-workbench.webp";


/**
 * ProjectsLanding — the 项目 tab's front door.
 *
 * TWO PARTS, IN THIS ORDER.
 *
 *  1. **A big input box.** The same shape as the 阅读 and 写作 front doors: she
 *     writes what she wants to do, in her own words, and 印记 works out what
 *     kind of project that is. She never picks a category off a menu — a
 *     13-year-old with an idea does not yet know whether it is "research" or
 *     "design", and making her choose first is asking her to classify a thing
 *     she has not built yet.
 *  2. **A kanban of everything she has going.** Reading and writing each
 *     concern one thing at a time, so their history lives in a shelf. Projects
 *     do not: she accumulates questions, some still being talked about, some
 *     underway, some finished, some being kept alive. The board is the honest
 *     shape of that, and it is why 项目 is one page rather than a room with a
 *     sidebar.
 *
 * The 圈点 ink-stroke flourish from `ReadingsLanding` is deliberately NOT
 * repeated here. That gesture is a reader's red pen; borrowing it for a second
 * page would turn a specific idea into product decoration.
 */
export function ProjectsLanding() {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [shake, setShake] = useState(false);
  const boxRef = useRef<HTMLTextAreaElement>(null);
  const [idea, setIdea] = useState("");
  const [creating, setCreating] = useState(false);
  // 默认看卡片：她多数时候是来找某一个项目接着做。
  const [view, setView] = useState<"cards" | "board">("cards");
  const [error, setError] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  // spec §4 的门：她的主页发布之前，这里给的是主页那一扇门，不是自由输入框。
  const [site, setSite] = useState<SiteState | null>(null);
  const [siteLoading, setSiteLoading] = useState(true);
  const [siteError, setSiteError] = useState<string | null>(null);
  const [siteAttempt, setSiteAttempt] = useState(0);
  const [opening, setOpening] = useState(false);

  /**
   * 把一个项目挪到另一列 —— 也就是「这个项目我做完了」这一下。
   *
   * 🚨 在这之前，界面上**没有任何一处**能把项目往后推。服务端五档全收
   * （`pbl_projects.go` 的白名单），审完计划会自动进「进行中」，然后就没有然后
   * 了：整个 lite 前端只有 `NameAndCover` 调过 `updateProject`，而它只改名字和
   * 封面。看板画着五列，却只有前两列进得去——她做完了也没办法说做完了。
   *
   * 为什么是拖：产品负责人 2026-09-03 那四张图里，拖是主要动词；而且这块看板
   * 本来就是「哪个项目现在在哪一档」的那张图，把卡片挪过去正是她心里那个动作。
   *
   * 先改本地再发请求：她松手那一刻卡片就该在新的一列里。请求失败就退回去，并且
   * 把后台原话显示出来（第 8 条）。
   */
  const justDragged = useRef(false);
  const move = useCallback(async (id: string, status: ProjectStatus) => {
    let before: ProjectStatus | undefined;
    setProjects((prev) => {
      if (!prev) return prev;
      before = prev.find((p) => p.id === id)?.status;
      return prev.map((p) => (p.id === id ? { ...p, status } : p));
    });
    try {
      await updateProject(id, { status });
    } catch (err) {
      setError(`移动失败：${apiErrorText(err)}`);
      setProjects((prev) =>
        prev && before ? prev.map((p) => (p.id === id ? { ...p, status: before! } : p)) : prev,
      );
    }
  }, []);

  const drag = useZoneDrag({
    onDrop: (id, zone) => {
      justDragged.current = true;
      window.setTimeout(() => (justDragged.current = false), 0);
      if (!zone) return;
      const now = projects?.find((p) => p.id === id)?.status;
      if (now === zone) return;
      void move(id, zone as ProjectStatus);
    },
  });

  /** Set once a project exists on the server and she has yet to name it. */

  useEffect(() => {
    let cancelled = false;
    setSiteLoading(true);
    setSiteError(null);
    getSite()
      .then((s) => {
        if (!cancelled) setSite(s);
      })
      .catch((err) => {
        if (!cancelled) setSiteError(apiErrorText(err));
      })
      .finally(() => {
        if (!cancelled) setSiteLoading(false);
      });
    return () => { cancelled = true; };
  }, [siteAttempt]);

  useEffect(() => {
    let cancelled = false;
    listProjects()
      .then((rows) => {
        if (!cancelled) setProjects(rows);
      })
      .catch((err) => {
        if (!cancelled) {
          setProjects([]);
          setLoadError(apiErrorText(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const columns = useMemo(() => groupByStatus(projects ?? []), [projects]);
  // 门开着 = 她的主页真的在线上。开着没做的主页项目挡不住任何事。
  const siteReady = site?.published === true;

  async function handleStart() {
    const text = idea.trim();
    if (creating) return;
    // 🚨 空着就按「开始」：抖一下，把光标送回输入框。
    //
    // 原来是把按钮置灰。置灰是一堵沉默的墙——她点不动，没人告诉她为什么，也
    // 没人指给她看该往哪写。灰字里已经写着要写什么了，抖一下就够，不用再加
    // 一句红字（产品负责人 2026-09-02，文案表 §7.2）。
    if (!text) {
      setShake(true);
      boxRef.current?.focus();
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const created = await createProject(text);
      setProjects((prev) => [created, ...(prev ?? [])]);
      setIdea("");
      // 直接进房间。产品负责人 2026-09-02：名字等问题想清楚了再改，别在她刚
      // 写下一句话、还什么都没想明白的时候拦住她要一个名字。她写的那句话进去
      // 就是她对印记说的第一句话。
      navigate(projectPath(created.id));
    } catch (err) {
      // 印记 failing to read her idea is surfaced, never smoothed over into a
      // project with a guessed type.
      setError(apiErrorText(err));
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="learning-landing learning-projects relative min-h-full">
      <div className="learning-landing-measure learning-landing-wide relative mx-auto flex w-full flex-col">
        {/* ── 1 · 门，或者输入框 ───────────────────────────────────────
            spec §4：她还没有主页的时候，第一个项目就是做一个。**不是推荐，
            是第一个项目就是它。** 产品负责人 2026-09-03 把它定成一道完整的门。

            以前这条规则在这一页上只是一句会自己消失的灰字提示，什么都没拦住，
            所以从来没有人触发过它。现在门关着的时候，这里给的就是那一扇门。

            🚨 看板照常显示。门是在这道输入框上，不在她已经有的东西上——已经
            建了项目、还没有主页的学生（门上线之前的每一个人）不能因为这次改动
            就进不去自己的项目。 */}
        <LandingHeader kind="project" title="项目" description="从真实的问题出发，规划、实践并记录你的成果。" />
        {/* ── 作业 ──────────────────────────────────────────────────────
            Above the gate, not inside it: a project the teacher assigned is
            reachable while her homepage is still unpublished. Renders nothing
            when no project is assigned and still open. */}
        <AssignmentStrip kind="project" className="mb-8 w-full max-w-[760px]" />
        {siteLoading ? (
          <p className="mt-6 text-mk-body text-mk-secondary" role="status">正在加载项目入口…</p>
        ) : siteError ? (
          <div className="learning-project-gate">
            <div role="alert" className="text-mk-body text-mk-secondary"><Says content={errorMarkdown(`读取主页状态失败：${siteError}`)} /></div>
            <button type="button" className="mt-4 text-mk-body font-semibold text-mk-accent-500" onClick={() => setSiteAttempt((n) => n + 1)}>重新加载</button>
          </div>
        ) : siteReady ? (

          <div className="learning-project-start w-full">
            <h2 className="text-mk-h2 text-mk-ink">新建项目</h2>
            {/* 🚨 原来写的是「一句话就行。想清楚要做什么，是我们一起的第一
                件事。」——「一句话就行」正是文案第 7 条点名要删的那种替她减压
                的话（它先假设了她怕）。改成先说这件事为什么值得做（第 5 条），
                再请她做（第 3 条）。 */}
            <p className="mt-3 text-mk-body text-mk-secondary">
              项目从一个真实的问题开始。请描述你想弄明白或想改变的那件事。
            </p>

            <div
              onAnimationEnd={() => setShake(false)}
              className={`mt-7 rounded-mk-lg border border-mk-border bg-mk-surface p-3 shadow-mk-sm focus-within:border-mk-accent-200${
                shake ? " mk-shake" : ""
              }`}
            >
              <textarea
                ref={boxRef}
                value={idea}
                onChange={(e) => setIdea(e.target.value)}
                rows={4}
                disabled={creating}
                placeholder="比如：我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。"
                className="w-full resize-none bg-transparent text-mk-prose text-mk-ink outline-none placeholder:text-mk-faint"
              />
              {/* Stacked on a phone. Side by side, the hint wrapped to two lines
                  and squeezed the button until 开始 broke across two lines as
                  「开 / 始」 — a two-character button cannot be allowed to wrap,
                  so the button is nowrap and the row stops competing for width
                  below `sm`. */}
              <div className="flex flex-col items-stretch gap-2 pt-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
                <span className="order-2 text-mk-small text-mk-muted sm:order-1">
                  {/* 界面负责给东西命名，不替印记说话（第 0 条）：这里说的是
                      按下去会发生什么，不是印记在跟她搭话。 */}
                  {creating ? "处理中" : "开始后，先和印记确定这个项目要解决什么"}
                </span>
                <button
                  type="button"
                  onClick={handleStart}
                  disabled={creating}
                  className="order-1 flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-40 sm:order-2"
                  style={{ background: "var(--mk-accent-500)" }}
                >
                  开始
                  <Icon icon={ArrowRight} size={15} />
                </button>
              </div>
            </div>

            {error && (
              <div className="mt-3 text-center text-mk-small" style={{ color: "var(--mk-danger)" }}>
                <Says content={errorMarkdown(error)} />
              </div>
            )}
          </div>
        ) : (
          <div className="learning-project-gate learning-project-gate-illustrated">
            <div className="learning-project-gate-copy">
            <p className="text-mk-small text-mk-muted">第一个项目</p>
            <h2 className="mt-2 text-mk-h2 text-mk-ink">创建个人主页</h2>
            <p className="mt-4 text-mk-body text-mk-secondary">
              用一页网站向你选择的读者介绍兴趣或作品。你决定给谁看、展示什么、怎样邀请对方参与；印记协助整理结构、设计页面和执行修改。
            </p>
            <p className="mt-3 text-mk-body text-mk-secondary">从确定读者开始，自由描述喜欢的感觉，构思并试用第一幕，再介绍自己与展示作品。</p>
            <button
              type="button"
              disabled={opening}
              onClick={async () => {
                setOpening(true);
                setError(null);
                try {
                  const p = await startSiteProject();
                  navigate(projectPath(p.id));
                } catch (err) {
                  setError(apiErrorText(err));
                  setOpening(false);
                }
              }}
              className="mt-8 inline-flex items-center justify-center gap-1.5 rounded-mk-full px-5 py-2.5 text-mk-body font-semibold text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              {site?.projectId ? "继续制作主页" : "开始制作主页"}
              <Icon icon={ArrowRight} size={15} />
            </button>

            <p className="mt-6 text-mk-small text-mk-muted">
              主页发布后可创建其他项目。发布前可以预览和修改，并由你决定是否公开。
            </p>

            {error && (
              <div className="mt-4 text-mk-small" style={{ color: "var(--mk-danger)" }}>
                <Says content={errorMarkdown(error)} />
              </div>
            )}
            </div>
            <img className="learning-project-gate-art" src={createHomepageIllustration} alt="" />
          </div>
        )}

        {/* ── 2 · the board ───────────────────────────────────────────── */}
        {loadError && (
          <div className="mt-10 text-center text-mk-small" style={{ color: "var(--mk-danger)" }}>
            <Says content={errorMarkdown(loadError)} />
          </div>
        )}

        {projects !== null && projects.length > 0 && (
          <div className="mt-16">
            {/* 视图切换。看板不再是唯一的画法——产品负责人 2026-09-02：
                「make kanban a new view please」。默认是卡片：她多数时候是来
                找某一个项目接着做，不是来看它们分布在哪几档。 */}
            <div className="mb-4 flex items-center gap-1">
              {(["cards", "board"] as const).map((v) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => setView(v)}
                  className="rounded-mk-full px-3 py-1 text-mk-small"
                  style={
                    view === v
                      ? { background: "var(--mk-accent-500)", color: "#fff" }
                      : { color: "var(--mk-secondary)" }
                  }
                >
                  {v === "cards" ? "全部" : "按状态"}
                </button>
              ))}
              <span className="ml-auto text-mk-small text-mk-faint">
                {projects.length} 个项目
              </span>
            </div>

            {view === "cards" ? (
              // 最近动过的排最前——服务端已经按 last_activity_at 倒序给了。
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {projects.map((p) => (
                  <ProjectCard key={p.id} project={p} onOpen={(x) => navigate(projectPath(x.id))} />
                ))}
              </div>
            ) : (
              // 列一起横向滚动；页面本身永远不横滚。
              <div className="-mx-4 overflow-x-auto px-4 sm:-mx-6 sm:px-6">
                <div className="flex min-w-[1180px] gap-4">
                  {PROJECT_STATUSES.map((status) => (
                    <section
                      key={status}
                      ref={drag.zoneRef(status)}
                      data-testid={`status-col-${status}`}
                      className="flex w-full min-w-[228px] flex-col gap-3 rounded-mk-lg p-1"
                      style={
                        drag.drag?.over === status
                          ? { boxShadow: "inset 0 0 0 2px var(--mk-accent-500)" }
                          : undefined
                      }
                    >
                      <header className="flex items-baseline justify-between border-b border-mk-border pb-2">
                        <h2 className="text-mk-label text-mk-secondary">
                          {PROJECT_STATUS_LABELS[status]}
                        </h2>
                        <span className="text-mk-small text-mk-faint">{columns[status].length}</span>
                      </header>
                      <div className="flex flex-col gap-3">
                        {columns[status].map((p) => (
                          <ProjectCard
                            key={p.id}
                            project={p}
                            testId={`project-card-${p.id}`}
                            dim={drag.drag?.id === p.id}
                            onPointerDown={(e) => drag.start(p.id, e)}
                            onOpen={(x) => {
                              // 🚨 刚拖完那一下不算点击。真拖过之后浏览器不会在
                              // 原元素上再发 click，这个 ref 兜的是"在同一张卡上
                              // 小幅挪了一下"——见 memory ·
                              // interaction-means-a-board（Decide 上吃过一次）。
                              if (justDragged.current) return;
                              navigate(projectPath(x.id));
                            }}
                          />
                        ))}
                      </div>
                    </section>
                  ))}
                </div>
                {/* 手里拿着的那张。position: fixed，否则拖出这一列就被
                    overflow 裁掉了——见 DragGhost 顶上那段。 */}
                <DragGhost drag={drag.drag}>
                  {(() => {
                    const p = projects?.find((x) => x.id === drag.drag?.id);
                    return p ? <ProjectCard project={p} onOpen={() => undefined} /> : null;
                  })()}
                </DragGhost>
              </div>
            )}
          </div>
        )}
      </div>

    </div>
  );
}
