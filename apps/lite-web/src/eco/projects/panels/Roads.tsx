import { useState } from "react";
import { ArrowRight, MessageCircle } from "lucide-react";
import { useEco } from "../../store";
import type { Approach, Project } from "../../data/types";
import { Btn, Field, Sys, cx } from "../../ui";

/**
 * 选路 — 印记 proposes roads and stops.
 *
 * This screen is where the product's central claim is either true or it is
 * decoration. 印记 has just been told a real problem, it can see two ways to
 * solve it, and it says both — **with the price of each named out loud** —
 * and then does nothing further.
 *
 * ## Three rules
 * 1. **The roads must be different KINDS of answer.** One lives on a screen
 *    and can be wrong and fixed in a minute; the other is screwed to a post
 *    and is right or it is not. Two flavours of one idea would teach that
 *    choosing is cosmetic.
 * 2. **Every road carries its costs**, written by 印记, unprompted. An option
 *    presented without its price has already been chosen for her.
 * 3. **A decision is not a click.** It is a road, plus why, plus what she is
 *    knowingly giving up. The button does not enable until all three exist,
 *    and the reason is quoted back at her for the rest of the project.
 *
 * Each road is also a HOOK: 单独问问这一条 opens a branch conversation about
 * that option alone (`BranchTalk`), and what she brings back out of it shows
 * on the card here as evidence she gathered rather than a mood she had.
 */
export function Roads({
  project,
  onOpenBranch,
}: {
  project: Project;
  onOpenBranch: (approachId: string) => void;
}) {
  const { decideRoad } = useEco();
  const [pick, setPick] = useState<string | null>(null);
  const [why, setWhy] = useState("");
  const [gaveUp, setGaveUp] = useState("");

  const chosen = project.approaches.find((a) => a.id === pick);
  // Only once she has picked. Without the guard this finds the FIRST road
  // whenever `pick` is null, so the empty form asked her what she was giving
  // up about the road she had not chosen — naming the wrong one.
  const other = pick ? project.approaches.find((a) => a.id !== pick) : undefined;
  const ready = Boolean(pick) && why.trim().length >= 6 && gaveUp.trim().length >= 4;

  return (
    <div className="h-full overflow-y-auto px-6 py-5">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <Sys>选路 · 印记不替你选</Sys>
        <span
          className="eco-mono rounded-mk-full px-2 py-0.5 text-mk-faint"
          style={{ border: "1px solid var(--mk-border)" }}
          title="这两条路是为原型里的示例问题写好的"
        >
          原型数据
        </span>
      </div>
      <h2 className="mt-1 text-mk-h1 text-mk-ink">我想到 {project.approaches.length} 条路</h2>
      <p className="mt-1.5 max-w-[68ch] text-mk-body leading-[1.85] text-mk-secondary">
        它们不是同一件事的两种做法。每条我都写了它要付的代价——
        <span className="text-mk-ink">看不到代价的选项，等于已经替你选了</span>。
        想细问哪一条，就点开它单独聊，聊完把你想清楚的那一句带回来。
      </p>

      <ul className="mt-5 grid gap-4 lg:grid-cols-2">
        {project.approaches.map((a) => (
          <li key={a.id}>
            <Road
              approach={a}
              takeaway={project.branches.find((b) => b.approachId === a.id)?.takeaway ?? ""}
              asked={project.branches.find((b) => b.approachId === a.id)?.log.length ?? 0}
              picked={pick === a.id}
              onPick={() => setPick(a.id)}
              onAsk={() => onOpenBranch(a.id)}
            />
          </li>
        ))}
      </ul>

      {/* ── the decision ────────────────────────────────────────────── */}
      <div
        className="mt-6 rounded-mk-lg border p-5"
        style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }}
      >
        <Sys className="!text-mk-accent-700">你的决定</Sys>
        <p className="mt-1.5 text-mk-body leading-[1.85] text-mk-ink">
          {chosen ? (
            <>
              你选的是 <span className="font-semibold">{chosen.name}</span>。
              现在写清楚为什么——这句话我会一直留着，做到一半你怀疑自己的时候可以回来看。
            </>
          ) : (
            "先在上面点一条。"
          )}
        </p>

        <div className="mt-4 space-y-4">
          <Field
            label="为什么是这条"
            hint="不要写「感觉更好」。写你在哪一点上被说服了。"
            value={why}
            onChange={setWhy}
            rows={3}
            placeholder={
              chosen
                ? "例如：因为它错了可以改。牌子装上去就很难动，而我现在还不确定几个点才够。"
                : "先选一条路"
            }
          />
          <Field
            label={other ? `你放弃了「${other.name}」的什么` : "你放弃了什么"}
            hint="说得出放弃了什么，才算真的选了。"
            value={gaveUp}
            onChange={setGaveUp}
            rows={2}
            placeholder="例如：不带手机的老人用不上我这个东西。这是我知道的、也接受的代价。"
          />
        </div>

        <Btn
          className="mt-4"
          disabled={!ready}
          iconStart={<ArrowRight size={16} strokeWidth={2} />}
          onClick={() => pick && decideRoad(project.id, pick, why.trim(), gaveUp.trim())}
        >
          就走这条，给我排计划
        </Btn>
        {!ready ? (
          <p className="mt-2 text-mk-small text-mk-muted">
            选一条 + 两栏都写了，才能往下走。
          </p>
        ) : null}
      </div>
    </div>
  );
}

