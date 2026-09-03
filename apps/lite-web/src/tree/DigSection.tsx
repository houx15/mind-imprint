import { useEffect, useState } from "react";
import { Hammer, Loader2, PenLine, BookOpen, Sparkles } from "lucide-react";
import { fetchKeywordDig, type DigKind, type DigSeed } from "../api/interest";
import { createReading } from "../api/readings";
import { createWriting } from "../api/writings";
import { createProject } from "../api/projects";
import { liteRoutePath, navigate } from "../routing";
import { Sys } from "./ui";

/**
 * 继续深挖 —— 一个关键词后面的四颗种子。
 *
 * # 为什么这一节值得单独存在
 *
 * 抽屉的前两节证明了这个词是从哪来的。到此为止，模型对她有观察，却对「那接下来
 * 干嘛」一无所知。原型在这里摆过四个通用动词（再读一篇 / 写一篇 / 做个项目 /
 * 问印记）—— **四个空动词摆在一个真的观察后面，教的是这个模型没有真的在看她。**
 *
 * 现在四颗种子由模型按**她在这个词上留下的原话**生成，而且三颗能一键变成 lite
 * 里一个真的房间：`readings` / `writings` / `pbl_project` 的创建接口都只要一个
 * 字段，所以种子的正文可以直接当标题送进去。这就是「想法」和「已经开始了」之间
 * 的那一步。
 *
 * # 想一想那颗不导航
 *
 * 它是一个**拿着走的问题**，不是一个任务。给它加一个按钮，就又变回四个空动词
 * 了 —— 只不过这次是四个具体的空动词。
 *
 * # 失败时这一节是空的
 *
 * 服务端生成失败会返回零颗种子加一句原话，界面照实说。**绝不自己补四个通用
 * 动词** —— 那正是这套东西要取代的东西。
 */

const META: Record<DigKind, { label: string; hue: string; icon: typeof BookOpen | null; cta: string }> = {
  think: { label: "想一想", hue: "var(--mk-gold)", icon: null, cta: "" },
  read: { label: "去读", hue: "var(--mk-lake)", icon: BookOpen, cta: "在阅读室打开" },
  write: { label: "去写", hue: "var(--mk-peach)", icon: PenLine, cta: "在写作间打开" },
  make: { label: "去做", hue: "var(--mk-taro)", icon: Hammer, cta: "开一个项目" },
};

export function DigSection({ keywordId }: { keywordId: string }) {
  const [seeds, setSeeds] = useState<DigSeed[] | null>(null);
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<DigKind | null>(null);

  useEffect(() => {
    let alive = true;
    setSeeds(null);
    setNote("");
    setError("");
    fetchKeywordDig(keywordId)
      .then((d) => {
        if (!alive) return;
        setSeeds(d.seeds);
        setNote(d.note);
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      alive = false;
    };
  }, [keywordId]);

  /** 把一颗种子变成一个真的房间，然后走进去。 */
  async function follow(seed: DigSeed) {
    setBusy(seed.kind);
    setError("");
    try {
      if (seed.kind === "read") {
        const r = await createReading({ title: seed.text });
        navigate(liteRoutePath({ tab: "readings", readingId: r.id }));
      } else if (seed.kind === "write") {
        const w = await createWriting({ idea: seed.text });
        navigate(liteRoutePath({ tab: "writings", writingId: w.id }));
      } else if (seed.kind === "make") {
        const p = await createProject(seed.text);
        navigate(liteRoutePath({ tab: "projects", projectId: p.id }));
      }
    } catch (e: unknown) {
      // 动词 + 失败，再接后台原话（AGENTS.md §8）。
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="mt-7">
      <div className="tree-scanline mb-5" />
      <h3 className="text-mk-h3 text-[#EFE7DC]">继续深挖</h3>
      <p className="mt-1 text-mk-small leading-[1.8] text-[#8E8175]">
        这四条是按你在这个词上写过的话生成的。点一个，它会带着这句话去到该去的地方。
      </p>

      {seeds === null && !error ? (
        <p className="mt-4 flex items-center gap-2 text-mk-small text-[#8E8175]">
          <Loader2 size={14} className="animate-spin" />
          正在按你写过的话生成
        </p>
      ) : null}

      {/* 后台原话原样给出。空着比摆四个通用动词诚实。 */}
      {note ? (
        <p className="mt-4 rounded-mk-md p-3 text-mk-small leading-[1.8] text-[#9A8E80]"
           style={{ border: "1px solid rgba(240,233,224,.14)", background: "rgba(240,233,224,.035)" }}>
          {note}
        </p>
      ) : null}

      {error ? (
        <p className="mt-4 rounded-mk-md p-3 text-mk-small leading-[1.8] text-[#F0D5D9]"
           style={{ background: "rgba(255,113,137,.12)", border: "1px solid rgba(255,113,137,.4)" }}>
          操作失败：{error}
        </p>
      ) : null}

      {seeds && seeds.length > 0 ? (
        <div className="mt-4 space-y-2.5">
          {seeds.map((s, i) => {
            const meta = META[s.kind];
            const Icon = meta.icon;
            return (
              <div
                key={s.kind}
                className="tree-in rounded-mk-md p-4"
                style={{
                  ["--i" as string]: i,
                  border: `1px solid color-mix(in srgb, ${meta.hue} 40%, transparent)`,
                  background: `color-mix(in srgb, ${meta.hue} 11%, rgba(240,233,224,.03))`,
                }}
              >
                <span className="tree-mono" style={{ color: meta.hue, letterSpacing: "0.1em" }}>
                  {meta.label}
                </span>
                <p className="mt-1.5 text-mk-body leading-[1.7] text-[#EDE4D9]">{s.text}</p>
                {s.why ? (
                  <p className="mt-1.5 text-mk-small leading-[1.7] text-[#8E8175]">{s.why}</p>
                ) : null}

                {Icon ? (
                  <button
                    type="button"
                    onClick={() => follow(s)}
                    disabled={busy !== null}
                    className="mt-3 inline-flex items-center gap-1.5 rounded-mk-full px-3.5 py-1.5 text-mk-small
                               font-semibold text-[#17130F] transition hover:opacity-90 disabled:opacity-45"
                    style={{ background: meta.hue }}
                  >
                    {busy === s.kind ? (
                      <>
                        <Loader2 size={13} className="animate-spin" />
                        处理中
                      </>
                    ) : (
                      <>
                        <Icon size={13} strokeWidth={2} />
                        {meta.cta}
                      </>
                    )}
                  </button>
                ) : (
                  // 想一想没有按钮：它是一个拿着走的问题，不是一个任务。
                  <p className="mt-2.5 flex items-center gap-1.5 text-mk-small text-[#7C7166]">
                    <Sparkles size={12} strokeWidth={2} />
                    这一条不用点，带着它去读下一篇就好。
                  </p>
                )}
              </div>
            );
          })}
        </div>
      ) : null}

      {seeds && seeds.length > 0 ? (
        <p className="mt-3 text-mk-small text-[#7C7166]">
          <Sys tone="dark">说明</Sys>{" "}
          点「去读 / 去写 / 开一个项目」会用这句话新建一个房间，并直接带你进去。
        </p>
      ) : null}
    </div>
  );
}
