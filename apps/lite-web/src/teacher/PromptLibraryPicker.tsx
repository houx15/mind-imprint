import { useCallback, useEffect, useRef, useState } from "react";
import { Library, Search } from "lucide-react";
import { Button, Icon, Modal } from "@/ui";
import { Pagination } from "../learning/Pagination";
import {
  listWritingPrompts,
  type PromptQuery,
  type WritingPrompt,
  type WritingPromptPage,
} from "../api/writingPrompts";
import { apiErrorText } from "../api/errorText";
import { Chip, LABEL_CLS } from "./formParts";

/**
 * PromptLibraryPicker —— 老师从写作题库里挑一道题，填进「题目」那一栏。
 *
 * # 它站在哪
 *
 * 老师布置写作作业时，「题目」本来只有两条路：自己打，或者
 * `WritingExtract` 从她粘进来的文件里抽。这是第三条：从 705 道真题里挑。
 * 三条路的终点是同一个 —— 都只是**把字填进那个 textarea**，填完她照样能改。
 *
 * 🚨 填完不锁。老师常常要在真题基础上改一句（换个字数、去掉「不少于800字」
 * 那半句）。挑题是省打字，不是替她做决定。
 *
 * # 和学生那一页共用一条接口
 *
 * 题库是内容，两边看到的是同一份（`GET /api/v1/writing-prompts`）。
 * 这里只是换了一身衣服：弹窗里、卡片更紧凑、挑中就关。
 */

const PAGE_SIZE = 8;

export function PromptLibraryPicker({
  onFill,
}: {
  onFill: (r: {
    prompt: string;
    targetWords: number | null;
    lang: "zh" | "en";
  }) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <div className="flex items-center gap-2">
        <span className={LABEL_CLS}>从题库选</span>
        <Button variant="secondary" size="sm" onClick={() => setOpen(true)}>
          <Icon icon={Library} size={14} />
          打开写作题库
        </Button>
      </div>
      {open && (
        <PickerModal
          onClose={() => setOpen(false)}
          onPick={(p) => {
            onFill({
              prompt: promptBody(p),
              targetWords: wordsOf(p.wordLimit),
              lang: p.lang,
            });
            setOpen(false);
          }}
        />
      )}
    </>
  );
}

/** 填进「题目」那一栏的字：题面本身，出处放在末尾一行。 */
export function promptBody(p: WritingPrompt): string {
  const tail = [p.source, p.wordLimit].filter(Boolean).join(" · ");
  return tail ? `${p.text}\n\n（${tail}）` : p.text;
}

/**
 * 从「不少于800字」「at least 250 words」「120-150词」里取一个数。
 *
 * 🚨 取不到就返回 null，**不要猜一个**。目标字数是会写进作业、学生那边
 * 照着它算进度的数；猜错了她会被一个不存在的要求追着跑。
 * 区间取**上限**：「120-150词」老师要的是 150 那一档。
 */
export function wordsOf(limit?: string): number | null {
  if (!limit) return null;
  const nums = limit.match(/\d+/g);
  if (!nums || nums.length === 0) return null;
  const vals = nums.map(Number).filter((n) => n >= 20 && n <= 100000);
  if (vals.length === 0) return null;
  return Math.max(...vals);
}

function PickerModal({
  onClose,
  onPick,
}: {
  onClose: () => void;
  onPick: (p: WritingPrompt) => void;
}) {
  const [data, setData] = useState<WritingPromptPage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [typed, setTyped] = useState("");
  const [q, setQ] = useState("");
  const [lang, setLang] = useState("");
  const [category, setCategory] = useState("");
  const [page, setPage] = useState(1);

  useEffect(() => {
    const t = setTimeout(() => {
      setQ(typed);
      setPage(1);
    }, 250);
    return () => clearTimeout(t);
  }, [typed]);

  // 号码牌：只认最后一次请求的结果，迟到的那一份丢掉。
  const ticket = useRef(0);
  const load = useCallback((query: PromptQuery) => {
    const mine = ++ticket.current;
    setError(null);
    listWritingPrompts({ ...query, pageSize: PAGE_SIZE })
      .then((res) => {
        if (mine === ticket.current) setData(res);
      })
      .catch((err) => {
        if (mine === ticket.current) setError(apiErrorText(err));
      });
  }, []);

  useEffect(() => {
    load({ q, lang, category, page });
  }, [load, q, lang, category, page]);

  return (
    <Modal open onClose={onClose} title="写作题库">
      <div className="flex flex-col gap-3">
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
            className="tc-input w-full pl-9"
          />
        </label>

        <div className="flex flex-wrap items-center gap-1.5">
          <Chip
            active={lang === ""}
            onClick={() => {
              setLang("");
              setPage(1);
            }}
          >
            全部语言
          </Chip>
          {(data?.facets.langs ?? []).map((f) => (
            <Chip
              key={f.value}
              active={lang === f.value}
              onClick={() => {
                setLang(lang === f.value ? "" : f.value);
                setPage(1);
              }}
            >
              {f.label} {f.count}
            </Chip>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <Chip
            active={category === ""}
            onClick={() => {
              setCategory("");
              setPage(1);
            }}
          >
            全部考试
          </Chip>
          {(data?.facets.categories ?? []).map((f) => (
            <Chip
              key={f.value}
              active={category === f.value}
              onClick={() => {
                setCategory(category === f.value ? "" : f.value);
                setPage(1);
              }}
            >
              {f.label} {f.count}
            </Chip>
          ))}
        </div>

        {error && (
          <p role="alert" className="text-mk-small text-mk-danger">
            {error}
          </p>
        )}
        {data === null && !error && (
          <p className="py-6 text-center text-mk-small text-mk-muted">处理中</p>
        )}

        {data !== null && (
          <>
            <p className="text-mk-label text-mk-muted">
              {data.total} 道
              {data.pages > 1 ? ` · 第 ${data.page} / ${data.pages} 页` : ""}
            </p>
            {data.items.length === 0 ? (
              <p className="py-6 text-center text-mk-small text-mk-secondary">
                没有匹配的题目。
              </p>
            ) : (
              <ul className="flex flex-col gap-2">
                {data.items.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => onPick(p)}
                      className="w-full rounded-mk-md border border-mk-border bg-mk-paper p-3 text-left transition-colors duration-[120ms] ease-mk hover:border-mk-accent-300"
                    >
                      <span className="flex flex-wrap items-center gap-1.5 text-mk-label text-mk-muted">
                        <span>{p.category}</span>
                        <span>·</span>
                        <span>{p.year}</span>
                        <span>·</span>
                        <span>{p.taskType}</span>
                        {p.wordLimit && (
                          <>
                            <span>·</span>
                            <span>{p.wordLimit}</span>
                          </>
                        )}
                      </span>
                      <span className="mt-1 block text-mk-small text-mk-ink">
                        {p.text.length > 120
                          ? p.text.slice(0, 120) + "…"
                          : p.text}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <Pagination page={data.page} pages={data.pages} onPick={setPage} />
          </>
        )}
      </div>
    </Modal>
  );
}
