import type { Reading, Writing } from "./types";

/**
 * 她读过的 / 她写的 — the library that feeds the tree and can be picked onto
 * her homepage.
 *
 * The prototype's student is 林知遥, 初二. Everything below is written as if
 * it really happened over one term, because the tree's whole claim is that it
 * grew out of real traces. Placeholder rows would make the tree a decoration.
 */

export const STUDENT = {
  name: "林知遥",
  handle: "zhiyao",
  grade: "初二",
  since: "2026-03",
};

export const READINGS: Reading[] = [
  {
    id: "r-coral",
    title: "红海北端那片不白化的珊瑚",
    source: "Science 科普编译",
    date: "2026-08-29",
    minutes: 22,
    takeaway: "一个例外能给希望，但它太小了——4 平方公里，不能拿来代表整片海。",
    quote: "我一开始觉得这是好消息，读到面积那一段才反应过来它有多小。",
    keywords: ["珊瑚", "例外与代表性", "气候"],
    field: "science",
    excerpt: [
      "红海北端的这片珊瑚在过去六年里经历了三次热浪。按照其他海域的经验，它们本该大面积白化——但它们没有。",
      "研究者把原因指向共生藻：这片珊瑚体内的虫黄藻属于一个耐热的分支，能在水温高出常年 2.4°C 时继续为宿主供能。",
      "然而这片珊瑚只有 4 平方公里，不到大堡礁的万分之二。它证明了「有可能」，没有证明「来得及」。",
      "论文最后一句写得很克制：一个避难所不是一个计划。",
    ],
  },
  {
    id: "r-sleep",
    title: "睡觉时大脑在重放什么",
    source: "Cell 摘要",
    date: "2026-08-25",
    minutes: 18,
    takeaway: "记住不是「用力记」，是「重放」。所以熬夜可能是在删掉自己白天的努力。",
    quote: "重放次数越多记得越牢——那我熬夜等于把重放的时间砍掉了。",
    keywords: ["记忆", "睡眠", "学习方法"],
    field: "science",
    excerpt: [
      "深睡期，小鼠海马体里白天走过的路径被压缩成几十毫秒的片段，一遍一遍地播放。",
      "研究者数了播放次数：重放得多的路径，第二天走得更准。",
      "这意味着记忆的巩固发生在你睡着以后，而不是你合上书之前。",
    ],
  },
  {
    id: "r-wait",
    title: "老师多等三秒会发生什么",
    source: "Educational Researcher",
    date: "2026-08-20",
    minutes: 14,
    takeaway: "沉默不是冷场，是给对方留的地方。我在小组讨论里抢话太快了。",
    quote: "0.9 秒到 3.5 秒——差的这 2.6 秒里，本来会有一个更完整的回答。",
    keywords: ["提问", "沉默", "对话"],
    field: "self",
    excerpt: [
      "1972 年，Mary Budd Rowe 发现老师提问后平均只等 0.9 秒就会自己接话。",
      "把这个等待时间拉长到 3.5 秒，学生的回答长度变成三倍，主动补充的次数上升，说「我不确定」的人也变多了。",
      "复现实验在五十年后得到了同样的结果。改变的成本是三秒。",
    ],
  },
  {
    id: "r-repair",
    title: "把螺丝钉重新设计一遍",
    source: "MIT Technology Review",
    date: "2026-08-18",
    minutes: 16,
    takeaway: "「修不好」很多时候是被设计出来的，不是技术做不到。",
    keywords: ["修理权", "设计伦理", "电子垃圾"],
    field: "making",
    excerpt: [
      "这款耳机只用三颗同规格螺丝，电池可拆，维修手册公开在官网。",
      "代价是售价高 12%，以及外壳比同类厚 1.8 毫米。",
      "一年后，31% 的首批用户自己换过至少一个零件。",
    ],
  },
  {
    id: "r-letters",
    title: "十万封普通人的家书",
    source: "British Library",
    date: "2026-08-12",
    minutes: 25,
    takeaway: "历史不只是大事，普通人的「你吃了吗」也是史料。",
    quote: "出现最多的词是钱、天气和你吃了吗——这三个词现在也一样。",
    keywords: ["档案", "普通人", "历史"],
    field: "humanities",
    excerpt: [
      "1850 到 1950 年，一百年间普通家庭之间的通信，十万封，全部扫描上线。",
      "词频统计的结果让整理者意外：最常出现的不是战争、不是政治，是钱、天气，和问对方吃了没有。",
      "档案馆写道：宏大的历史由这些琐碎的句子铺成。",
    ],
  },
  {
    id: "r-restore",
    title: "修了十一年，只补了 3%",
    source: "故宫博物院",
    date: "2026-08-05",
    minutes: 20,
    takeaway: "克制也是技术。知道什么时候停手，比会画更难。",
    keywords: ["修复", "克制", "手艺"],
    field: "arts",
    excerpt: [
      "十一年里，大部分时间用在检测：颜料层的成分、纸基的酸度、每一处起翘的走向。",
      "真正落笔补全的部分不到画面的百分之三，并且全部使用可逆材料——后人想去掉，随时可以。",
      "修复师说：我们的工作是让它多活两百年，不是让它看起来像新的。",
    ],
  },
  {
    id: "r-lonely",
    title: "青少年说孤独的时候",
    source: "JAMA Pediatrics",
    date: "2026-07-28",
    minutes: 15,
    takeaway: "孤独不等于没人在旁边，是没有一个能说真话的人。",
    quote: "我身边有很多人，但能说真话的可能只有一个半。",
    keywords: ["孤独", "友谊", "真话"],
    field: "self",
    excerpt: [
      "一万两千名 12 到 17 岁的少年填了同一份问卷。",
      "自评孤独感与社交频次几乎无关（r=0.08），与「是否有一个可以说真话的人」高度相关（r=0.61）。",
      "研究者建议，别再用「多出去走走」回应一个说自己孤独的孩子。",
    ],
  },
  {
    id: "r-gesture",
    title: "为盲人设计的手势",
    source: "ACM Interactions",
    date: "2026-07-20",
    minutes: 12,
    takeaway: "为最少数人做的设计，常常对所有人都更好。",
    keywords: ["无障碍", "设计", "溢出效应"],
    field: "making",
    excerpt: [
      "三指回退最初是为视障用户设计的：不需要瞄准，任何位置都能触发。",
      "两年后，明眼用户使用它的比例超过了系统自带的返回键。",
      "设计者称之为无障碍的溢出效应——路缘坡道最初是为轮椅修的，现在推婴儿车的人用得最多。",
    ],
  },
];

