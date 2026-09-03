import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, Check, Link2, Monitor, Smartphone } from "lucide-react";
import { Icon } from "@/ui";
import {
  getSite,
  publishSite,
  putSiteContent,
  putSiteLayout,
  revokeSite,
  type SiteState,
} from "../api/site";
import { apiErrorText } from "../api/errorText";
import { navigate } from "../routing";
import { BuiltSite } from "../site/BuiltSite";
import { SITE_LAYOUTS } from "../site/themes";
import {
  EMPTY_CONTENT,
  EMPTY_DRAFT,
  normalizeDraft,
  type SiteContent,
  type SiteDraft,
  type SiteLayout,
} from "../site/types";

/**
 * SiteStudio — 主页项目的工作面。
 *
 * 这个文件是 app 的外壳，所以它**可以**用 `@/ui` 和 `mk-*`；渲染出来的那一页
 * （`site/` 下面那几个文件）不可以。两者的边界就是 `<BuiltSite>` 这一个标签。
 *
 * ## 三件事，按她真实的顺序
 *
 * 1. **样子** — 三个版式，每一个都用**她自己的内容真画出来**。
 *
 *    🚨 这是原型最严重的两个缺陷之一的修法。原型在这一步给的是三条灰色骨架
 *    条，真正的页面要到第六步才出现——她是在为一个自己没见过的东西写理由。
 *    三个版式是三个真正不同的页面，那就得让她看见三个页面。
 *
 * 2. **内容** — 她写的字。左边写，右边实时看见。
 * 3. **发布** — 缺什么服务端说了算（`missing`），补齐了才给链接。
 *
 * 预览和真实页面是**同一个组件**。预览要是另一个组件，它迟早会变成一句谎话。
 */
