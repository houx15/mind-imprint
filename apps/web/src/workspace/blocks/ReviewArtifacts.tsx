import { useEffect, useState } from "react";
import type { Proposal, LogEntry } from "@mind-imprint/contracts";
import { getLog, getDraft } from "../api/workspace";
import { MarkdownPreview } from "./MarkdownPreview";

// ReviewArtifacts — §7 · the review room's LEFT panel: four read-only tabs of the
// journey's artifacts — 活动日志 / 研究框架 / 提案 / 成品 (the finished paper) — so the
// student reflects against what they actually produced. Everything here is
// view-only; the student's reflection form sits to the right.

const FRAMEWORK_DIMS: { key: keyof Proposal; label: string }[] = [
  { key: "objective", label: "目标 · 研究问题" },
  { key: "reason", label: "缘由" },
  { key: "activities", label: "活动与时间" },
  { key: "resources", label: "资源" },
  { key: "counterpoints", label: "可能的反例 / 张力" },
];

type TabKey = "log" | "framework" | "proposal" | "essay";

export function ReviewArtifacts({ projectId, proposal }: { projectId: string; proposal: Proposal }) {
  const [tab, setTab] = useState<TabKey>("log");
  const [log, setLog] = useState<LogEntry[] | null>(null);
  const [proposalDoc, setProposalDoc] = useState<string | null>(null);
  const [essayDoc, setEssayDoc] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void getLog(projectId).then((l) => { if (!cancelled) setLog(l); }).catch(() => { if (!cancelled) setLog([]); });
    void getDraft(projectId, "proposal").then((d) => { if (!cancelled) setProposalDoc(d); }).catch(() => { if (!cancelled) setProposalDoc(""); });
    void getDraft(projectId, "essay").then((d) => { if (!cancelled) setEssayDoc(d); }).catch(() => { if (!cancelled) setEssayDoc(""); });
    return () => { cancelled = true; };
  }, [projectId]);

  const tabs: { key: TabKey; label: string }[] = [
    { key: "log", label: "活动日志" },
    { key: "framework", label: "研究框架" },
    { key: "proposal", label: "提案" },
    { key: "essay", label: "成品" },
  ];

  return (
    <div className="flex h-full min-h-0 flex-col border-r border-mk-border bg-mk-surface">
      <div className="flex flex-none items-center gap-1 border-b border-mk-border px-3 pt-2">
        {tabs.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            className={`-mb-px border-b-2 px-3 py-1.5 text-[13px] font-bold ${tab === t.key ? "border-mk-accent text-mk-accent" : "border-transparent text-mk-muted hover:text-mk-ink"}`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-5 py-4">
        {tab === "log" && (
          log == null ? <Loading /> : log.length === 0 ? <Empty text="还没有活动记录。" /> : (
            <ul className="flex flex-col gap-2">
              {log.map((e) => (
                <li key={e.id} className="flex gap-3 text-[13px] leading-relaxed">
                  <span className="flex-none font-mono text-[12px] text-mk-faint">{e.date}</span>
                  <span className="text-mk-ink">{e.text}</span>
                </li>
              ))}
            </ul>
          )
        )}

        {tab === "framework" && (
          <div className="flex flex-col gap-4">
            {FRAMEWORK_DIMS.map((d) => {
              const value = (proposal[d.key] ?? "").toString().trim();
              return (
                <div key={d.key}>
                  <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">{d.label}</p>
                  {value ? (
                    <p className="mt-1 whitespace-pre-line text-[14px] leading-relaxed text-mk-ink">{value}</p>
                  ) : (
                    <p className="mt-1 text-[14px] text-mk-faint">还没写</p>
                  )}
                </div>
              );
            })}
          </div>
        )}

        {tab === "proposal" && (
          proposalDoc == null ? <Loading /> : proposalDoc.trim() === "" ? <Empty text="还没有提案正文。" /> : <MarkdownPreview text={proposalDoc} />
        )}

        {tab === "essay" && (
          essayDoc == null ? <Loading /> : essayDoc.trim() === "" ? <Empty text="还没有成品论文。" /> : <MarkdownPreview text={essayDoc} />
        )}
      </div>
    </div>
  );
}

function Loading() {
  return <p className="text-[14px] text-mk-faint">加载中…</p>;
}
function Empty({ text }: { text: string }) {
  return <p className="text-[14px] text-mk-faint">{text}</p>;
}
