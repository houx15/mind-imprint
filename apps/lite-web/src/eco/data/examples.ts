import type { PageExample } from "./types";

/**
 * 看看别人的家 — six real personal homepages, verified live (2026-08).
 *
 * These are REAL sites by REAL people. Two rules follow from that:
 *  1. The `good` bullets describe what the page actually does — no invented
 *     features, no fake quotes attributed to anyone.
 *  2. The thumbnails are ABSTRACT mocks (`shape` + `swatch`), never a
 *     screenshot or an imitation of someone's brand. We are pointing at their
 *     work, not reproducing it.
 *
 * Why exemplars come first in PBL#0: a 14-year-old asked to "make a personal
 * page" makes a résumé. Shown six pages that are obviously not résumés, she
 * makes something else. The teaching is in the annotation — 「你能偷走的那一招」
 * is one concrete structural move per site, not a compliment.
 */

export const PAGE_EXAMPLES: PageExample[] = [
  {
    id: "ex-victor",
    name: "Bret Victor",
    who: "界面研究者 · 他想让计算living在纸和墙上，而不是屏幕里",
    url: "worrydream.com",
    shape: "essay",
    swatch: ["#FFFFFF", "#33302E"],
    good: [
      "开头是一句承诺，不是头衔：「我把一生献给创造一种人性化的动态媒介。」",
      "整页是一个长列表，但按「想法」分组，不按「作品类型」——小标题本身就在告诉你他怎么思考。",
      "每个项目给一个年份 + 一句话；大项目给好几个入口（视频 / PDF / 海报），让不同耐心的人都进得去。",
      "零颜色、零装饰，黑字白底加几条横线。读起来像一个保存得很好的档案馆，不像一份自我推销。",
    ],
    steal: "先用一句话说出你着迷的东西，然后让作品列表去证明它。句子在前，证据在后。",
  },
  {
    id: "ex-appleton",
    name: "Maggie Appleton",
    who: "设计师 + 人类学背景 · 画「视觉长文」讲编程与文化",
    url: "maggieappleton.com",
    shape: "notebook",
    swatch: ["#FBF6EC", "#2F5D50"],
    good: [
      "标题一句话说清「形式 + 主题」：她做的是关于编程、设计、人类学的视觉长文。11 个词，你就知道会看到什么。",
      "她给内容分类，而且每一类都带定义：长文 =「有立场的叙事」，笔记 =「我还没完全搞懂的东西的松散记录」。",
      "「花园」里每篇都标了成熟度：种子 / 生长中 / 常青，还能筛选。没想清楚的东西是**故意**发出来的。",
      "所有插图都是她自己画的，视觉系统就是她的手，不需要另外做一套品牌。",
    ],
    steal: "标注每件东西的完成度。把半成品标成半成品，你就可以现在发布，而不是永远不发。",
  },
  {
    id: "ex-evans",
    name: "Julia Evans",
    who: "程序员 · 用手画的漫画小册子讲 DNS、git 这些难东西",
    url: "jvns.ca",
    shape: "minimal",
    swatch: ["#FFFFFF", "#C0392B"],
    good: [
      "像一个人在跟你打招呼：「嗨！我是 Julia。这是我的博客，我写过的每一篇都在这，按主题分好了。」",
      "首页就是全部存档——十篇最新，然后几百篇按 Git / DNS / 终端 / 职业分类。没有「阅读更多」的漏斗。",
      "导航里有一个「我最喜欢的」，她自己挑了约 40 篇打上星。第一次来的人不用赌运气。",
      "几乎没有样式：纯链接、日期、标题。它的本事是让一千篇文章依然找得到。",
    ],
    steal: "自己挑出最好的五件，指给别人看。访客只给你 30 秒，别让他们自己挖。",
  },
  {
    id: "ex-case",
    name: "Nicky Case",
    who: "独立作者 · 做能玩的解释器：用小游戏讲信任、隔离、焦虑",
    url: "ncase.me",
    shape: "playful",
    swatch: ["#FFF8E6", "#E8590C"],
    good: [
      "一口气说完自己：「嗨，我是 Nicky！我给好奇又爱玩的人做东西。」",
      "作品不按类型分，按**你能拿它干什么**分：可以玩的 / 可以读的 / 可以看的。这是动词菜单，它顺便告诉你该怎么花时间。",
      "每个作品是一张卡：缩略图 + 标题 + 一句话，点进去直接就能玩。作品本身就是作品集，没有另做的案例介绍。",
      "语气从头到尾不出戏，而且把「公共领域授权」写在首页——那是一个被展示出来的价值观。",
    ],
    steal: "把东西给人看，别描述它。能点开就直接体验的作品，胜过任何一段介绍它的话。",
  },
  {
    id: "ex-sloan",
    name: "Robin Sloan",
    who: "小说家 · 也做橄榄油。写了很多年的邮件通讯",
    url: "robinsloan.com",
    shape: "grid",
    swatch: ["#F7F3EA", "#1F4B99"],
    good: [
      "他不挑现成的头衔，自己造了一个：「创意实业家」。因为没人是这个，所以你记得住。",
      "导航只有三项：首页、关于、《Moonbound》。他最想让你看的那一个，单独占了一格。",
      "完整目录放在**页面底部**——基础、存档、长篇、中篇、特别项目、短篇、值得一读的随笔。顶部保持安静，同时什么都没藏。",
      "细节在做人格化的事：他给自己的最爱打星，页面上还显示他小说在图书馆的实时借阅排队人数。一个怪细节比一段自我介绍说得多。",
    ],
    steal: "别在现成的分类里挑一个词。给自己造一个标签——它比「作家 & 创业者」更像你。",
  },
  {
    id: "ex-rsms",
    name: "Rasmus Andersson",
    who: "设计师 / 工程师 · Inter 字体的作者",
    url: "rsms.me",
    shape: "terminal",
    swatch: ["#111111", "#EEEEEE"],
    good: [
      "两句话介绍，第二句是一个信条：「软件是我表达自己的媒介。」",
      "导航是五个单词：关于、项目、工作、摄影、商店。",
      "首页只放精选，「查看全部」后面才是真正的档案（75 个项目、512 篇文章）。首页是高光，深度只隔一次点击。",
      "一种字体、没有图片、没有动画、没有渐变。唯一的装饰是一行 Inter 的字样样张——他设计的那款字体自己在说话。",
    ],
    steal: "少即是自信。一种字体、一个主色、四五个导航词。页面安静，作品才响亮。",
  },
];