export const WRITINGS: Writing[] = [
  {
    id: "w-coral",
    title: "一片珊瑚不能代表一片海",
    date: "2026-08-30",
    words: 862,
    spine: "立场式（让步段收尾）",
    keywords: ["例外与代表性", "珊瑚", "论证"],
    field: "science",
    body: [
      "红海北端有一片珊瑚，在三次热浪里几乎没有白化。这个消息传开的时候，很多人松了一口气。我一开始也是。",
      "但我后来去查了它的面积：4 平方公里。大堡礁是 34.4 万平方公里。也就是说，这个「好消息」覆盖的范围，还不到我们担心的那片海的万分之二。",
      "我承认这片珊瑚很重要。它证明了耐热的共生藻是存在的，它给了研究者一个可以研究的样本，它甚至可能是未来移植的种源。这些都是真的。",
      "可是「有一个例外」和「问题解决了」之间，隔着一整个规模的问题。一个避难所不是一个计划。我们不能拿百分之零点零二的运气，去替百分之九十九点九八做决定。",
      "所以我的结论是：这条新闻值得高兴三秒钟，然后我们要问的第一个问题应该是——它能被复制吗？",
    ],
  },
  {
    id: "w-three-seconds",
    title: "我决定在小组讨论里闭嘴三秒",
    date: "2026-08-22",
    words: 640,
    spine: "记叙 + 反思",
    keywords: ["沉默", "对话", "自我观察"],
    field: "self",
    body: [
      "读到「老师多等三秒」那篇之后，我做了一件很别扭的事：在小组讨论里，别人说完，我强迫自己数到三再开口。",
      "第一次数到二的时候我就受不了了，觉得空气都是尴尬的。但第三次的时候，我数到二点五，林悦突然补了一句：「其实我刚才那个想法有问题。」",
      "那句话如果我抢在两秒的时候说话，就永远不会出现。",
      "我以前一直以为讨论是抢时间，谁先说谁就带节奏。现在我觉得，会留白的人才带节奏。",
    ],
  },
  {
    id: "w-letters",
    title: "如果一百年后只剩我的聊天记录",
    date: "2026-08-14",
    words: 710,
    spine: "钩子式",
    keywords: ["历史", "普通人", "记录"],
    field: "humanities",
    body: [
      "十万封家书里出现最多的三个词：钱、天气、你吃了吗。",
      "我翻了翻自己的聊天记录，前三名是：好、在吗、笑哭的表情。一百年后如果有人研究我，他会得出什么结论？",
      "但我想了想，那些家书在当时也不是为了留下来写的。它们只是当时的人在过日子。",
      "所以也许问题不是我的记录够不够体面，而是我有没有在真的过日子。",
    ],
  },
  {
    id: "w-repair",
    title: "被设计成修不好的东西",
    date: "2026-08-19",
    words: 580,
    spine: "起承转合",
    keywords: ["修理权", "设计伦理"],
    field: "making",
    body: [
      "我家有一个坏了的台灯，修灯的师傅看了一眼说：这个胶封死了，拆开就废了。",
      "我当时以为是技术限制。读完那篇讲螺丝钉的文章我才知道，那是一个选择——有人在图纸上决定了它修不好。",
      "换成三颗同规格的螺丝，成本会高一点，外壳会厚一点。但那盏灯本来可以再亮五年。",
    ],
  },
];

export function readingById(id: string): Reading | undefined {
  return READINGS.find((r) => r.id === id);
}

export function writingById(id: string): Writing | undefined {
  return WRITINGS.find((w) => w.id === id);
}
