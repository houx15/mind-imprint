import type { ArtifactSpec } from "./types";

/**
 * 印记 做出来的东西 — the other half of the loop.
 *
 * A 工具卡 is work SHE does. An artifact is work **印记 does** and hands over
 * for judgement. In a project 印记 may genuinely build — write the code, lay
 * out the page, generate the options, assemble a draft from her own library —
 * and that is the revision to 铁律② the product owner asked for. What it may
 * never do is decide, so every artifact charges her exactly three things:
 *
 *   ① read it, ② decide about it, ③ **say why**.
 *
 * 🚨 No artifact settles without a `why`. That is the whole design; the
 * moment a 「就用这个」 button works on its own, this becomes a machine that
 * generates and a student who approves, which is the thing we are trying not
 * to build.
 *
 * ## Two habits every `note` here keeps
 *  - **It names what 印记 guessed.** A handover that hides its assumptions
 *    cannot be reviewed, only accepted.
 *  - **It admits what is wrong with its own work** (`admits` on a build
 *    round). An AI that only reports success teaches a student to stop
 *    looking.
 */
export const ARTIFACTS: Record<string, ArtifactSpec> = {
  /* ── 个人主页 ─────────────────────────────────────────────────────── */
  "hp-style": {
    id: "hp-style",
    kind: "options",
    title: "三个方案",
    payoff:
      "你选的这版会变成我后面所有工作的约束。你写的理由更要紧：我做出来的东西你觉得不对的时候，那句理由就是你说「不对」的依据。",
    note:
      "按你写的三个词，和你收的那四个参考，我做了三版。\n\n先说我猜的地方：你写了「安静」，但你挑的参考里有两个信息密度很高——这两件事是有张力的，所以 A 和 B 是往两个方向拉的。C 是我自己想试的一版，你可以直接否掉。",
    ask: "挑一版，然后写清楚你为什么不要另外两版。理由写得越具体，我后面每一步就越不用问你。",
    steps: [
      "在读你的三个词……",
      "在读你收的四个参考，找它们的共同点……",
      "在拉三个不同方向的版式……",
      "在检查这三版是不是真的不一样……",
    ],
    options: [
      {
        id: "A",
        name: "一句话开场",
        tag: "长页 · 无导航 · 很空",
        bullets: [
          "第一屏只有一句话：你是谁、你在想什么。",
          "往下滚是三件你做过的事，每件配一句「我为什么做它」。",
          "没有导航栏，因为只有一页。",
        ],
        paper: "#FBF8F4",
        ink: "#33302E",
        accent: "#EA5140",
        font: '"Noto Serif SC","Songti SC",Georgia,serif',
      },
      {
        id: "B",
        name: "索引式",
        tag: "密 · 像一份档案柜",
        bullets: [
          "首页是一张列表：日期 + 标题 + 一句话，一屏看到十几条。",
          "顶部一行小字说明这里是什么。",
          "点进去才是正文。密度优先。",
        ],
        paper: "#14120F",
        ink: "#E8E2D8",
        accent: "#6FBFB0",
        font: 'ui-monospace,"SF Mono","PingFang SC",monospace',
      },
      {
        id: "C",
        name: "一个作品打头",
        tag: "图先行 · 强对比",
        bullets: [
          "开头直接是你最好的一件作品，占满一屏，不解释。",
          "往下才是「这是谁做的」。",
          "其余作品做成小图排在最后。",
        ],
        paper: "#FFFFFF",
        ink: "#1A1A1A",
        accent: "#E0A63A",
        font: '-apple-system,"PingFang SC","Helvetica Neue",sans-serif',
      },
    ],
  },

  "hp-content": {
    id: "hp-content",
    kind: "draft",
    title: "内容草稿",
    payoff:
      "你改过的句子我会原样用，一个字不动。没改的那几句会以我的口气出现在你的主页上——别人读到的就是我。",
    note:
      "我从你读过、写过、做过的东西里拟了一版。\n\n三件事先说清楚：这些句子是我写的，不是你写的；「一个关于我的怪细节」那一块我完全是编的，因为你的库里没有；我按「时间最近」排的顺序，不是按「你最喜欢」——那个顺序只有你知道。",
    ask: "把不像你的话改掉。改过的地方越多越好——这一页上留着我的句子，别人读到的就是我，不是你。",
    steps: [
      "在翻你读过的 12 篇……",
      "在翻你写过的 4 篇……",
      "在找它们之间反复出现的那条线……",
      "在把每一块写成一段草稿……",
    ],
    blocks: [
      {
        id: "intro",
        label: "一句话介绍",
        hint: "这句是我按你的关键词拼的。它大概率不像你说话。",
        text: "我是知遥。我对「东西为什么会被扔掉」这件事着迷，从一盏修不好的台灯开始。",
      },
      {
        id: "question",
        label: "我在乎的问题",
        hint: "这是你的树上最粗的那条线。措辞是我的。",
        text: "一件还能修的东西，是谁决定它该被扔的？",
      },
      {
        id: "works",
        label: "我做过的",
        hint: "从你的项目里取的。「学到什么」那句是我替你总结的。",
        text: "让爷爷看得清的药盒 —— 做了三版，最后一版最丑但他在用。学到的是：给别人做的东西，好不好看由我说了不算。",
      },
      {
        id: "writes",
        label: "我写的",
        hint: "我按最近排的。你可能更想把另一篇放前面。",
        text: "《修不好的台灯》——一篇关于「修理权」的短文，我在里面第一次用了让步段。",
      },
      {
        id: "detail",
        label: "一个关于我的怪细节",
        hint: "🚨 这一条是我编的。你的库里没有任何依据。要么换成真的，要么删掉。",
        text: "我留着每一个我拆开过的东西的螺丝。",
      },
    ],
  },

  "hp-build": {
    id: "hp-build",
    kind: "build",
    title: "印记开工",
    payoff:
      "这一版验收完就会变成一个真的网址。你之后发布的每个项目也会长在这一页上，所以这一页的样子会跟着你很久。",
    note:
      "这一步我要做久一点：按你选的方案和你改过的内容，把整一页写出来。\n\n做完你在手机和电脑上都看一眼，然后告诉我哪里不对。说得越具体我改得越准——「第二屏的字太小」我能改，「感觉怪怪的」我只能瞎猜。",
    ask: "看完给一条具体的反馈，或者说它可以了。",
    steps: [
      "在按你选的那一版搭骨架……",
      "在把你改过的五段内容排进去……",
      "在处理手机上的排版……",
      "在检查在深色模式下还读不读得清……",
      "在压缩图片、生成网址……",
    ],
    rounds: [
      {
        changed: "第一版。按你选的那一版，配你改过的内容。",
        admits: [
          "手机上第一屏那句话会断成三行，我还没调好。",
          "药盒那一条我没有图，现在看起来有点空。",
        ],
      },
      {
        changed: "按你说的，第一屏那句话在手机上不断行了；药盒那条加了图位。",
        admits: ["图位还是空的——那张照片得你拍，我没有。"],
      },
      {
        changed: "换上了你拍的照片，联系方式加在了最后。",
        admits: ["没有别的了。剩下的取决于你还想往上放什么。"],
      },
    ],
  },

  /* ── 社区花园地图 ────────────────────────────────────────────────── */
  "gd-points": {
    id: "gd-points",
    kind: "options",
    title: "三种密度",
    payoff:
      "这个数字直接决定你要走多少趟、印多少张码、以后每两周补几张。它也决定了我在页面上画几个点。",
    note:
      "按你画的图和你标的六个迷路点，我算了三种。\n\n我猜的地方：我假设你能维护的范围是「自己一个人，每两周去看一次」。如果你能拉到人一起，第三种才有可能。",
    ask: "挑一个数字，写清楚你为什么选它。这个数字后面会决定你要印多少张码、走多少趟。",
    steps: [
      "在读你画的那张图……",
      "在把你标的六个迷路点放上去……",
      "在算「站在一个点能不能看见下一个」……",
      "在拉三种密度并算各自的维护量……",
    ],
    options: [
      {
        id: "few",
        name: "6 个点",
        tag: "只盖你标的迷路点",
        bullets: [
          "只在你真的犹豫过的六个位置贴。",
          "好处：一周之内你能全部做完，而且维护得起。",
          "坏处：走在两点之间的人还是会心虚——他看不见下一个。",
        ],
        paper: "#F4F7F0",
        ink: "#2F3B2C",
        accent: "#5FA97E",
      },
      {
        id: "mid",
        name: "12 个点",
        tag: "每个岔路口都有",
        bullets: [
          "所有岔路口 + 三个门 + 中心亭。",
          "好处：基本上走到哪儿都能定位。",
          "坏处：十二张码，你大概每两周要补一张。",
        ],
        paper: "#FBF8F4",
        ink: "#33302E",
        accent: "#EA5140",
      },
      {
        id: "many",
        name: "20 个点",
        tag: "连直路中段也有",
        bullets: [
          "岔路口之外，长直路的中间也放。",
          "好处：任何位置抬头都能看见一个。",
          "坏处：二十张码一个人维护不过来，而且物业更可能不批。",
        ],
        paper: "#FFFFFF",
        ink: "#1A1A1A",
        accent: "#E0A63A",
      },
    ],
  },

  "gd-build": {
    id: "gd-build",
    kind: "build",
    title: "印记做这个网页",
    payoff:
      "这一版验收完，二维码就会印出来贴到园子里，被不认识路的人扫。你在这儿放过去的问题，会变成一个人站在树底下的困惑。",
    note:
      "我按你的图和你定的点数写这一页：扫任意一个码，打开就是「你在 7 号点」，下面才是整张图。\n\n我猜了两件事：我假设扫码的人多数在走路，所以第一屏只放一件事；我把你手画的图重画成了矢量图，路的形状可能和你画的有出入——这个你得核。",
    ask: "在手机上真的扫一次，然后告诉我哪里不对。",
    steps: [
      "在把你手画的图转成矢量图……",
      "在给六个点编号并生成对应的链接……",
      "在写「你在这里」那一屏……",
      "在处理手机上的显示……",
      "在生成六张二维码……",
    ],
    rounds: [
      {
        changed: "第一版。六个点，扫码进来先说你在哪儿。",
        headline: "你在 3 号点 · 东三门进来第二个岔路口",
        lines: [
          "往左：中心亭，约 80 米",
          "往右：5 号楼 – 8 号楼，约 140 米",
          "———— 整张图 ————",
          "[图] 六个编号点 · 三个门 · 中心亭",
        ],
        admits: [
          "「约 80 米」是我按你图上的比例估的，我没量过。这个数你得去核。",
          "图在小屏上要横过来看才清楚，我还没做竖屏版。",
        ],
      },
      {
        changed: "距离改成了你实测的步数；图加了竖屏版。",
        headline: "你在 3 号点 · 东三门进来第二个岔路口",
        lines: [
          "往左：中心亭，约 110 步",
          "往右：5 号楼 – 8 号楼，约 190 步",
          "———— 整张图 ————",
          "[图·竖屏] 六个编号点 · 三个门 · 中心亭",
        ],
        admits: ["用步数不用米，是你的主意，比我原来的好——步数不用估。"],
      },
      {
        changed: "按物业的条件改了：码贴在树牌上，页面底部加了「本页由住户制作」。",
        headline: "你在 3 号点 · 东三门进来第二个岔路口",
        lines: [
          "往左：中心亭，约 110 步",
          "往右：5 号楼 – 8 号楼，约 190 步",
          "———— 整张图 ————",
          "[图·竖屏] 六个编号点 · 三个门 · 中心亭",
          "本页由住户制作 · 有错请告诉我",
        ],
        admits: ["没有别的了。剩下的是它在真实的雨里能撑多久，那个只有时间知道。"],
      },
    ],
  },

  /**
   * 问住户 — the form step.
   *
   * 印记 writes five questions and then **marks its own two bad ones**. That
   * move is the whole lesson: a draft that arrives already flagged turns the
   * review from politeness into work, and it teaches the named failure modes
   * (双重问题 / 引导性问法 — see `METHODS`, from Pew) on a question she is
   * about to send to someone she knows.
   *
   * The invitation is hers to write. 印记 drafts one and says outright that it
   * should not be sent as written: a message in the AI's voice, under her
   * name, to her neighbours, is the one place in this project where 印记
   * writing for her would be a lie about who is asking.
   */
  "gd-ask": {
    id: "gd-ask",
    kind: "form",
    title: "问住户",
    payoff:
      "这份问卷会真的发进业主群，真的有人填。收回来的答案会直接决定你把点定在哪儿——你多留一道坏题，就会多收一堆没法用的数据，而数据收完就改不了了。",
    note:
      "按你踏勘时标的那几个位置，我拟了五道题。\n\n**其中两道我写得有问题**，我在下面标出来了——一道在问两件事，一道已经把答案塞给对方了。你可以改掉，也可以直接删掉，但别原样发出去。\n\n我猜的地方：我假设填的人在手机上填，所以每道题都短。",
    ask: "把我标的那两道处理掉，再写一句你自己的邀请。然后它就真的发出去了。",
    steps: [
      "在读你标的六个迷路点……",
      "在按每个点拟一道题……",
      "在检查这几道题有没有双重、有没有引导……",
      "在给自己找出来的毛病做标记……",
    ],
    form: {
      audience: "小区业主群 + 东三门的门卫",
      questions: [
        {
          id: "q1",
          q: "你在花园里迷过路吗？",
          kind: "choice",
          options: ["迷过", "没有", "帮别人指过路"],
        },
        {
          id: "q2",
          q: "你觉得花园的路标不清楚、树也挡视线，对吗？",
          kind: "choice",
          options: ["对", "不对"],
          flag: "这道我写坏了。它把答案塞给了对方——「对吗」这三个字会让人顺着点「对」。而且它其实在问两件事：路标和树。",
          fault: "引导性问法",
        },
        {
          id: "q3",
          q: "你一般是在哪个位置停下来找方向的？",
          kind: "text",
        },
        {
          id: "q4",
          q: "你觉得装指示牌好还是做一个手机上的地图更好，为什么？",
          kind: "text",
          flag: "这道也有毛病：它在问两件事（哪个更好 + 为什么），而且它假设填的人知道这两个东西分别是什么。多数人会只答前半句。",
          fault: "双重问题",
        },
        {
          id: "q5",
          q: "上一次你在花园里找不到路，是什么时候？",
          kind: "text",
        },
      ],
      inviteDraft:
        "您好！我们正在做一个关于社区花园导航的调研，希望能占用您两分钟时间填写这份问卷，您的反馈对我们非常重要，感谢您的支持！",
      replies: [
        {
          id: "r1",
          who: "3 号楼 · 一位住了两年的住户",
          answers: [
            { qId: "q1", a: "帮别人指过路" },
            { qId: "q3", a: "东三门进来第二个岔路口。我每次都要停一下想一想。" },
            { qId: "q5", a: "上周。有人问我 5 号楼怎么走，我自己指错了。" },
          ],
        },
        {
          id: "r2",
          who: "东三门 · 门卫师傅",
          answers: [
            { qId: "q1", a: "帮别人指过路" },
            { qId: "q3", a: "都是在我这儿问完，走到中心亭那儿又回来问第二遍。" },
            { qId: "q5", a: "天天有。周末最多，一下午能有七八个。" },
          ],
        },
        {
          id: "r3",
          who: "8 号楼 · 刚搬来一个月",
          answers: [
            { qId: "q1", a: "迷过" },
            { qId: "q3", a: "从中心亭往回走的时候。三条路长得一模一样。" },
            { qId: "q5", a: "前天晚上，走了二十分钟才出去。" },
          ],
        },
        {
          id: "r4",
          who: "12 号楼 · 一位老人",
          answers: [
            { qId: "q1", a: "没有" },
            { qId: "q3", a: "我住了十几年了，闭着眼睛也走得出去。" },
            { qId: "q5", a: "没有过。" },
          ],
        },
      ],
      readout: [
        "4 个人填了。3 个说迷过或替别人指过路，1 个说从来没有——那位住了十几年。",
        "「你在哪儿停下来找方向」这道题，有 3 个人写到了同一个地方：**东三门进来第二个岔路口**，和你自己标的第一个点重合。",
        "门卫师傅那条是你没料到的：他说人在他那儿问完，走到中心亭又回来问第二遍。这说明问题不只在门口，还在中间那一段。",
        "🚨 我要说清楚一件事：4 份不足以说「大家都」。它够你定第一版点位，不够写进任何一句带比例的结论。",
      ],
    },
    rounds: [],
  },

  "gs-signs": {
    id: "gs-signs",
    kind: "options",
    title: "三种标法",
    payoff:
      "你选的这种标法会做成实物装上去。装上去之后改一次的成本大概是改一个网页的五十倍，所以这一次的判断比后面很多次都贵。",
    note:
      "同样的六个位置，三种写法。它们教给路人的东西完全不一样。\n\n我猜的地方：我假设看牌子的人是第一次来。如果你更在乎的是住户，第二种的价值会高很多。",
    ask: "挑一种，写清楚你为什么。这一次的理由要回答一个问题：你想解决「这一次」，还是「以后」。",
    steps: [
      "在读你标的六个迷路点……",
      "在把三种写法各排一版……",
      "在检查在两米外还看不看得清……",
    ],
    options: [
      {
        id: "num",
        name: "只写编号",
        tag: "最简单 · 配合电话用",
        bullets: [
          "牌子上只有一个大数字。",
          "好处：做起来最快，两米外也看得清。",
          "坏处：人知道自己在哪，但不知道往哪走。",
        ],
        paper: "#14120F",
        ink: "#E8E2D8",
        accent: "#6FBFB0",
      },
      {
        id: "place",
        name: "写地名",
        tag: "教人认识这个园子",
        bullets: [
          "「海棠道」「中心亭」这样的名字。",
          "好处：来第二次的人开始有方向感。",
          "坏处：第一次来的人还是不知道该往哪走。",
        ],
        paper: "#FBF8F4",
        ink: "#33302E",
        accent: "#EA5140",
      },
      {
        id: "arrow",
        name: "箭头 + 距离",
        tag: "这一次一定找得到",
        bullets: [
          "「5–8 号楼 →  190 步」。",
          "好处：解决问题最直接。",
          "坏处：走过一百次的人，还是不认识这个园子。",
        ],
        paper: "#F4F7F0",
        ink: "#2F3B2C",
        accent: "#5FA97E",
      },
    ],
  },
};

export function artifactById(id: string): ArtifactSpec | undefined {
  return ARTIFACTS[id];
}
