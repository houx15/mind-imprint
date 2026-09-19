import { useEffect, useState } from "react";

import { type AwakeningReport, finishAwakening, fetchReport } from "../api/awakening";
import { SYSTEM_VOICE, voiceUrl } from "./assets";
import "./awakening.css";
import { DOOR, TERMINAL } from "./content";
import { ReportView } from "./ReportView";
import { EnergyScene, NavigatorScene, TalentScene } from "./scenes/Cards";
import {
  ArchiveScene,
  BootScene,
  DeckScene,
  ObserverScene,
  RejoinScene,
  WarningScene,
  WorldScene,
} from "./scenes/Prologue";
import { ChallengeScene, LensScene, TerminalScene } from "./scenes/Terminal";
import { Ghost, Pill, Primary, Stage } from "./ui";
import { useAwakeningRun } from "./useAwakeningRun";
import { useVoice } from "./useVoice";

/**
 * AwakeningRoom —— 这个房间本身。
 *
 * # 它是一个房间，不是一条路由
 *
 * 门槛那条约束（spec §7）在这里落地：它铺满整个视口，**里面不出现 lite 的
 * 导航、页头和返回按钮**。她在里面的时候，产品的其它部分不在。
 *
 * 进门由调用方（TreeView）负责：树暗下去，让位给这里，一次动作，不跳路由。
 * 出门是同一个动作反过来 —— 报告那一屏是暖色的，所以温差本身就是「回来了」。
 *
 * # 分支
 *
 *   boot → world ─┬─ joined ────────────────────────────→ energy
 *                 └─ observer → warning → archive → deck → rejoin ─┬→ energy
 *                                                                  └→ observer（落点，可再回来）
 *   energy → navigator → terminal → lens → challenge → talent → report
 */

