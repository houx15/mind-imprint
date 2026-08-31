import { Check, Eye, Globe } from "lucide-react";
import { useEco } from "../store";
import { PAGE_STYLES, styleById } from "../data/homepage";
import { READINGS, WRITINGS } from "../data/library";
import { go } from "../route";
import type { HomepageSection } from "../data/types";
import { Btn, Field, Panel, SectionHead, Sys, cx } from "../ui";

/**
 * 我的主页 · the compose surface.
 *
 * ## What this is NOT any more (2026-08-31)
 * It used to be a six-step wizard that doubled as the product's first PBL and
 * as a gate on every other track. All three jobs are gone: the *learning*
 * (look at examples, name a style, write an exact command) moved into the card
 * library where every track can use it, and the *gate* was deleted outright.
 *
 * What is left is the mechanical half, which genuinely is mechanical: choose
 * which of your things go on the page, choose a look, publish. No lesson, no
 * ceremony, no steps. If she wants the lesson, she starts a 网站 project and
 * 印记 walks her through 参考搜集 → 命名风格 → 明确指令 → 三个版本.
 *
 * ## She curates
 * The page shows only what she picked. Publishing a project auto-picks it (see
 * `store.publishProject`) because the publish copy promises exactly that, but
 * she can unpick anything here.
 */
