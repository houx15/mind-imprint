import type { AwakeningReportRow } from "../../api/awakening";
import { HISTORY } from "../content";
import { historyRows } from "../library";
import { Choice, Ghost } from "../ui";

/**
 * 兴趣印记的历史。她每一次总结留下的那一份都在这里。
 *
 * # 它守的是哪条反馈
 *
 * 2026-09-21：「查看兴趣印记点进去后，只能看到上一次兴趣测试的印记，
 * 无法回顾之前的。」
 *
 * 树上那条入口原来只带着 `latestReportRunId` 一个 id —— 之前那几份**没有任何
 * 一条路通向它们**。线索库之后这件事更明显：她可以有好几条线索，每条还可能
 * 总结不止一次。
 *
 * # 一份就直接打开那一份
 *
 * 只有一份的时候不摆这张表 —— 一张只有一行的列表是白让她多点一下
 * （判断在 AwakeningRoom 里，这一屏只负责有好几份时怎么摆）。
 */

export function HistoryScene({
  reports,
  onOpen,
  onBack,
}: {
  reports: AwakeningReportRow[];
  onOpen: (report: AwakeningReportRow) => void;
  onBack: () => void;
}) {
  const rows = historyRows(reports, new Date());

  return (
    <section className="awk-screen" aria-label="兴趣印记">
      <div className="awk-wrap awk-hub">
        <div>
          <div className="awk-eyebrow">{HISTORY.eyebrow}</div>
          <h2 className="awk-h2">{HISTORY.title}</h2>
          <p className="awk-p" style={{ marginTop: 10 }}>
            {reports.length === 0 ? HISTORY.empty : HISTORY.lead}
          </p>
          <div className="mt-7">
            <Ghost onClick={onBack}>{HISTORY.back}</Ghost>
          </div>
        </div>

        <div className="awk-choice-stack" aria-label="你拿到过的印记">
          {rows.map((row, i) => (
            <Choice
              key={row.id}
              index={String(i + 1).padStart(2, "0")}
              hwId="IMPRINT"
              title={row.name}
              body={row.meta}
              onClick={() => onOpen(reports[i]!)}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