export function AwakeningRoom({
  open,
  onClose,
  /** 打开时直接显示某一份已经生成的报告（从树上点「查看兴趣印记」进来）。 */
  reportRunId,
  onOpenReading,
}: {
  open: boolean;
  /** 关闭。`grew` 为真表示树上的词可能变了，调用方据此重新拉一次树。 */
  onClose: (grew: boolean) => void;
  reportRunId?: string;
  onOpenReading: (slug: string, tier: number) => void;
}) {
  // 只在直接看报告之外的情况下开一趟 —— 点「查看兴趣印记」不该新开一趟。
  const { run, state, error, loading, go, patch, setRun } = useAwakeningRun(
    open && !reportRunId,
  );
  const [report, setReport] = useState<AwakeningReport | null>(null);
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState("");
  const [grew, setGrew] = useState(false);
  const voice = useVoice();

  // 一屏一句。换屏就换那一句，上一句停掉 —— 否则她快速翻过三屏会同时听见
  // 三个人说话。静音时 play 什么都不做，所以这个 effect 照常跑没有代价。
  useEffect(() => {
    if (!open) return;
    const g = state.navigator;
    const clip: Partial<Record<string, string>> = {
      boot: SYSTEM_VOICE.intro,
      warning: SYSTEM_VOICE.warning,
      navigator: voiceUrl(g, "quote"),
      terminal: voiceUrl(g, "connect"),
      lens: voiceUrl(g, "lens"),
      challenge: voiceUrl(g, "challenge"),
    };
    const url = clip[state.stage];
    if (url) voice.play(url);
    else voice.stop();
    // voice 的成员都是 useCallback 出来的，但把它整个列进依赖会让每次静音
    // 状态变化都重播一次当前这一屏。只认「哪一屏」和「哪个助手」。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, state.stage, state.navigator]);

  // 报告那一屏的那一句，跟着报告出现而不是跟着 stage —— 生成要等几秒，
  // 在等待时就播完会让她听见一句对不上的话。
  useEffect(() => {
    if (!open || !report) return;
    voice.play(voiceUrl(report.navigator, "result"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, report]);

  // 直接看一份旧报告。
  useEffect(() => {
    if (!open || !reportRunId) return;
    let alive = true;
    fetchReport(reportRunId)
      .then((r) => {
        if (alive) setReport(r);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [open, reportRunId]);

  // 她走到过的最后一屏如果是 report，说明这一趟已经走完，直接读那份报告。
  useEffect(() => {
    if (!run || report || reportRunId) return;
    if (!run.finishedAt) return;
    let alive = true;
    fetchReport(run.id)
      .then((r) => {
        if (alive) setReport(r);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [run, report, reportRunId]);

  // 房间打开时锁住背后那一层的滚动 —— 否则她在房间里滚，背后的树也在滚。
  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  if (!open) return null;

  const finish = async () => {
    if (!run || finishing) return;
    setFinishing(true);
    setFinishError("");
    try {
      const r = await finishAwakening(run.id);
      setReport(r);
      setGrew(r.pursuing.length > 0);
    } catch (e: unknown) {
      setFinishError(e instanceof Error && e.message ? e.message : "生成失败");
    } finally {
      setFinishing(false);
    }
  };

  const body = () => {
    if (report) {
      return (
        <ReportView
          report={report}
          onBackToTree={() => onClose(grew)}
          onOpenReading={(slug, tier) => {
            onClose(grew);
            onOpenReading(slug, tier);
          }}
        />
      );
    }
    if (finishing) {
      return (
        <Stage>
          <div className="flex flex-col items-center gap-5 py-24 text-center">
            <span className="awk-dot" />
            <p className="text-[17px]">{TERMINAL.finishing}</p>
            <p className="awk-dim text-[13px]">正在从你写下的内容里选词，并写进你的树。</p>
          </div>
        </Stage>
      );
    }
    if (loading) {
      return (
        <Stage>
          <div className="flex items-center justify-center gap-3 py-24">
            <span className="awk-dot" />
            <span className="awk-dim text-[14px]">正在连接</span>
          </div>
        </Stage>
      );
    }
    if (error || !run) {
      return (
        <Stage>
          <div className="py-24 text-center">
            <p className="text-[17px]" style={{ color: "var(--danger)" }}>
              连接失败：{error || "没有拿到作答"}
            </p>
            <div className="mt-6">
              <Ghost onClick={() => onClose(false)}>{DOOR.leave}</Ghost>
            </div>
          </div>
        </Stage>
      );
    }

    switch (state.stage) {
      case "boot":
        return <BootScene onDone={() => go("world")} />;
      case "world":
        return (
          <WorldScene
            onChoose={(route) => go(route === "joined" ? "energy" : "warning", { route })}
          />
        );
      case "warning":
        return <WarningScene onNext={() => go("archive")} />;
      case "archive":
        return (
          <ArchiveScene
            attempts={state.archiveAttempts}
            onAttempt={() => patch({ archiveAttempts: state.archiveAttempts + 1 })}
            onNext={() => go("deck")}
          />
        );
      case "deck":
        return <DeckScene onDone={() => go("rejoin")} />;
      case "rejoin":
        return (
          <RejoinScene
            onJoin={() => go("energy", { route: "joined" })}
            onStay={() => go("observer", { route: "observer" })}
          />
        );
      case "observer":
        return (
          <ObserverScene
            question={state.observerQuestion}
            onPick={(key) => patch({ observerQuestion: key })}
            onConfirm={() => go("observer")}
            onRejoin={() => go("energy", { route: "joined" })}
            onLeave={() => onClose(false)}
          />
        );
      case "energy":
        return <EnergyScene onDone={(profile) => go("navigator", { energyProfile: profile })} />;
      case "navigator":
        return (
          <NavigatorScene
            navigator={state.navigator}
            onPick={(id) => patch({ navigator: id })}
            onConfirm={() => go("terminal")}
          />
        );
      case "terminal":
        return <TerminalScene run={run} onDone={() => go("lens")} />;
      case "lens":
        return (
          <LensScene
            choice={state.lensChoice}
            onPick={(key) => patch({ lensChoice: key })}
            onConfirm={() => go("challenge")}
          />
        );
      case "challenge":
        return (
          <ChallengeScene
            choice={state.challengeChoice}
            onPick={(key) => patch({ challengeChoice: key })}
            onConfirm={() => go("talent")}
          />
        );
      case "talent":
        return (
          <TalentScene
            onDone={(talent) => {
              go("report", { talent });
              void finish();
            }}
          />
        );
      case "report":
        return (
          <Stage>
            <div className="py-24 text-center">
              <p className="text-[17px]" style={{ color: "var(--danger)" }}>
                {finishError ? `生成失败：${finishError}` : "报告还没有生成"}
              </p>
              <div className="mt-6 flex justify-center gap-4">
                <Primary onClick={() => void finish()}>重新生成</Primary>
                <Ghost onClick={() => onClose(grew)}>{DOOR.leave}</Ghost>
              </div>
            </div>
          </Stage>
        );
      default:
        return null;
    }
  };

  const inRoom = !report;

  return (
    <div
      className="fixed inset-0 z-50 overflow-y-auto"
      role="dialog"
      aria-modal="true"
      aria-label="觉醒协议"
      style={inRoom ? undefined : { background: "var(--mk-bg, #faf7f2)" }}
    >
      {inRoom ? (
        <div className="awk min-h-full">
          {/* 房间里唯一的常驻件：连接状态，和一个离开。没有导航，没有页头。 */}
          <div className="sticky top-0 z-20 flex items-center justify-between gap-4 px-5 py-4 sm:px-8">
            <Pill>AWAKENING PROTOCOL</Pill>
            <div className="flex items-center gap-3">
              {/* 声音。默认关，一个常驻开关，任何一屏都不等它。 */}
              <button
                type="button"
                className="awk-ghost"
                onClick={voice.toggle}
                aria-pressed={voice.muted ? "false" : "true"}
              >
                {voice.muted ? "声音已关闭" : "声音已开启"}
              </button>
              <button
                type="button"
                className="awk-ghost"
                onClick={() => onClose(grew)}
                title={DOOR.leaveConfirm}
              >
                {DOOR.leave}
              </button>
            </div>
          </div>
          {body()}
        </div>
      ) : (
        body()
      )}
    </div>
  );
}
