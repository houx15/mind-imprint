import { useEffect, useState } from "react";

import { type AwakeningStatus, fetchAwakeningStatus } from "../api/awakening";

/**
 * 她和觉醒协议之间的状态。
 *
 * 🚨 **三态，不是布尔。** `null` = 还不知道（请求在飞，或者失败了）。这个区别
 * 有实际后果：把「不知道」当成「没做过」，会在她每次打开树时闪一下那条邀请，
 * 而她上个月已经走过一趟了；把「不知道」当成「做过了」，则是把入口藏起来。
 * 不知道的时候两个都不做。
 *
 * 这条规矩从 useQuizStatus 继承下来，那个文件随七屏兴趣测试一起删掉了。
 */
export function useAwakeningStatus(nonce = 0): AwakeningStatus | null {
  const [status, setStatus] = useState<AwakeningStatus | null>(null);

  useEffect(() => {
    let alive = true;
    fetchAwakeningStatus()
      .then((s) => {
        if (alive) setStatus(s);
      })
      .catch(() => {
        // 静默：一个读不到的状态不该在她的树上弹一条报错。入口照常显示
        // —— 只有明确知道她走过一趟时才换掉那句话。
        if (alive) setStatus(null);
      });
    return () => {
      alive = false;
    };
  }, [nonce]);

  return status;
}
