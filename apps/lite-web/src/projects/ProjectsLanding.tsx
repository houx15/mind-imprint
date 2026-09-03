import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  createProject,
  groupByStatus,
  listProjects,
  PROJECT_STATUSES,
  PROJECT_STATUS_LABELS,
  type Project,
} from "../api/projects";
import { navigate, projectPath } from "../routing";
import { getSite, startSiteProject, type SiteState } from "../api/site";
import { ProjectCard } from "./ProjectCard";
import { apiErrorText } from "../api/errorText";

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
  const [opening, setOpening] = useState(false);
  /** Set once a project exists on the server and she has yet to name it. */

  useEffect(() => {
    let cancelled = false;
    getSite()
      .then((s) => {
        if (!cancelled) setSite(s);
      })
      .catch(() => {
        // 读不到主页状态时按「门关着」处理。反过来（默认门开着）会让她建出一个
        // 服务端一定会拒掉的项目，那是一次白写的输入。
        if (!cancelled) setSite(null);
      });
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
    <div className="relative min-h-full">
      <div className="relative mx-auto flex w-full max-w-[1100px] flex-col px-4 pb-20 pt-16 sm:px-6">
        {/* ── 1 · 门，或者输入框 ───────────────────────────────────────
            spec §4：她还没有主页的时候，第一个项目就是做一个。**不是推荐，
            是第一个项目就是它。** 产品负责人 2026-09-03 把它定成一道完整的门。

            以前这条规则在这一页上只是一句会自己消失的灰字提示，什么都没拦住，
            所以从来没有人触发过它。现在门关着的时候，这里给的就是那一扇门。

            🚨 看板照常显示。门是在这道输入框上，不在她已经有的东西上——已经
            建了项目、还没有主页的学生（门上线之前的每一个人）不能因为这次改动
            就进不去自己的项目。 */}
        {siteReady ? (
          <div className="mx-auto w-full max-w-[720px]">
            <h1 className="text-center text-mk-display text-mk-ink">最近想做点什么</h1>
            <p className="mt-3 text-center text-mk-body text-mk-secondary">
              一句话就行。想清楚要做什么，是我们一起的第一件事。
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
                  {creating ? "处理中" : "写完按开始，印记会先跟你聊清楚要做什么。"}
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
              <p className="mt-3 text-center text-mk-small" style={{ color: "var(--mk-danger)" }}>
                {error}
              </p>
            )}
          </div>
        ) : (
          <div className="mx-auto w-full max-w-[640px] text-center">
            <h1 className="text-mk-display text-mk-ink">先做你自己的主页。</h1>
            <p className="mt-4 text-mk-body text-mk-secondary">
              往后你读的、写的、做的都要有地方放，那个地方得先存在。它也是你第一次
              把一件东西真正做到能给别人看。
            </p>
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
              {site?.projectId ? "接着做我的主页" : "开始做我的主页"}
              <Icon icon={ArrowRight} size={15} />
            </button>

            <p className="mt-6 text-mk-small text-mk-muted">
              做完发布之后，这里就能开别的项目了。
            </p>

            {error && (
              <p className="mt-4 text-mk-small" style={{ color: "var(--mk-danger)" }}>
                {error}
              </p>
            )}
          </div>
        )}

        {/* ── 2 · the board ───────────────────────────────────────────── */}
        {loadError && (
          <p className="mt-10 text-center text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {loadError}
          </p>
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
                    <section key={status} className="flex w-full min-w-[228px] flex-col gap-3">
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
                            onOpen={(x) => navigate(projectPath(x.id))}
                          />
                        ))}
                      </div>
                    </section>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

    </div>
  );
}
