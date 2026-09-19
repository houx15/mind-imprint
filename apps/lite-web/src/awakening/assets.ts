/**
 * awakening/assets —— 这个房间的图和声音在哪里。
 *
 * # 为什么走公开 CDN，不走产品自己的对象存储
 *
 * 产品那套 OSS（`internal/oss`）是**私有桶 + 签名下载**，签出来的链接会过期。
 * 那对学生自己上传的东西是对的，对一张背景图没有意义：它不是她的数据，
 * 它是产品美术，应该被缓存，而且永远可读。
 *
 * 所以这些文件和营销站共用那个公开桶（`mind-open` / `mind-assets.uni-robot.cn`），
 * 但在 `awakening/v1/` 这个前缀下自成一块。上传用 `deploy/upload-site-assets.sh`：
 *
 *     SITE_ASSET_DIR=<一个含 awakening/v1/ 的目录> deploy/upload-site-assets.sh
 *
 * # 为什么路径写死在这里
 *
 * 一份清单，一个地方。文件名散在十四个组件里，改一次版本要改十四处，而漏掉的
 * 那一处会在生产上变成一个 404 —— 一张不显示的图，没有任何报错。
 *
 * # v1 是什么意思
 *
 * 换图不覆盖，换前缀。CDN 缓存一天，覆盖上去的新图在一天之内是随机新旧参半的；
 * 而 `awakening/v2/` 从第一秒起就是确定的。
 */

const BASE = "https://mind-assets.uni-robot.cn/awakening/v1";

/** 图。四张，全部来自参考设计里游戏真正引用的那几个文件。 */
export const IMAGES = {
  /** 历史档案那张教育警示图。三条证据说的就是这张图里的三个部分。 */
  archiveWarning: `${BASE}/education-warning.jpg`,
  /** 印记的形象。开场和选择助手那两屏用它。 */
  guide: `${BASE}/nova-assistant.png`,
  /** 档案那一屏的底。 */
  archiveBackdrop: `${BASE}/legacy-archive-background.svg`,
  /** 开场那一屏的底。 */
  wastelandBackdrop: `${BASE}/legacy-wasteland-background.svg`,
} as const;

/**
 * 语音。
 *
 * 文件名是 `{前缀}-{场景}.m4a`：前缀是印记助手（hot=NOVA / sage=SAGE /
 * dark=KIRO），场景是这一句在哪一屏说。参考设计按它当时那版七屏录的，
 * 五个场景在这一版里仍然各有落点：
 *
 *     quote      选择助手那一屏，它的自我介绍
 *     connect    终端接上的那一刻
 *     lens       三层追问
 *     challenge  下一步
 *     result     报告
 *
 * 另外两个不分助手：开场（system-intro-legacy）和提醒（system-warning）。
 *
 * 🚨 **默认不播。** 教室里默认外放是灾难。而且任何一屏都不等音频 ——
 * 播放失败、文件 404、浏览器拦截自动播放，都不该让她卡在那一屏。
 */
export type VoiceScene = "quote" | "connect" | "lens" | "challenge" | "result";

/** 印记助手代号 → 音频文件前缀。和 Go 侧 awakening.GuideProfile.AudioPrefix 一致。 */
const VOICE_PREFIX: Record<string, string> = {
  NOVA: "hot",
  SAGE: "sage",
  KIRO: "dark",
};

/** 某个助手在某一屏说的那一句。查不到助手时给 NOVA 的。 */
export function voiceUrl(guide: string, scene: VoiceScene): string {
  const prefix = VOICE_PREFIX[guide] ?? "hot";
  return `${BASE}/${prefix}-${scene}.m4a`;
}

/** 不分助手的两句。 */
export const SYSTEM_VOICE = {
  intro: `${BASE}/system-intro-legacy.m4a`,
  warning: `${BASE}/system-warning.m4a`,
} as const;

/** 她把声音打开过没有。只是一个方便，丢了不影响任何东西。 */
const MUTE_KEY = "awakening.muted";

export function isMuted(): boolean {
  try {
    // 默认静音：没有存过就是静音。
    return localStorage.getItem(MUTE_KEY) !== "off";
  } catch {
    // 无痕窗口、禁了站点数据、预览环境 —— 读不到就当静音。
    return true;
  }
}

export function setMuted(muted: boolean): void {
  try {
    localStorage.setItem(MUTE_KEY, muted ? "on" : "off");
  } catch {
    // 存不下就算了，这一趟仍然按她刚点的那个状态走。
  }
}
