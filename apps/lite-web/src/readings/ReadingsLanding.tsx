import { useEffect, useState } from "react";
import { BookOpen } from "lucide-react";
import { Button, Card, CompactRow, Icon, Input, Textarea } from "@/ui";
import { ApiError } from "../api/client";
import { createReading, listReadings, putReadingSource, type Reading } from "../api/readings";
import { navigate, readingPath } from "../routing";

/**
 * ReadingsLanding — the 阅读 tab's landing page: a "开始阅读" composer up top
 * (paste a title + the article body, straight into a fresh reading) and
 * 过往的阅读 below it (history lives on the tab's landing page, not in the
 * sidebar — see AGENTS.md's shell rationale). Selecting a past reading, or
 * finishing the composer, both `navigate` into `/readings/:id`; LiteApp then
 * mounts `ReadingRoomHost` (Task 12) for that id.
 *
 * NOTE for Task 14 (verbatim, load-bearing e2e selectors):
 *   - title input placeholder: 「给这次阅读起个名字（可留空）」
 *   - body textarea placeholder: 「把文章正文粘贴到这里…」
 *   - submit button label: 「开始阅读」
 */
export function ReadingsLanding() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);

  const [history, setHistory] = useState<Reading[] | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listReadings()
      .then((rows) => { if (!cancelled) setHistory(rows); })
      .catch(() => { if (!cancelled) setHistoryError("过往的阅读暂时加载不出来，刷新一下再试试。"); });
    return () => { cancelled = true; };
  }, []);

  async function handleStart() {
    const text = body.trim();
    if (!text || starting) return;
    setStarting(true);
    setStartError(null);
    try {
      const { id } = await createReading({ title: title.trim() });
      await putReadingSource(id, { title: title.trim(), text });
      navigate(readingPath(id));
    } catch (err) {
      setStartError(err instanceof ApiError ? err.message : "开始阅读失败，请重试。");
      setStarting(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-8 p-8">
      <div className="flex flex-col gap-1">
        <h1 className="text-mk-display text-mk-ink">阅读</h1>
        <p className="text-mk-body text-mk-muted">把一篇文章带进来，AI 陪你一段一段读透它。</p>
      </div>

      <Card className="flex flex-col gap-4 p-6">
        <h2 className="text-mk-h2 text-mk-ink">开始一次新的阅读</h2>
        <Input
          value={title}
          onChange={setTitle}
          placeholder="给这次阅读起个名字（可留空）"
          disabled={starting}
        />
        <Textarea
          value={body}
          onChange={setBody}
          placeholder="把文章正文粘贴到这里…"
          disabled={starting}
          className="min-h-[220px]"
        />
        {startError && <p className="text-mk-small text-mk-danger">{startError}</p>}
        <div className="flex justify-end">
          <Button onClick={handleStart} disabled={!body.trim()} loading={starting}>
            开始阅读
          </Button>
        </div>
      </Card>

      <div className="flex flex-col gap-3">
        <h2 className="text-mk-h2 text-mk-ink">过往的阅读</h2>
        {historyError && <p className="text-mk-small text-mk-danger">{historyError}</p>}
        {!historyError && history === null && (
          <p className="text-mk-body text-mk-muted">加载中…</p>
        )}
        {!historyError && history !== null && history.length === 0 && (
          <p className="text-mk-body text-mk-muted">还没有开始过阅读，从上面开始第一次吧。</p>
        )}
        {history !== null && history.length > 0 && (
          <div className="flex flex-col gap-2">
            {history.map((r) => (
              <CompactRow
                key={r.id}
                thumb={<Icon icon={BookOpen} size={18} className="text-mk-accent-600" />}
                title={r.title}
                meta={statusLabel(r)}
                onClick={() => navigate(readingPath(r.id))}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function statusLabel(r: Reading): string {
  if (r.finishedAt) return "已完成";
  if (r.hasSource) return "阅读中";
  return "尚未粘贴正文";
}
