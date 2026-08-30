import { ArrowLeft, ArrowRight, MessageCircle } from "lucide-react";
import { useEco } from "../store";
import { HOMEPAGE_STEPS } from "../data/homepage";
import { go } from "../route";
import { Btn, StepRail, Sys } from "../ui";
import { ExamplesStep } from "./hp/ExamplesStep";
import { InstructionStep } from "./hp/InstructionStep";
import { StyleStep } from "./hp/StyleStep";
import { ComposeStep } from "./hp/ComposeStep";
import { PublishStep } from "./hp/PublishStep";
import { ShareStep } from "./hp/ShareStep";

/**
 * PBL#0 · 建一个属于你的主页 — the studio shell.
 *
 * Six steps, forward AND backward: every step she has reached stays clickable
 * in the rail. A wizard that traps you is a form; a wizard you can walk back
 * through is a workspace, and this one is meant to be revisited (she will come
 * back to change her intro after her first project publishes).
 *
 * `canAdvance` is the only gate, and it gates on the thing that step exists to
 * produce — an example chosen, a mode chosen, a style chosen, an intro
 * written. Nothing is gated on time spent or on scrolling to the bottom.
 */
export function HomepageStudio() {
  const { state, hpGo, openCoach } = useEco();
  const hp = state.homepage;
  // `hpGo` is always called with an in-range index, but the array read is
  // still unchecked — clamp once here so every use below is a definite value.
  const step = Math.min(Math.max(hp.step, 0), HOMEPAGE_STEPS.length - 1);
  const meta = HOMEPAGE_STEPS[step]!;

  const gate: Record<number, { ok: boolean; why: string }> = {
    0: { ok: hp.exampleId !== null, why: "先挑一个你想学的页面" },
    1: { ok: hp.mode !== null, why: "先选一种和 AI 合作的方式" },
    2: { ok: hp.style !== null, why: "先选一个风格" },
    3: {
      ok: (hp.sections.find((s) => s.id === "intro")?.value.trim().length ?? 0) > 0,
      why: "至少把「一句话介绍」写出来",
    },
    4: { ok: hp.published, why: "先发布" },
    5: { ok: true, why: "" },
  };

  const canAdvance = gate[step]?.ok ?? true;
  const reachable = Math.max(step, hp.published ? 5 : 0);

  return (
    <div className="mx-auto max-w-[1180px] px-8 py-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Sys>项目 00 · 我的主页</Sys>
          <h1 className="mt-1 text-mk-h1 text-mk-ink">{meta.label}</h1>
          <p className="mt-1 text-mk-body text-mk-secondary">{meta.sub}</p>
        </div>
        <div className="flex items-center gap-2">
          <Btn variant="quiet" size="sm" iconStart={<MessageCircle size={15} />} onClick={() => openCoach("homepage")}>
            问印记
          </Btn>
          <Btn variant="quiet" size="sm" onClick={() => go({ name: "projects" })}>
            回项目页
          </Btn>
        </div>
      </div>

      <div className="mt-5 overflow-x-auto pb-1">
        <StepRail steps={HOMEPAGE_STEPS} current={step} onGo={hpGo} reachable={reachable} />
      </div>

      <hr className="eco-hair my-6" />

      <div key={step} className="eco-view-in">
        {step === 0 ? <ExamplesStep /> : null}
        {step === 1 ? <InstructionStep /> : null}
        {step === 2 ? <StyleStep /> : null}
        {step === 3 ? <ComposeStep /> : null}
        {step === 4 ? <PublishStep /> : null}
        {step === 5 ? <ShareStep /> : null}
      </div>

      {/* footer nav — always visible, so she always knows what "next" costs */}
      {step < 5 ? (
        <div className="sticky bottom-0 mt-10 -mx-8 flex items-center justify-between gap-4 border-t border-mk-border px-8 py-4"
             style={{ background: "color-mix(in srgb, var(--mk-paper) 92%, transparent)", backdropFilter: "blur(8px)" }}>
          <Btn
            variant="quiet"
            iconStart={<ArrowLeft size={16} />}
            disabled={step === 0}
            onClick={() => hpGo(Math.max(0, step - 1))}
          >
            上一步
          </Btn>
          <div className="flex items-center gap-3">
            {!canAdvance ? (
              <span className="text-mk-small text-mk-muted">{gate[step]?.why}</span>
            ) : null}
            <Btn disabled={!canAdvance} onClick={() => hpGo(Math.min(5, step + 1))}>
              {step === 3 ? "去发布" : "下一步"}
              <ArrowRight size={16} />
            </Btn>
          </div>
        </div>
      ) : null}
    </div>
  );
}