export function SiteStudio({ projectId }: { projectId: string }) {
  const [state, setState] = useState<SiteState | null>(null);
  const [draft, setDraft] = useState<SiteDraft>(EMPTY_DRAFT);
  // 🚨 先写内容，再挑版式。
  //
  // 原型让她在第四步挑版式、第五步才写内容，于是她是在三条灰色骨架之间做选择。
  // 这一版三个版式都是真页面，但如果她还一个字都没写，那三张预览还是几乎空的
  // ——她仍然看不见自己的页面，只看得见三个空壳。写在前面，她挑的就是**装着她
  // 自己的话的那三页**，spec §15 要的「她写了理由的那个选择必须看得见」才真的
  // 成立。
  const [step, setStep] = useState<"look" | "words" | "ship">("words");
  const [narrow, setNarrow] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getSite()
      .then((s) => {
        if (cancelled) return;
        setState(s);
        setDraft(normalizeDraft(s.draft));
        // 已经发布过的，回来多半是要改或者要那条链接。
        if (s.published) setStep("ship");
      })
      .catch((err) => {
        if (!cancelled) setError(apiErrorText(err));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  /**
   * 预览用的内容：服务端合成好的那一份，叠上她此刻还没保存的字。
   *
   * 列表（文章 / 作品 / 在读）永远来自服务端——那些是她真做过的事，前端没有第
   * 二个来源。这里覆盖的只有她正在敲的那些字段，和 Go 的 BuildSite 一一对应。
   */
  const preview: SiteContent = useMemo(() => {
    const base = state?.content ?? EMPTY_CONTENT;
    const blurb = (id: string) => (draft.blurbs[id] ?? "").trim();
    return {
      ...base,
      role: draft.role.trim(),
      headline: draft.headline.trim(),
      lead: draft.lead.trim(),
      now: draft.now.trim(),
      motto: draft.motto.filter((s) => s.trim()),
      tags: draft.tags.filter((s) => s.trim()),
      about: draft.about.filter((s) => s.trim()),
      nowList: draft.nowList.filter((s) => s.trim()),
      email: draft.contact.trim(),
      posts: base.posts.map((p) => ({ ...p, blurb: blurb(p.id) })),
      projects: base.projects.map((p) => ({ ...p, blurb: blurb(p.id) })),
      reads: base.reads.map((r) => ({ ...r, takeaway: blurb(r.id) })),
    };
  }, [state, draft]);

  const save = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const next = await putSiteContent(draft);
      setState(next);
      setSaved(true);
      window.setTimeout(() => setSaved(false), 1600);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }, [draft]);

  if (error && !state) {
    return (
      <div className="mx-auto max-w-[720px] px-6 py-20 text-center">
        <p className="text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      </div>
    );
  }
  if (!state) return <div className="min-h-full" />;

  const layout = state.layout;
  const chosen = state.layoutWhy.trim() !== "";

  return (
    <div className="relative min-h-full">
      <div className="mx-auto flex w-full max-w-[1240px] flex-col px-4 pb-24 pt-8 sm:px-6">
        <header className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => navigate("/projects")}
            className="flex items-center gap-1.5 text-mk-small text-mk-secondary"
          >
            <Icon icon={ArrowLeft} size={15} />
            项目
          </button>
          <h1 className="text-mk-title text-mk-ink">我自己的主页</h1>
          {state.published ? (
            <span className="rounded-mk-full px-2.5 py-0.5 text-mk-small" style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-600)" }}>
              已发布
            </span>
          ) : null}
        </header>

        <nav className="mt-6 flex items-center gap-1">
          {(
            [
              ["words", "内容"],
              ["look", "样子"],
              ["ship", "发布"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              onClick={() => setStep(id)}
              className="rounded-mk-full px-3.5 py-1.5 text-mk-small"
              style={
                step === id
                  ? { background: "var(--mk-accent-500)", color: "#fff" }
                  : { color: "var(--mk-secondary)" }
              }
            >
              {label}
            </button>
          ))}
          <div className="ml-auto flex items-center gap-1">
            {(
              [
                ["desk", Monitor, false],
                ["phone", Smartphone, true],
              ] as const
            ).map(([id, icon, isNarrow]) => (
              <button
                key={id}
                type="button"
                aria-label={isNarrow ? "手机" : "电脑"}
                onClick={() => setNarrow(isNarrow)}
                className="rounded-mk-full p-1.5"
                style={
                  narrow === isNarrow
                    ? { background: "var(--mk-accent-100)", color: "var(--mk-accent-600)" }
                    : { color: "var(--mk-faint)" }
                }
              >
                <Icon icon={icon} size={15} />
              </button>
            ))}
          </div>
        </nav>

        {error ? (
          <p className="mt-4 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {error}
          </p>
        ) : null}

        {step === "look" ? (
          <LookStep
            site={preview}
            current={layout}
            chosen={chosen}
            currentWhy={state.layoutWhy}
            onPick={async (id, why) => {
              setBusy(true);
              setError(null);
              try {
                setState(await putSiteLayout(id, why));
                setStep("words");
              } catch (err) {
                setError(apiErrorText(err));
              } finally {
                setBusy(false);
              }
            }}
            busy={busy}
          />
        ) : null}

        {step === "words" ? (
          <WordsStep
            draft={draft}
            setDraft={setDraft}
            site={preview}
            layout={layout}
            narrow={narrow}
            onSave={save}
            busy={busy}
            saved={saved}
          />
        ) : null}

        {step === "ship" ? (
          <ShipStep
            state={state}
            site={preview}
            narrow={narrow}
            busy={busy}
            onPublish={async () => {
              setBusy(true);
              setError(null);
              try {
                await publishSite();
                setState(await getSite());
              } catch (err) {
                setError(apiErrorText(err));
              } finally {
                setBusy(false);
              }
            }}
            onRevoke={async () => {
              setBusy(true);
              setError(null);
              try {
                await revokeSite();
                setState(await getSite());
              } catch (err) {
                setError(apiErrorText(err));
              } finally {
                setBusy(false);
              }
            }}
          />
        ) : null}
      </div>
    </div>
  );
}

/* ── 样子 ─────────────────────────────────────────────────────────────── */

/**
 * 三个版式，每一个都真画出来。
 *
 * 缩放而不是简化：`Frame` 把一个 1100px 宽的真实页面缩到卡片里，所以她看到的
 * 就是那一页，只是小。做一份「示意图」等于又造了一个会和真实页面走散的东西。
 */