function Road({
  approach,
  takeaway,
  asked,
  picked,
  onPick,
  onAsk,
}: {
  approach: Approach;
  takeaway: string;
  asked: number;
  picked: boolean;
  onPick: () => void;
  onAsk: () => void;
}) {
  return (
    <div
      className={cx(
        "flex h-full flex-col rounded-mk-lg border p-5 transition-colors duration-[140ms]",
        picked ? "shadow-mk-md" : "",
      )}
      style={{
        borderColor: picked ? approach.hue : "var(--mk-border)",
        background: picked
          ? `color-mix(in srgb, ${approach.hue} 8%, var(--mk-surface))`
          : "var(--mk-surface)",
      }}
    >
      <div className="flex items-center gap-2">
        <span className="h-2.5 w-2.5 rounded-mk-full" style={{ background: approach.hue }} />
        <Sys>{approach.shape}</Sys>
      </div>
      <h3 className="mt-1.5 text-mk-h2 text-mk-ink">{approach.name}</h3>

      <ul className="mt-3 space-y-1.5">
        {approach.how.map((h) => (
          <li key={h} className="flex gap-2 text-mk-body leading-[1.8] text-mk-secondary">
            <span className="text-mk-faint">·</span>
            {h}
          </li>
        ))}
      </ul>

      <p className="mt-3.5 text-mk-small font-semibold text-mk-ink">它要付的代价</p>
      <ul className="mt-1 space-y-1">
        {approach.costs.map((c) => (
          <li key={c} className="flex gap-2 text-mk-small leading-[1.75] text-mk-secondary">
            <span style={{ color: approach.hue }}>·</span>
            {c}
          </li>
        ))}
      </ul>

      <p className="mt-3.5 rounded-mk-md p-3 text-mk-small leading-[1.8] text-mk-ink"
         style={{ background: "var(--mk-paper)" }}>
        <span className="text-mk-muted">它需要你：</span>
        {approach.needs}
      </p>

      {takeaway ? (
        <p
          className="mt-3 rounded-mk-md border-l-2 p-3 text-mk-small leading-[1.8] text-mk-ink"
          style={{ borderColor: approach.hue, background: "var(--mk-paper)" }}
        >
          <span className="eco-mono block text-mk-faint">你问完之后带回来的</span>
          {takeaway}
        </p>
      ) : null}

      <div className="mt-auto flex flex-wrap gap-2 pt-4">
        <Btn
          size="sm"
          variant={picked ? "primary" : "outline"}
          onClick={onPick}
        >
          {picked ? "已选这条" : "选这条"}
        </Btn>
        <Btn
          size="sm"
          variant="quiet"
          iconStart={<MessageCircle size={14} strokeWidth={1.9} />}
          onClick={onAsk}
        >
          {asked > 0 ? `接着问（问过 ${Math.ceil(asked / 2)} 个）` : "单独问问这一条"}
        </Btn>
      </div>
    </div>
  );
}
