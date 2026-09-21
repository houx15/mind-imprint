import { useEffect, useRef, useState } from "react";

import {
  type AwakeningReport,
  type AwakeningStage,
  type AwakeningThread,
  fetchAwakeningStatus,
  finishAwakening,
  fetchReport,
  reopenAwakeningThread,
  setThreadTitle,
} from "../api/awakening";
import { SYSTEM_VOICE, voiceUrl } from "./assets";
import "./awakening.css";
import { DOOR, HUB, TERMINAL } from "./content";
import { ReportView } from "./ReportView";
import { EnergyScene, NavigatorScene, TalentScene } from "./scenes/Cards";
import { BootScene } from "./scenes/Boot";
import {
  ArchiveScene,
  DeckScene,
  ObserverScene,
  RejoinScene,
  WarningScene,
  WorldScene,
} from "./scenes/Chapter";
import { planReentry } from "./reentry";
import { HubScene } from "./scenes/Hub";
import { LibraryScene } from "./scenes/Library";
import { NamingScene } from "./scenes/Naming";
import { ChallengeScene, LensScene, TerminalScene } from "./scenes/Terminal";
import { RoomShell } from "./RoomShell";
import { Ghost, Primary, Stage } from "./ui";
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
 * 第一趟是一条线。**复访不是** —— 她落在入口那一屏（`hub`），四件事自己挑，
 * 见 scenes/Hub.tsx。
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
  /** 关闭。`grew` 说这一趟有没有往树上写词，调用方可以据此决定说什么；
   *  重拉树和重查入口状态则**每次出门都做**（见 LiteApp）。 */
  onClose: (grew: boolean) => void;
  reportRunId?: string;
  onOpenReading: (slug: string, tier: number) => void;
}) {
  // 只在直接看报告之外的情况下开一趟 —— 点「查看兴趣印记」不该新开一趟。
  const { run, state, error, loading, go, patch, setRun, adopt, startFresh } =
    useAwakeningRun(open && !reportRunId);
  const [report, setReport] = useState<AwakeningReport | null>(null);
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState("");
  const [grew, setGrew] = useState(false);
  const voice = useVoice();

  // 复访的入口。`null` = 这一趟还没判断过（run 还在路上）。判一次就不再改，
  // 所以一趟里 `go` 触发的每次 run 刷新不会把她弹回菜单。
  const [hub, setHub] = useState<boolean | null>(null);
  // 从入口点进去的那一屏，走完之后回入口，而不是沿着第一趟那条线往下走。
  const [detour, setDetour] = useState(false);
  // 回顾剧情。**不存进 stage** —— 存进去就等于把她的进度倒回开场。
  const [replay, setReplay] = useState(false);
  // 「继续」要去的那一屏：接着没走完的那一趟，或者（重做时）直接进探询。
  const resumeRef = useRef<AwakeningStage>("terminal");
  /*
   * 盖在当前这一屏上面的一层：线索库、给线索起名。
   *
   * 和 `replay` 一样**不进 run.stage** —— 它们不是她在这条线索上走到的位置。
   */
  const [view, setView] = useState<"" | "library" | "naming">("");
  const [threads, setThreads] = useState<AwakeningThread[]>([]);
  // 线索库这一层的操作失败时后台那句原话。照实显示，不假装什么都没发生。
  const [libError, setLibError] = useState("");

  // 进门时把线索库拉一次：入口那张「兴趣线索库」要说清楚里面有几条，
  // 而那个数字在她点开库之前就得是对的。
  useEffect(() => {
    if (!open || reportRunId) return;
    let alive = true;
    fetchAwakeningStatus()
      .then((s) => {
        if (alive) setThreads(s.threads);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [open, reportRunId]);

  // 进门时判一次：她是不是回来的人。判据是纯函数，有测试（reentry.test.ts）。
  useEffect(() => {
    if (!run || hub !== null || reportRunId) return;
    const plan = planReentry(run);
    resumeRef.current = plan.resume;
    setHub(plan.hub);
  }, [run, hub, reportRunId]);

  /** 从入口跳到某一屏，并记住走完要回来。 */
  const detourTo = (stage: AwakeningStage) => {
    setDetour(true);
    setHub(false);
    go(stage);
  };

  /** 从一次绕路回到入口，把这一屏的产出一起存下来。 */
  const backToHub = (patchState?: Parameters<typeof go>[1]) => {
    setDetour(false);
    go(resumeRef.current, patchState);
    setHub(true);
  };

  // 关上门就把这一趟在这一层留下的痕迹清掉。
  //
  // 🚨 同上：组件不卸载。不清的话她走完一趟、出门、再进来，看见的是**上一趟
  // 的报告**，而不是新的一趟 —— 按钮写着「再做一次」，点下去却回到了上次的
  // 结果页。这一条和 useAwakeningRun 里那一条是同一个毛病的两半，缺一不可。
  useEffect(() => {
    if (open) return;
    setReport(null);
    setFinishing(false);
    setFinishError("");
    setGrew(false);
    setHub(null);
    setDetour(false);
    setReplay(false);
    setView("");
    setThreads([]);
    setLibError("");
  }, [open]);

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
    const url = hub ? undefined : clip[state.stage];
    if (url) voice.play(url);
    else voice.stop();
    // voice 的成员都是 useCallback 出来的，但把它整个列进依赖会让每次静音
    // 状态变化都重播一次当前这一屏。只认「哪一屏」和「哪个助手」。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, state.stage, state.navigator, hub]);

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

  /** 打开线索库，把她提出过的线索拉回来。 */
  const openLibrary = async () => {
    setLibError("");
    setView("library");
    try {
      const s = await fetchAwakeningStatus();
      setThreads(s.threads);
    } catch (e: unknown) {
      setLibError(e instanceof Error && e.message ? e.message : "请再试一次");
    }
  };

  /**
   * 接着问某一条线索。
   *
   * 已经总结过的那条也走这里：reopen 把章去掉，报告一份都不动
   * （产品负责人 2026-09-21：总结过的仍然点得进去）。
   */
  const openThread = async (t: AwakeningThread) => {
    setLibError("");
    try {
      const fresh = await reopenAwakeningThread(t.id);
      adopt(fresh);
      // 一条新开的线索停在 boot，但她已经看过剧情了 —— 直接进探询。
      const at: AwakeningStage = fresh.stage === "boot" ? "terminal" : fresh.stage;
      resumeRef.current = at;
      setView("");
      setHub(false);
      go(at);
    } catch (e: unknown) {
      setLibError(e instanceof Error && e.message ? e.message : "请再试一次");
    }
  };

  /** 开启新线索。旧的那几条原样停在库里，一个字都不动。 */
  const freshThread = async () => {
    setLibError("");
    try {
      await startFresh();
      resumeRef.current = "terminal";
      setView("");
      setHub(false);
      go("terminal");
    } catch (e: unknown) {
      setLibError(e instanceof Error && e.message ? e.message : "请再试一次");
    }
  };

  /**
   * 「暂时保留兴趣线索」按下之后：先给它起个名字，再出门。
   *
   * 名字是给线索库用的，而她第一次需要认出一条线索，正是在她把它放下、过几天
   * 回来的时候。放下的这一刻问，她脑子里还记得这条是关于什么的。
   */
  const nameAndLeave = async (title: string) => {
    if (title && run) {
      // 起名失败不挡她出门 —— 线索库那边没名字会显示她的原话。
      try {
        await setThreadTitle(run.id, title);
      } catch {
        /* 名字没存上，线索仍然在库里。 */
      }
    }
    onClose(grew);
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

    // 入口那一屏还没判出来（判断就在同一次 commit 之后）。这一帧什么都不画：
    // 否则复访的人会先闪一下她上次停下的那一屏，连带着播一句语音。
    if (hub === null) {
      return (
        <Stage>
          <div className="flex items-center justify-center gap-3 py-24">
            <span className="awk-dot" />
            <span className="awk-dim text-[14px]">正在连接</span>
          </div>
        </Stage>
      );
    }

    // 回顾剧情。放在 switch 前面，因为它盖在任何一屏上，走完回入口。
    if (replay) {
      return <BootScene onDone={() => setReplay(false)} />;
    }
    // 线索库和起名同样盖在任何一屏上 —— 它们不是她在这条线索上走到的位置。
    if (view === "library") {
      return (
        <>
          {libError ? (
            <p className="awk-p" style={{ color: "var(--danger)", padding: "0 24px" }}>
              {libError}
            </p>
          ) : null}
          <LibraryScene
            threads={threads}
            onOpen={(t) => void openThread(t)}
            onView={(t) => {
              void fetchReport(t.id)
                .then(setReport)
                .catch((e: unknown) =>
                  setLibError(e instanceof Error && e.message ? e.message : "读取失败"),
                );
            }}
            onFresh={() => void freshThread()}
            onBack={() => {
              setView("");
              setHub(true);
            }}
          />
        </>
      );
    }
    if (view === "naming" && run) {
      return <NamingScene runId={run.id} onDone={(t) => void nameAndLeave(t)} />;
    }
    if (hub) {
      return (
        <HubScene
          returning={run.attemptNo > 1 && run.stage === "boot"}
          turnsDone={run.turns.length}
          resume={resumeRef.current}
          navigator={state.navigator}
          hasEnergy={Boolean(state.energyProfile.domains?.length)}
          threadCount={threads.length}
          onContinue={() => {
            setHub(false);
            go(resumeRef.current);
          }}
          onLibrary={() => void openLibrary()}
          onEnergy={() => detourTo("energy")}
          onNavigator={() => detourTo("navigator")}
          onStory={() => setReplay(true)}
        />
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
        return (
          <EnergyScene
            doneLabel={detour ? HUB.back : undefined}
            onDone={(profile) =>
              detour
                ? backToHub({ energyProfile: profile })
                : go("navigator", { energyProfile: profile })
            }
          />
        );
      case "navigator":
        return (
          <NavigatorScene
            navigator={state.navigator}
            onPick={(id) => patch({ navigator: id })}
            onConfirm={() => (detour ? backToHub() : go("terminal"))}
          />
        );
      case "terminal":
        return (
          <TerminalScene
            run={run}
            onDone={() => go("lens")}
            // 暂时保留：先给这条线索起个名字，再出门。它原样留在线索库里。
            onHold={() => setView("naming")}
            // 现在总结：跳过后面那三屏，直接用她已经写下的话生成报告。
            onSummarize={() => {
              go("report");
              void finish();
            }}
          />
        );
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
      aria-label="兴趣测试"
      style={inRoom ? undefined : { background: "var(--mk-bg, #faf7f2)" }}
    >
      {inRoom ? (
        <div className="awk">
          {/* 顶栏、扫描线、4:3 舞台都在 RoomShell 里 —— 它每一屏都在。 */}
          <RoomShell
            stage={replay ? "boot" : view || hub ? "hub" : state.stage}
            attemptNo={run?.attemptNo ?? 1}
            muted={voice.muted}
            onToggleVoice={voice.toggle}
            onLeave={() => onClose(grew)}
          >
            {body()}
          </RoomShell>
        </div>
      ) : (
        body()
      )}
    </div>
  );
}
