import type { AwakeningThread } from "../../api/awakening";
import { LIBRARY } from "../content";
import { libraryRows } from "../library";
import { Choice, Ghost } from "../ui";

/**
 * 兴趣线索库。她提出过的每一条都在这里。
 *
 * # 它替掉了什么
 *
 * 2026-09-20 那一版里，「换一条线索」只能实现成**清空**：库里一个人最多有
 * 一趟没走完的作答，想换个话题就得把上次写的删掉。
 *
 * 2026-09-21 的反馈直接说了这件事不成立：
 *
 *   「如果学生只是暂时对上次的线索没有进一步的想法，想先放一放，清空了就
 *     没有记录了。」
 *
 * 所以这一屏里没有「清空」这个动作。停下就是停下，每一条都留着。
 *
 * # 总结过的线索不是只读的
 *
 * 它仍然摆在库里，也仍然可以点进去接着问（产品负责人：「she should be able
 * to click to continue on that」）。总结是这条线索到目前为止的一份交代，
 * 不是它的终点。
 */

export function LibraryScene({
  threads,
  onOpen,
  onView,
  onFresh,
  onBack,
}: {
  threads: AwakeningThread[];
  /** 接着问某一条。已经总结过的那条也走这里。 */
  onOpen: (thread: AwakeningThread) => void;
  /** 看某一条已经总结出来的兴趣印记。 */
  onView: (thread: AwakeningThread) => void;
  onFresh: () => void;
  onBack: () => void;
}) {
  const rows = libraryRows(threads, new Date());

  return (
    <section className="awk-screen" aria-label="兴趣线索库">
      <div className="awk-wrap awk-hub">
        <div>
          <div className="awk-eyebrow">{LIBRARY.eyebrow}</div>
          <h2 className="awk-h2">{LIBRARY.title}</h2>
          <p className="awk-p" style={{ marginTop: 10 }}>
            {rows.length === 0 ? LIBRARY.empty : LIBRARY.lead}
          </p>
          <div className="mt-7">
            <Ghost onClick={onBack}>{LIBRARY.back}</Ghost>
          </div>
        </div>

        <div className="awk-choice-stack" aria-label="你提出过的线索">
          {rows.map((row, i) => {
            const t = threads[i]!;
            return (
              <div key={row.id} className="awk-thread">
                <Choice
                  index={String(i + 1).padStart(2, "0")}
                  hwId={row.summarized ? "DONE" : "OPEN"}
                  title={row.name}
                  body={[row.state, row.when, row.extra].filter(Boolean).join(" · ")}
                  onClick={() => onOpen(t)}
                />
                {/* 总结过的那条多一条路：去看那份印记，而不是接着问。 */}
                {row.summarized ? (
                  <button type="button" className="awk-thread-view" onClick={() => onView(t)}>
                    {LIBRARY.view}
                  </button>
                ) : null}
              </div>
            );
          })}

          <Choice
            index="＋"
            hwId="NEW"
            title={LIBRARY.fresh}
            body={LIBRARY.freshBody}
            onClick={onFresh}
          />
        </div>
      </div>
    </section>
  );
}
