import { useEffect, useState } from "react";
import { fetchQuizStatus } from "../../api/interestQuiz";

/**
 * 她做完过兴趣测试没有。
 *
 * 🚨 三态，不是布尔。`null` = **还不知道**（请求在飞，或者失败了）。这个区别
 * 有实际后果：把「不知道」当成「没做过」，会在她每次打开树时闪一下那条邀请，
 * 而她上个月已经做过了；把「不知道」当成「做过了」，则是把入口藏起来。
 * 不知道的时候两个都不做。
 */
export function useQuizTaken(nonce = 0): boolean | null {
  const [taken, setTaken] = useState<boolean | null>(null);

  useEffect(() => {
    let alive = true;
    fetchQuizStatus()
      .then((s) => {
        if (alive) setTaken(s.taken);
      })
      .catch(() => {
        // 静默：一个读不到的测试状态不该在她的树上弹一条报错。入口照常显示
        // （见下面 TreeView 的用法：只有明确的 `true` 才收起那条邀请）。
        if (alive) setTaken(null);
      });
    return () => {
      alive = false;
    };
  }, [nonce]);

  return taken;
}
