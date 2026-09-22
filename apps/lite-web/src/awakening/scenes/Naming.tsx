import { useEffect, useState } from "react";

import { suggestThreadTitles } from "../../api/awakening";
import { NAMING } from "../content";
import { Choice, Ghost, Primary } from "../ui";

/**
 * 给一条线索起名字。她按下「暂时保留兴趣线索」之后走这一步。
 *
 * # 为什么在这里问
 *
 * 名字是给**线索库**用的，而她第一次需要认出一条线索，正是在她把它放下、
 * 过几天回来的时候。放下的那一刻问，她脑子里还记得这条线索是关于什么的。
 *
 * # 模型给候选，她挑一个
 *
 * 产品负责人 2026-09-21：「model suggest and user selects」。不是模型直接定
 * —— 一个概括出来的名字可能根本不是她心里那件事，而这条线索是她的。
 *
 * # 🚨 最后一个候选永远是她自己的原话
 *
 * 服务端保证这件事（awakening/title.go 的 TitleChoices）。模型那一次没回上来
 * 时它是唯一的一个，界面照实说一句「生成失败」，**绝不把她的原话冒充成模型的
 * 建议**（memory: ai-errors-must-surface-never-fake）。
 */

export function NamingScene({
  runId,
  /** 她自己写下的第一句。服务端那份候选拿不到时，这里至少还有东西可显示。 */
  onDone,
}: {
  runId: string;
  /** 挑定了（或者跳过）。`title` 为空串表示不起名。 */
  onDone: (title: string) => void;
}) {
  const [titles, setTitles] = useState<string[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [picked, setPicked] = useState("");

  useEffect(() => {
    let alive = true;
    suggestThreadTitles(runId)
      .then((r) => {
        if (!alive) return;
        setTitles(r.titles);
        setFailed(r.failed);
      })
      .catch(() => {
        if (!alive) return;
        // 请求本身没打通也是一次失败 —— 照实说，这一步可以跳过。
        setTitles([]);
        setFailed(true);
      });
    return () => {
      alive = false;
    };
  }, [runId]);

  if (titles === null) {
    return (
      <section className="awk-screen" aria-label="正在拟名字">
        <div className="awk-wrap awk-hub">
          <div>
            <div className="awk-eyebrow">{NAMING.eyebrow}</div>
            <h2 className="awk-h2">{NAMING.title}</h2>
          </div>
          <div className="flex items-center gap-3">
            <span className="awk-dot" />
            <span className="awk-dim text-[14px]">{NAMING.loading}</span>
          </div>
        </div>
      </section>
    );
  }

  // 一个候选都没有（她一个字都还没写）——— 这一步没有意义，直接过。
  if (titles.length === 0 && !failed) {
    onDone("");
    return null;
  }

  return (
    <section className="awk-screen" aria-label="给这条线索起名字">
      <div className="awk-wrap awk-hub">
        <div>
          <div className="awk-eyebrow">{NAMING.eyebrow}</div>
          <h2 className="awk-h2">{NAMING.title}</h2>
          <p className="awk-p" style={{ marginTop: 10 }}>
            {NAMING.lead}
          </p>
          {failed ? (
            <p className="awk-p" style={{ color: "var(--danger)", marginTop: 12 }}>
              {NAMING.failed}
            </p>
          ) : null}
          <div className="mt-7 flex flex-wrap items-center gap-4">
            <Primary disabled={picked === ""} onClick={() => onDone(picked)}>
              {NAMING.confirm}
            </Primary>
            <Ghost onClick={() => onDone("")}>{NAMING.skip}</Ghost>
          </div>
        </div>

        <div className="awk-choice-stack" aria-label="名字候选">
          {titles.map((t, i) => (
            <Choice
              key={t}
              index={String(i + 1).padStart(2, "0")}
              title={t}
              /* 最后那个是从她原话裁出来的，说清楚 —— 她挑的时候知道这是谁写的。 */
              body={i === titles.length - 1 ? NAMING.ownWords : undefined}
              selected={picked === t}
              onClick={() => setPicked(t)}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
