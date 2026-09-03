import { createContext, useContext } from "react";

/**
 * 把工具拉出来看。
 *
 * 🚨 产品负责人 2026-09-03，看完线上的「审核助手」：「需要拉出来，更充分的
 * 视觉空间」。
 *
 * 右边那一栏是 360px。对便签板、问题识别这种"一次填一句"的工具够用，但审核
 * 助手要她**读一份文档**——正文、印记划出来的句子、几个要留意的方面，全挤在
 * 一条比手机还窄的柱子里，字号还和旁边的标签一样小。
 *
 * docs/2026-09-01-pbl-detail.md 对审核的第一条要求就是：
 *
 *	“A good review, the first requirement is a easy-to-read thing needs my review.”
 *
 * 读不下去，审就无从谈起——她只会一路划到底，点「审核通过」。
 *
 * 做成 context 而不是往下传 prop：十个工具面各自把 wide 接进 ToolFrame 是十处
 * 改动、十个可以忘掉的地方，而这件事跟工具本身没关系，是外壳的事。
 */
export interface WidePane {
  /** 现在是不是铺开的。 */
  wide: boolean;
  /** 切换。没有提供就是这个工具不支持铺开（比如手机上本来就是全屏）。 */
  toggle?: () => void;
}

const WidePaneContext = createContext<WidePane>({ wide: false });

export const WidePaneProvider = WidePaneContext.Provider;

export function useWidePane(): WidePane {
  return useContext(WidePaneContext);
}