function LookStep({
  site,
  current,
  chosen,
  currentWhy,
  onPick,
  busy,
}: {
  site: SiteContent;
  current: SiteLayout;
  chosen: boolean;
  currentWhy: string;
  onPick: (id: SiteLayout, why: string) => void;
  busy: boolean;
}) {
  const [picked, setPicked] = useState<SiteLayout | null>(chosen ? current : null);
  const [why, setWhy] = useState(currentWhy);

  return (
    <div className="mt-8">
      <p className="text-mk-body text-mk-secondary">
        三个版式是三个不一样的页面。下面是用你自己的内容渲染的效果，请挑一个。
      </p>

      <div className="mt-6 grid gap-5 lg:grid-cols-3">
        {SITE_LAYOUTS.map((opt) => {
          const active = picked === opt.id;
          return (
            <button
              key={opt.id}
              type="button"
              onClick={() => setPicked(opt.id)}
              className="overflow-hidden rounded-mk-lg border text-left transition-colors"
              style={{
                borderColor: active ? "var(--mk-accent-500)" : "var(--mk-border)",
                background: "var(--mk-surface)",
              }}
            >
              <Frame width={1100} height={430} scale={0.31}>
                <BuiltSite site={site} layout={opt.id} editing={false} />
              </Frame>
              <div className="border-t border-mk-border p-4">
                <div className="flex items-baseline justify-between gap-2">
                  <h3 className="text-mk-body font-semibold text-mk-ink">{opt.name}</h3>
                  <span className="text-mk-small text-mk-faint">{opt.tag}</span>
                </div>
                <ul className="mt-2 space-y-1">
                  {opt.bullets.map((b) => (
                    <li key={b} className="text-mk-small text-mk-secondary">
                      {b}
                    </li>
                  ))}
                </ul>
              </div>
            </button>
          );
        })}
      </div>

      {picked ? (
        <div className="mt-8 max-w-[720px]">
          <label className="text-mk-label text-mk-secondary" htmlFor="site-why">
            选它的理由
          </label>
          <p className="mt-1 text-mk-small text-mk-muted">
            请写清楚你为什么要这一版、不要另外两版。后面我做出来的东西你觉得不对时，这句话就是你说「不对」的依据。
          </p>
          <textarea
            id="site-why"
            value={why}
            onChange={(e) => setWhy(e.target.value)}
            rows={3}
            className="mt-3 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface p-3 text-mk-body text-mk-ink outline-none focus:border-mk-accent-200"
            placeholder="比如：我做的东西比写的字多，索引式一屏能看到十几条，另外两版一屏只放得下一件。"
          />
          <button
            type="button"
            disabled={busy || !why.trim()}
            onClick={() => onPick(picked, why.trim())}
            className="mt-3 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            确认
          </button>
          {/* 置灰的按钮是一堵沉默的墙，所以旁边写清楚它在等什么。服务端那一道
              也拦着（400），这里只是把话说在前面。 */}
          {!why.trim() ? (
            <span className="ml-3 text-mk-small text-mk-muted">写完理由才能确认。</span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

/* ── 内容 ─────────────────────────────────────────────────────────────── */

function WordsStep({
  draft,
  setDraft,
  site,
  layout,
  narrow,
  onSave,
  busy,
  saved,
}: {
  draft: SiteDraft;
  setDraft: (d: SiteDraft) => void;
  site: SiteContent;
  layout: SiteLayout;
  narrow: boolean;
  onSave: () => void;
  busy: boolean;
  saved: boolean;
}) {
  const set = <K extends keyof SiteDraft>(k: K, v: SiteDraft[K]) => setDraft({ ...draft, [k]: v });
  const setBlurb = (id: string, v: string) =>
    setDraft({ ...draft, blurbs: { ...draft.blurbs, [id]: v } });

  return (
    <div className="mt-8 gap-8 lg:flex">
      <div className="min-w-0 lg:w-[440px] lg:shrink-0">
        <Field
          label="首屏那句话"
          hint="整页围绕它写。一个问题，或者一句你真的想说的话。"
          value={draft.headline}
          onChange={(v) => set("headline", v)}
          rows={2}
          placeholder="一件还能修的东西，是谁决定它该被扔的？"
        />
        <Field
          label="你是谁"
          hint="名字底下那一行。"
          value={draft.role}
          onChange={(v) => set("role", v)}
          rows={1}
          placeholder="读 IB 的高二学生 · 在拆东西"
        />
        <Field
          label="开场一段"
          hint="给第一次点进来的人。两三句。"
          value={draft.lead}
          onChange={(v) => set("lead", v)}
          rows={3}
        />
        <ListField
          label="关于"
          hint="一段一行。这是页面上最像你的地方。"
          value={draft.about}
          onChange={(v) => set("about", v)}
          rows={4}
        />
        <Field
          label="现在"
          hint="一行状态。真实个人站上的 /now。"
          value={draft.now}
          onChange={(v) => set("now", v)}
          rows={1}
        />
        <ListField
          label="现在在做的事"
          hint="一件一行。"
          value={draft.nowList}
          onChange={(v) => set("nowList", v)}
          rows={1}
        />
        <ListField
          label="标签"
          hint="一个一行。"
          value={draft.tags}
          onChange={(v) => set("tags", v)}
          rows={1}
        />
        <ListField
          label="页头几个词"
          hint="一个一行，显示在头图上，只有「一个作品打头」这一版用得到。"
          value={draft.motto}
          onChange={(v) => set("motto", v)}
          rows={1}
        />
        <Field
          label="联系方式"
          hint="可以不填。填了它会公开显示在页面上，谁都看得到。"
          value={draft.contact}
          onChange={(v) => set("contact", v)}
          rows={1}
        />

        {site.posts.length || site.projects.length || site.reads.length ? (
          <div className="mt-8">
            <h3 className="text-mk-label text-mk-secondary">每一条你自己写的那句话</h3>
            <p className="mt-1 text-mk-small text-mk-muted">
              这些是你真写完、真做完的东西。介绍它们的话由你写——空着就在页面上空着。
            </p>
            {[
              ...site.posts.map((p) => ({ id: p.id, title: p.title, kind: "文章" })),
              ...site.projects.map((p) => ({ id: p.id, title: p.title, kind: "作品" })),
              ...site.reads.map((r) => ({ id: r.id, title: r.title, kind: "在读" })),
            ].map((it) => (
              <div key={it.id} className="mt-4">
                <p className="text-mk-small text-mk-ink">
                  <span className="text-mk-faint">{it.kind}</span> {it.title}
                </p>
                <textarea
                  value={draft.blurbs[it.id] ?? ""}
                  onChange={(e) => setBlurb(it.id, e.target.value)}
                  rows={2}
                  className="mt-1.5 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface p-2.5 text-mk-small text-mk-ink outline-none focus:border-mk-accent-200"
                />
              </div>
            ))}
          </div>
        ) : null}

        <div className="sticky bottom-4 mt-8 flex items-center gap-3">
          <button
            type="button"
            onClick={onSave}
            disabled={busy}
            className="rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            保存
          </button>
          {saved ? (
            <span className="flex items-center gap-1 text-mk-small text-mk-muted">
              <Icon icon={Check} size={14} /> 已保存
            </span>
          ) : null}
        </div>
      </div>

      {/* 右边是真的那一页，随她敲字变。 */}
      <div className="mt-10 min-w-0 flex-1 lg:mt-0">
        <div className="sticky top-6">
          <p className="mb-2 text-mk-small text-mk-faint">这是别人会看到的效果</p>
          <PreviewBox narrow={narrow}>
            <BuiltSite site={site} layout={layout} narrow={narrow} editing />
          </PreviewBox>
        </div>
      </div>
    </div>
  );
}

function Field({
  label,
  hint,
  value,
  onChange,
  rows,
  placeholder,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  rows: number;
  placeholder?: string;
}) {
  return (
    <div className="mt-6">
      <label className="text-mk-label text-mk-secondary">{label}</label>
      {hint ? <p className="mt-1 text-mk-small text-mk-muted">{hint}</p> : null}
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={rows}
        placeholder={placeholder}
        className="mt-2 w-full resize-none rounded-mk-lg border border-mk-border bg-mk-surface p-3 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}

/** 一行一条。比「加一条 / 删一条」的按钮组少一层交互，也更好改。 */
function ListField({
  label,
  hint,
  value,
  onChange,
  rows,
}: {
  label: string;
  hint?: string;
  value: string[];
  onChange: (v: string[]) => void;
  rows: number;
}) {
  return (
    <Field
      label={label}
      hint={hint}
      value={value.join("\n")}
      onChange={(v) => onChange(v.split("\n"))}
      rows={Math.max(rows, Math.min(value.length + 1, 8))}
    />
  );
}

/* ── 发布 ─────────────────────────────────────────────────────────────── */

function ShipStep({
  state,
  site,
  narrow,
  busy,
  onPublish,
  onRevoke,
}: {
  state: SiteState;
  site: SiteContent;
  narrow: boolean;
  busy: boolean;
  onPublish: () => void;
  onRevoke: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  useEffect(() => () => {
    if (timer.current) window.clearTimeout(timer.current);
  }, []);

  return (
    <div className="mt-8 gap-8 lg:flex">
      <div className="lg:w-[440px] lg:shrink-0">
        {state.missing.length ? (
          <>
            <h3 className="text-mk-body font-semibold text-mk-ink">这一页还缺你自己写的</h3>
            <ul className="mt-3 space-y-1.5">
              {state.missing.map((m) => (
                <li key={m} className="text-mk-body text-mk-secondary">
                  · {m}
                </li>
              ))}
            </ul>
            <p className="mt-4 text-mk-small text-mk-muted">
              发出去的是你的主页，得先有你说的话。补齐之后回到这里。
            </p>
          </>
        ) : state.published ? (
          <>
            <h3 className="text-mk-body font-semibold text-mk-ink">已经在线上了</h3>
            <p className="mt-2 text-mk-small text-mk-muted">
              这条链接谁拿到谁就能打开，不需要登录。搜索引擎收不到它。你随时可以收回。
            </p>
            <div className="mt-4 flex items-center gap-2 rounded-mk-lg border border-mk-border bg-mk-surface p-3">
              <Icon icon={Link2} size={15} />
              <input
                readOnly
                value={state.url}
                className="min-w-0 flex-1 bg-transparent text-mk-small text-mk-ink outline-none"
              />
              <button
                type="button"
                onClick={() => {
                  void navigator.clipboard?.writeText(state.url);
                  setCopied(true);
                  timer.current = window.setTimeout(() => setCopied(false), 1600);
                }}
                className="shrink-0 text-mk-small"
                style={{ color: "var(--mk-accent-600)" }}
              >
                {copied ? "已复制" : "复制"}
              </button>
            </div>
            <button
              type="button"
              onClick={onRevoke}
              disabled={busy}
              className="mt-4 text-mk-small disabled:opacity-40"
              style={{ color: "var(--mk-danger)" }}
            >
              收回这条链接
            </button>
            <p className="mt-1.5 text-mk-small text-mk-muted">
              收回之后这条链接立刻打不开。你可以再发布一次，链接还是这一条。
            </p>
          </>
        ) : (
          <>
            <h3 className="text-mk-body font-semibold text-mk-ink">准备好了</h3>
            <p className="mt-2 text-mk-small text-mk-muted">
              发布之后你会拿到一条链接。拿到链接的人不用登录就能打开，搜索引擎收不到它，你随时可以收回。
            </p>
            <button
              type="button"
              // 顶上的步骤条里也有一个叫「发布」的按钮（那是切页签）。两个同名
              // 按钮里，只有这一个真的会把页面发出去，所以给它一个稳定的钩子。
              data-testid="site-publish"
              onClick={onPublish}
              disabled={busy}
              className="mt-4 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              发布
            </button>
          </>
        )}
      </div>

      <div className="mt-10 min-w-0 flex-1 lg:mt-0">
        <PreviewBox narrow={narrow}>
          <BuiltSite site={site} layout={state.layout} narrow={narrow} editing={!state.published} />
        </PreviewBox>
      </div>
    </div>
  );
}

/* ── 预览容器 ─────────────────────────────────────────────────────────── */

/** 手机预览是一个真的 390px 宽的框，桌面预览占满剩下的宽度。
 *  `narrow` 走 prop 而不是媒体查询——见 BuiltSite 的注释。 */
function PreviewBox({ narrow, children }: { narrow: boolean; children: React.ReactNode }) {
  return (
    <div className="flex justify-center">
      <div
        className="overflow-hidden rounded-mk-lg border border-mk-border"
        style={{ width: narrow ? 390 : "100%", height: 720 }}
      >
        <div className="h-full overflow-y-auto">{children}</div>
      </div>
    </div>
  );
}

/** 把一个真实宽度的页面等比缩进卡片里。缩放，不简化。 */
function Frame({
  width,
  height,
  scale,
  children,
}: {
  width: number;
  height: number;
  scale: number;
  children: React.ReactNode;
}) {
  return (
    <div style={{ height, overflow: "hidden", position: "relative" }}>
      <div
        style={{
          width,
          height: height / scale,
          transform: `scale(${scale})`,
          transformOrigin: "top left",
          pointerEvents: "none",
        }}
      >
        {children}
      </div>
    </div>
  );
}
