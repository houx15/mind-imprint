import { useCallback, useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { LandingHeader } from "../learning/LandingHeader";
import { Pagination } from "../learning/Pagination";
import { navigate, writingPath } from "../routing";
import {
  listWritingPrompts,
  startWritingFromPrompt,
  type PromptQuery,
  type WritingPromptPage,
} from "../api/writingPrompts";
import { apiErrorText } from "../api/errorText";
import { PromptCard } from "./PromptCard";
import { WritingsTabs } from "./WritingsTabs";

/**
 * WritingPromptLibraryPage —— 写作题库（`/writings/library`）。
 *
 * 705 道 2024–2026 的真题与官方题：中考 / 高考的语文与英语、托福、雅思、GRE。
 *
 * # 🚨 筛、搜、翻页都在服务端
 *
 * 阅读那边（48 篇）是前端筛的，而且它的注释写着「等库长到几百篇，这里会换成
 * 服务端筛」。这里就是那个几百篇：整库发过来 900KB，而且「筛完之后每一维还剩
 * 哪些值、各有多少道」只有看得见全库的人算得出来。
 *
 * 所以每一次改筛选、翻页、敲搜索，都是一次请求。两件事因此必须做对：
 *
 *   - **搜索要防抖**，否则她打「短视频」三个字就是三次请求，而且回来的顺序
 *     不保证 —— 第二个字的结果可能盖掉第三个字的。
 *   - **回来的结果要认得出是哪一次请求的**。用一个自增的号码牌：迟到的那一份
 *     直接丢掉。不这么做的话，快的那次请求会让屏幕停在一个她已经改过的筛选上。
 *
 * # 改筛选要回到第 1 页
 *
 * 她在第 12 页选了「中考语文」，结果只有 3 页 —— 不回第 1 页的话她看到的是
 * 一片空白，读起来像「这个筛选没有题」。
 */

const PAGE_SIZE = 24;

export function WritingPromptLibraryPage() {
  const [data, setData] = useState<WritingPromptPage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [q, setQ] = useState("");
  const [lang, setLang] = useState("");
  const [category, setCategory] = useState("");
  const [difficulty, setDifficulty] = useState(0);
  const [topic, setTopic] = useState("");
  const [page, setPage] = useState(1);

  // 搜索框里的字和真正发出去的那一个是两回事：中间隔一个 250ms 的防抖。
  const [typed, setTyped] = useState("");
  useEffect(() => {
    const t = setTimeout(() => {
      setQ(typed);
      setPage(1);
    }, 250);
    return () => clearTimeout(t);
  }, [typed]);

  // 🚨 号码牌：只认最后一次请求的结果。
  const ticket = useRef(0);
  const load = useCallback((query: PromptQuery) => {
    const mine = ++ticket.current;
    setError(null);
    listWritingPrompts({ ...query, pageSize: PAGE_SIZE })
      .then((res) => {
        if (mine !== ticket.current) return; // 迟到的那一份，丢掉
        setData(res);
      })
      .catch((err) => {
        if (mine !== ticket.current) return;
        setError(apiErrorText(err));
      });
  }, []);

  useEffect(() => {
    load({ q, lang, category, difficulty, topic, page });
  }, [load, q, lang, category, difficulty, topic, page]);

  // 换任何一个筛选都回到第 1 页。
  function pick<T>(set: (v: T) => void, v: T) {
    set(v);
    setPage(1);
  }

  async function start(id: string) {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      navigate(writingPath(await startWritingFromPrompt(id)));
    } catch (err) {
      setError(apiErrorText(err));
      setBusy(false);
    }
  }

  const filtered = Boolean(q.trim() || lang || category || difficulty || topic);

  return (
    <div className="learning-landing learning-library min-h-full">
      <div className="learning-landing-measure learning-landing-wide mx-auto w-full">
        <div className="flex justify-center">
          <WritingsTabs active="library" libraryCount={data?.total} />
        </div>

        <LandingHeader
          kind="writing"
          title="写作题库"
          description="2024–2026 年的中考、高考、托福、雅思与 GRE 作文题，可按语言、考试、难度和话题筛选。"
        />

        <div className="reading-library-filters flex flex-col gap-3">
          <label className="relative block">
            <span className="sr-only">搜索题目</span>
            <Search
              size={16}
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-mk-muted"
            />
            <input
              type="search"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder="搜索题面、出处或话题"
              className="w-full rounded-mk-full border border-mk-input-border bg-mk-surface py-2 pl-9 pr-4 text-mk-small text-mk-ink outline-none transition-colors duration-[120ms] ease-mk placeholder:text-mk-muted focus:border-mk-accent-300"
            />
          </label>

          <FilterRow label="语言">
            <Chip label="全部" active={lang === ""} onPick={() => pick(setLang, "")} />
            {(data?.facets.langs ?? []).map((f) => (
              <Chip
                key={f.value}
                label={`${f.label} ${f.count}`}
                active={lang === f.value}
                onPick={() => pick(setLang, lang === f.value ? "" : f.value)}
              />
            ))}
          </FilterRow>

          <FilterRow label="考试">
            <Chip label="全部" active={category === ""} onPick={() => pick(setCategory, "")} />
            {(data?.facets.categories ?? []).map((f) => (
              <Chip
                key={f.value}
                label={`${f.label} ${f.count}`}
                active={category === f.value}
                onPick={() => pick(setCategory, category === f.value ? "" : f.value)}
              />
            ))}
          </FilterRow>

          <FilterRow label="难度">
            <Chip label="全部" active={difficulty === 0} onPick={() => pick(setDifficulty, 0)} />
            {(data?.facets.difficulties ?? []).map((f) => (
              <Chip
                key={f.value}
                label={`${f.label} ${f.count}`}
                active={difficulty === Number(f.value)}
                onPick={() =>
                  pick(setDifficulty, difficulty === Number(f.value) ? 0 : Number(f.value))
                }
              />
            ))}
          </FilterRow>

          <FilterRow label="话题">
            <Chip label="全部" active={topic === ""} onPick={() => pick(setTopic, "")} />
            {(data?.facets.topics ?? []).map((f) => (
              <Chip
                key={f.value}
                label={`${f.label} ${f.count}`}
                active={topic === f.value}
                onPick={() => pick(setTopic, topic === f.value ? "" : f.value)}
              />
            ))}
          </FilterRow>
        </div>

        {error && (
          <p role="alert" className="mt-4 text-mk-small text-mk-danger">
            {error}
          </p>
        )}

        {data === null && !error && (
          <p className="mt-10 text-center text-mk-small text-mk-muted">处理中</p>
        )}

        {data !== null && (
          <>
            {/* 推荐只在没筛没搜的第一页出现（服务端也是这么判的）。 */}
            {data.recommended.length > 0 && !filtered && page === 1 && (
              <section className="mt-6" aria-label="推荐题目">
                <p className="text-mk-caption text-mk-muted">先试试这几道</p>
                <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
                  {data.recommended.map((r) => (
                    <PromptCard
                      key={`rec:${r.prompt.id}`}
                      recommended
                      prompt={r.prompt}
                      busy={busy}
                      onStart={(id) => void start(id)}
                    />
                  ))}
                </div>
              </section>
            )}

            <p className="mt-6 text-mk-label text-mk-muted">
              {data.total} 道{filtered ? "（已筛选）" : ""}
              {data.pages > 1 ? ` · 第 ${data.page} / ${data.pages} 页` : ""}
            </p>

            {data.items.length === 0 ? (
              <p className="mt-10 text-center text-mk-small text-mk-secondary">
                没有匹配的题目。换一个词，或者清掉筛选。
              </p>
            ) : (
              <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {data.items.map((p) => (
                  <PromptCard
                    key={p.id}
                    prompt={p}
                    busy={busy}
                    onStart={(id) => void start(id)}
                  />
                ))}
              </div>
            )}

            <Pagination
              page={data.page}
              pages={data.pages}
              onPick={(n) => {
                setPage(n);
                // 翻页之后回到顶部：不这么做的话她按了「下一页」，
                // 屏幕还停在上一页的底部，看起来像什么都没发生。
                window.scrollTo({ top: 0, behavior: "smooth" });
              }}
            />
          </>
        )}
      </div>
    </div>
  );
}

function FilterRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="reading-filter-label">{label}</span>
      {children}
    </div>
  );
}

function Chip({
  label,
  active,
  onPick,
}: {
  label: string;
  active: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onPick}
      aria-pressed={active}
      className="rounded-mk-full border px-3 py-1 text-mk-label transition-colors duration-[120ms] ease-mk"
      style={
        active
          ? {
              borderColor: "var(--mk-accent-500)",
              background: "color-mix(in srgb, var(--mk-accent-500) 16%, transparent)",
              color: "color-mix(in srgb, var(--mk-accent-500) 74%, black)",
            }
          : { borderColor: "var(--mk-border)", color: "var(--mk-secondary)" }
      }
    >
      {label}
    </button>
  );
}