/** The synthesis. Shown after the gallery, as the thing she is meant to keep. */
export const PAGE_PRINCIPLES: { n: string; title: string; body: string }[] = [
  {
    n: "01",
    title: "用一句话说清楚：你是谁，你着迷于什么",
    body: "不是头衔，是一个承诺。「我是初二学生」谁都能写；「我想知道普通人的日常怎么变成历史」只有你会写。",
  },
  {
    n: "02",
    title: "让人直接点开你的作品，而不是读你对作品的介绍",
    body: "能读的就放全文，能玩的就放进去玩。一段「我做了一个关于……的项目」抵不上一个能点开的东西。",
  },
  {
    n: "03",
    title: "你自己挑出最好的五件，放在最显眼的地方",
    body: "访客只给你 30 秒。你不挑，他们就随便点一个，然后以为那就是你的水平。",
  },
  {
    n: "04",
    title: "标注它的完成度",
    body: "种子 / 生长中 / 完成。说清楚它是半成品，你就可以现在发布——而不是永远等着它完美。",
  },
  {
    n: "05",
    title: "少一点：一种字体、一个主色、四五个导航词",
    body: "页面安静，作品才响亮。加东西很容易，删东西才需要判断。",
  },
];

/** The one thing every good page has, said plainly. */
export const WEIRD_DETAIL_NOTE =
  "还有一件事：上面六个页面，每一个都有一个**奇怪的私人细节**——Sloan 显示自己小说在图书馆的排队人数，Case 从头到尾管作品叫「shtuff」，Victor 用一句「你永远不知道谁还醒着」结尾。正是那个细节，让页面变成「某个人的页面」，而不是一个模板。你的那个细节是什么？";
