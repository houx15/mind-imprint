import { useCallback, useEffect, useRef, useState } from "react";

import {
  type AwakeningRoute,
  type AwakeningRun,
  type AwakeningStage,
  type EnergyProfile,
  type TalentState,
  saveAwakening,
  startAwakening,
} from "../api/awakening";

/**
 * useAwakeningRun —— 这一趟的全部状态，以及它怎么存回服务端。
 *
 * # 为什么状态在前端，进度在服务端
 *
 * 十四屏里有九屏根本不需要服务端参与（剧情、档案、底牌、卡牌）。每翻一页发一次
 * 请求会让这个房间在网络差的时候卡住，而卡住的代价是她关掉页面。
 *
 * 所以：**状态在前端走，换屏时把整份状态存一次**。存失败不挡她往下走（那一屏
 * 她已经看完了），只是下次进来会从上一个存成功的屏接着走。
 *
 * # 存的是整份，不是增量
 *
 * `saveAwakening` 每次带全字段，服务端整行覆盖。它因此幂等：一次网络重试的
 * 结果和只发一次完全一样。代价是这里必须始终持有全份状态 —— 少带一个字段就是
 * 把它清空。
 */

/** 这一趟在前端持有的全份状态。字段和 `saveAwakening` 的入参一一对应。 */
export interface RunState {
  stage: AwakeningStage;
  route: AwakeningRoute;
  navigator: string;
  energyProfile: EnergyProfile;
  talent: TalentState;
  lensChoice: string;
  challengeChoice: string;
  archiveAttempts: number;
  observerQuestion: string;
}

function stateOf(run: AwakeningRun): RunState {
  return {
    stage: run.stage,
    route: run.route,
    navigator: run.navigator,
    energyProfile: run.energyProfile,
    talent: run.talent,
    lensChoice: run.lensChoice,
    challengeChoice: run.challengeChoice,
    archiveAttempts: run.archiveAttempts,
    observerQuestion: run.observerQuestion,
  };
}

export interface UseAwakeningRun {
  /** 服务端那一行。`turns` 和 `openingAsk` 只从这里读。 */
  run: AwakeningRun | null;
  /** 前端持有的状态。每一屏读它、改它。 */
  state: RunState;
  /** 开一趟（或接上没走完的那一趟）失败时的原话。 */
  error: string;
  loading: boolean;
  /**
   * 改状态并换屏。
   *
   * `patch` 只带这一屏改动的字段；其余从当前状态继承，所以调用点不用每次
   * 都写全九个字段。存回服务端仍然是整份。
   */
  go: (stage: AwakeningStage, patch?: Partial<RunState>) => void;
  /** 只改状态不换屏（同一屏里的多次选择）。不触发存盘。 */
  patch: (patch: Partial<RunState>) => void;
  /** 把服务端那一行换成新的（终端发完一轮之后要刷新 turns）。 */
  setRun: (run: AwakeningRun) => void;
}

export function useAwakeningRun(enabled: boolean): UseAwakeningRun {
  const [run, setRunRaw] = useState<AwakeningRun | null>(null);
  const [state, setState] = useState<RunState>({
    stage: "boot",
    route: "",
    navigator: "",
    energyProfile: {},
    talent: {},
    lensChoice: "",
    challengeChoice: "",
    archiveAttempts: 0,
    observerQuestion: "",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  // 存盘用的 id 和最新状态放在 ref 里：存盘发生在事件回调里，而回调闭包捕获的
  // 是那一次渲染时的值。用 ref 读，存出去的永远是刚改完的那一份。
  const idRef = useRef("");
  const stateRef = useRef(state);
  stateRef.current = state;

  useEffect(() => {
    // 🚨 关上门要把这一行丢掉，并且回到「正在连接」。
    //
    // 这个 hook 的宿主（AwakeningRoom）**不会卸载** —— 它只是 return null。
    // 不丢的话，她走完一趟再进来，第一帧读到的还是上一趟那一行；而那一行
    // finished_at 不为空，于是房间立刻去拉它的报告，把新开的一趟盖掉。
    // 2026-09-20 反馈的「完成一次之后再次进入无法从头开始」就是它。
    if (!enabled) {
      setRunRaw(null);
      idRef.current = "";
      setLoading(true);
      return;
    }
    let alive = true;
    setLoading(true);
    startAwakening()
      .then((r) => {
        if (!alive) return;
        idRef.current = r.id;
        setRunRaw(r);
        setState(stateOf(r));
        setError("");
      })
      .catch((e: unknown) => {
        if (!alive) return;
        // 报错照实说，带后台原话 —— 学生和我们看到同一句
        // （AGENTS.md §界面文案怎么写 第 8 条）。
        setError(e instanceof Error && e.message ? e.message : "无法开始");
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [enabled]);

  const patch = useCallback((p: Partial<RunState>) => {
    setState((s) => ({ ...s, ...p }));
  }, []);

  const go = useCallback((stage: AwakeningStage, p: Partial<RunState> = {}) => {
    const next: RunState = { ...stateRef.current, ...p, stage };
    stateRef.current = next;
    setState(next);
    const id = idRef.current;
    if (!id) return;
    // 存盘不挡她往下走。失败只意味着下次进来从上一个存成功的屏接着走，
    // 而她刚看完的那一屏本来也不需要服务端参与。
    void saveAwakening(id, next)
      .then((r) => setRunRaw(r))
      .catch(() => undefined);
  }, []);

  const setRun = useCallback((r: AwakeningRun) => setRunRaw(r), []);

  return { run, state, error, loading, go, patch, setRun };
}