export function PageStudio() {
  const { state, hpWrite, hpPick, hpSetSections, hpSetStyle, hpPublish } = useEco();
  const hp = state.homepage;
  const style = hp.style ? styleById(hp.style) : null;
  const written = hp.sections.filter((s) => s.enabled && (s.value.trim() || s.picked?.length)).length;

  function toggle(id: string) {
    hpSetSections(hp.sections.map((s) => (s.id === id ? { ...s, enabled: !s.enabled } : s)));
  }

  return (
    <div className="mx-auto max-w-[1000px] px-8 py-8">
      <SectionHead
        index="我的主页 · MY PAGE"
        title="这一页是你对外的门牌"
        sub="挑要放什么、选个样子、发布。想学「怎么把想法变成 AI 能执行的指令」，去开一个「一个网站」的项目。"
        right={
          <div className="text-right">
            <Sys>已填</Sys>
            <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
              {written}/{hp.sections.filter((s) => s.enabled).length}
            </span>
          </div>
        }
      />

      {/* ── ① sections ─────────────────────────────────────────────────── */}
      <Sys className="mb-2.5 block">① 这一页放什么</Sys>
      <div className="mb-3 flex flex-wrap gap-1.5">
        {hp.sections.map((s) => (
          <button
            key={s.id}
            type="button"
            onClick={() => toggle(s.id)}
            className={cx(
              "inline-flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-small",
              "transition-colors duration-[120ms] focus-visible:outline-none focus-visible:ring-2",
              "focus-visible:ring-mk-accent-200",
              s.enabled
                ? "border-mk-accent bg-mk-accent-50 text-mk-ink"
                : "border-mk-border bg-mk-surface text-mk-muted",
            )}
          >
            {s.enabled ? <Check size={12} strokeWidth={3} className="text-mk-accent-700" /> : null}
            {s.label}
          </button>
        ))}
      </div>

      <div className="space-y-4">
        {hp.sections
          .filter((s) => s.enabled)
          .map((s) => (
            <SectionEditor
              key={s.id}
              section={s}
              onWrite={(v) => hpWrite(s.id, v)}
              onPick={(itemId) => hpPick(s.id, itemId)}
              projects={state.projects}
            />
          ))}
      </div>

      {/* ── ② style ───────────────────────────────────────────────────── */}
      <Sys className="mb-2.5 mt-9 block">② 长什么样</Sys>
      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {PAGE_STYLES.map((st) => {
          const on = hp.style === st.id;
          return (
            <li key={st.id}>
              <button
                type="button"
                onClick={() => hpSetStyle(st.id)}
                className={cx(
                  "h-full w-full overflow-hidden rounded-mk-lg border text-left transition-all duration-[140ms]",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                  on ? "border-mk-accent shadow-mk-md" : "border-mk-border hover:border-mk-accent-200",
                )}
              >
                {/* A real miniature of the style, not a swatch: the whole
                    decision is "what does the page feel like". */}
                <span
                  className="block px-4 py-5"
                  style={{ background: st.paper, color: st.ink, fontFamily: st.font }}
                >
                  <span
                    className="block"
                    style={{
                      fontSize: st.headline === "sans-tight" ? 20 : 18,
                      fontWeight: st.headline === "sans-tight" ? 800 : 600,
                      letterSpacing: st.headline === "mono-caps" ? "0.16em" : undefined,
                      textTransform: st.headline === "mono-caps" ? "uppercase" : undefined,
                      fontStyle: st.headline === "serif-italic" ? "italic" : undefined,
                    }}
                  >
                    林知遥
                  </span>
                  <span
                    className="mt-1.5 block h-px w-10"
                    style={{ background: st.accent }}
                  />
                  <span className="mt-2 block text-[11px] leading-[1.7] opacity-70">
                    我在想：普通人的日常，谁在记录。
                  </span>
                </span>
                <span className="block bg-mk-surface px-4 py-3">
                  <span className="block text-mk-body font-semibold text-mk-ink">{st.label}</span>
                  <span className="mt-0.5 block text-mk-small leading-[1.7] text-mk-muted">
                    {st.blurb}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>

      {/* ── ③ publish ─────────────────────────────────────────────────── */}
      <Panel className="mt-9 p-6">
        <Sys>③ 发布</Sys>
        <h3 className="mt-1 text-mk-h1 text-mk-ink">
          {hp.published ? "这一页已经在线上了" : "把它发出去"}
        </h3>
        <p className="mt-2 max-w-[60ch] text-mk-body leading-[1.9] text-mk-secondary">
          {hp.published
            ? "改完以后自动生效，链接不变。"
            : "半成品也可以发布。发出去以后你随时能改，链接不会变。"}
        </p>
        <p className="mt-2 font-mono text-mk-small text-mk-faint">
          /eco/p/zhiyao
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          {!hp.published ? (
            <Btn
              iconStart={<Globe size={16} strokeWidth={1.9} />}
              disabled={!style || written === 0}
              onClick={hpPublish}
            >
              发布
            </Btn>
          ) : null}
          <Btn
            variant={hp.published ? "primary" : "outline"}
            iconStart={<Eye size={16} strokeWidth={1.9} />}
            onClick={() => go({ name: "page", handle: "zhiyao" })}
          >
            {hp.published ? "去看这一页" : "先预览"}
          </Btn>
        </div>
        {!style ? (
          <p className="mt-2 text-mk-small text-mk-muted">先在上面挑一个样子。</p>
        ) : written === 0 ? (
          <p className="mt-2 text-mk-small text-mk-muted">
            至少写一块内容，或者挑一件你做过的东西。
          </p>
        ) : null}
      </Panel>
    </div>
  );
}

function SectionEditor({
  section,
  onWrite,
  onPick,
  projects,
}: {
  section: HomepageSection;
  onWrite: (v: string) => void;
  onPick: (itemId: string) => void;
  projects: { id: string; title: string; status: string }[];
}) {
  if (!section.picker) {
    return (
      <Panel className="p-5">
        <Field
          label={section.label}
          hint={section.hint}
          value={section.value}
          onChange={onWrite}
          rows={3}
        />
      </Panel>
    );
  }

  const items =
    section.picker === "readings"
      ? READINGS.map((r) => ({ id: r.id, title: r.title, sub: r.source }))
      : section.picker === "writings"
        ? WRITINGS.map((w) => ({ id: w.id, title: w.title, sub: `${w.words} 字` }))
        : projects.map((p) => ({
            id: p.id,
            title: p.title,
            sub: p.status === "published" ? "已发布" : "在做",
          }));
  const picked = section.picked ?? [];

  return (
    <Panel className="p-5">
      <p className="text-mk-h3 text-mk-ink">{section.label}</p>
      <p className="mt-1 text-mk-small text-mk-muted">{section.hint}</p>
      {items.length === 0 ? (
        <p className="mt-3 text-mk-small text-mk-faint">还没有可以挑的。</p>
      ) : (
        <ul className="mt-3 flex flex-wrap gap-1.5">
          {items.map((it) => {
            const on = picked.includes(it.id);
            return (
              <li key={it.id}>
                <button
                  type="button"
                  onClick={() => onPick(it.id)}
                  className={cx(
                    "inline-flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-small",
                    "transition-colors duration-[120ms] focus-visible:outline-none focus-visible:ring-2",
                    "focus-visible:ring-mk-accent-200",
                    on
                      ? "border-mk-accent bg-mk-accent-50 text-mk-ink"
                      : "border-mk-border bg-mk-surface text-mk-secondary hover:border-mk-accent-200",
                  )}
                >
                  {on ? <Check size={12} strokeWidth={3} className="text-mk-accent-700" /> : null}
                  {it.title}
                  <span className="text-mk-faint">{it.sub}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}
