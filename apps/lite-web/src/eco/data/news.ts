import type { Bi, Domain, NewsItem } from "./types";

/**
 * 世界 · the news planets.
 *
 * FIVE items per day, ranked 1..5. Rank is EDITORIAL importance, not
 * engagement — nothing here counts clicks, and nothing in the UI rewards
 * coming back two days in a row (铁律②).
 *
 * 🚨 These items are WRITTEN FOR THE PROTOTYPE. They are plausible and
 * sourced-looking; they are not real reporting. `WorldView` renders a
 * permanent 「原型数据」chip because of this. When real headlines replace
 * them, remove the chip — not before.
 *
 * The political filter is a DATA-LEVEL fact, not a UI toggle: no item in this
 * file is about an election, a war, a party, or a government dispute. The
 * world view states the omission out loud (`POLITICS_NOTE`) rather than
 * quietly presenting a filtered set as if it were everything.
 */

export const DOMAIN_META: Record<Domain, { zh: string; en: string; hue: string; glyph: string }> = {
  tech: { zh: "科技", en: "Tech", hue: "var(--mk-mist)", glyph: "⌘" },
  science: { zh: "科学", en: "Science", hue: "var(--mk-lake)", glyph: "◎" },
  environment: { zh: "环境", en: "Environment", hue: "var(--mk-matcha)", glyph: "❋" },
  space: { zh: "太空", en: "Space", hue: "var(--mk-taro)", glyph: "✧" },
  health: { zh: "健康", en: "Health", hue: "var(--mk-berry)", glyph: "✚" },
  culture: { zh: "文化", en: "Culture", hue: "var(--mk-peach)", glyph: "✎" },
  economy: { zh: "经济", en: "Economy", hue: "var(--mk-butter)", glyph: "◈" },
  education: { zh: "教育", en: "Education", hue: "var(--mk-accent-300)", glyph: "▲" },
};

export const DOMAIN_ORDER: Domain[] = [
  "science",
  "tech",
  "environment",
  "space",
  "health",
  "culture",
  "economy",
  "education",
];

/** Why the political filter exists, in the product's own voice. Shown when
 *  the filter chip is opened — the omission is explained, never hidden. */
export const POLITICS_NOTE: Bi = {
  zh: "这里不放政治与冲突类的新闻。不是因为它们不重要——是因为讨论它们需要的东西（背景、立场、分寸）比一颗行星能给的多得多，我们不想让你在半分钟里对一件复杂的事下判断。世界很大，今天先看这五件。",
  en: "No politics or conflict here. Not because they don't matter — discussing them well needs more context, care and time than one planet can give, and we won't invite you to judge something complicated in thirty seconds. The world is wide; here are five other things today.",
};

/** How the five are chosen. Transparency chip in the world view. */
export const SELECTION_NOTE: Bi = {
  zh: "挑选标准：① 它改变了什么事实，而不只是有人说了什么；② 一个初中生读得懂它为什么重要；③ 它能牵出一个好问题；④ 五条之间尽量不同领域；⑤ 不含政治与冲突。",
  en: "How these five are picked: ① it changes a fact, not just who said what; ② a 13-year-old can see why it matters; ③ it opens a real question; ④ the five spread across different fields; ⑤ no politics or conflict.",
};

export const NEWS_DATES = ["2026-08-30", "2026-08-29", "2026-08-28", "2026-08-27", "2026-08-26"];

export const TODAY = NEWS_DATES[0]!;

export const NEWS: NewsItem[] = [
  // ── 2026-08-30 ────────────────────────────────────────────────────────────
  {
    id: "n-0830-1",
    date: "2026-08-30",
    rank: 1,
    domain: "environment",
    weight: 0.92,
    source: "Nature Sustainability",
    keywords: ["海洋碳汇", "气候", "测量与证据"],
    title: {
      zh: "海洋吸走的碳，比我们以为的少 7%",
      en: "The ocean is absorbing 7% less carbon than we thought",
    },
    summary: {
      zh: "一支国际团队重新校准了 40 年的海表测量数据，发现过去的模型高估了海洋的吸碳能力。差值不大，但它意味着留给陆地的减排任务比原计划更重。",
      en: "An international team recalibrated forty years of sea-surface measurements and found earlier models overstated how much carbon the ocean takes up. The gap is small — and it means land-based emission cuts must do more work than planned.",
    },
    hook: {
      zh: "如果一个用了四十年的数字被证明错了 7%，我们该重做多少已经做完的决定？",
      en: "If a number used for forty years turns out to be 7% wrong, how many finished decisions have to be reopened?",
    },
  },
  {
    id: "n-0830-2",
    date: "2026-08-30",
    rank: 2,
    domain: "science",
    weight: 0.85,
    source: "Cell",
    keywords: ["记忆", "睡眠", "大脑"],
    title: {
      zh: "睡觉时，大脑会把当天的记忆「重放」两百遍",
      en: "The sleeping brain replays the day about two hundred times",
    },
    summary: {
      zh: "研究者在小鼠海马体中记录到，白天走过的路径在深睡期被压缩成毫秒级的片段反复播放，播放次数越多，第二天记得越牢。",
      en: "Recording from the mouse hippocampus, researchers watched daytime routes replay in millisecond bursts during deep sleep. The more replays, the better the memory held the next day.",
    },
    hook: {
      zh: "如果记住一件事靠的是「重放」，那熬夜复习到底在做什么？",
      en: "If remembering depends on replay, what exactly does an all-nighter accomplish?",
    },
  },
  {
    id: "n-0830-3",
    date: "2026-08-30",
    rank: 3,
    domain: "tech",
    weight: 0.78,
    source: "IEEE Spectrum",
    keywords: ["能耗", "人工智能", "取舍"],
    title: {
      zh: "小模型跑赢大模型：一次关于「够用就好」的实验",
      en: "A small model beat a big one — an experiment in \"good enough\"",
    },
    summary: {
      zh: "在一项手写批改任务上，一个 30 亿参数的小模型准确率只比旗舰模型低 1.4%，能耗却只有它的 1/18。作者说，问题不在模型多强，而在任务多难。",
      en: "On a handwriting-grading task, a three-billion-parameter model scored just 1.4% below a flagship — using one eighteenth of the energy. The authors argue the question is not how strong the model is, but how hard the task is.",
    },
    hook: {
      zh: "「更强」和「刚好够」之间，你会为哪一个多付十八倍的电费？",
      en: "Between \"stronger\" and \"enough\", which one is worth eighteen times the electricity?",
    },
  },
  {
    id: "n-0830-4",
    date: "2026-08-30",
    rank: 4,
    domain: "culture",
    weight: 0.71,
    source: "The Paris Review",
    keywords: ["修改", "写作", "手稿"],
    title: {
      zh: "一位作家把三十年的删改稿公开了",
      en: "A novelist published thirty years of her deletions",
    },
    summary: {
      zh: "她把每一本书被删掉的段落整理成一册出版，理由是：读者只看见成品，会误以为好句子是一次写成的。这本「删掉的书」比原著厚。",
      en: "She collected the passages cut from every one of her novels into a single volume, because readers who see only finished work assume good sentences arrive whole. The book of deletions is thicker than the originals.",
    },
    hook: {
      zh: "如果把你删掉的部分也交上去，老师会看到一个更好的你，还是更差的你？",
      en: "If you handed in what you deleted, would your teacher see a better writer, or a worse one?",
    },
  },
  {
    id: "n-0830-5",
    date: "2026-08-30",
    rank: 5,
    domain: "space",
    weight: 0.66,
    source: "ESA",
    keywords: ["土卫二", "生命", "探测"],
    title: {
      zh: "土卫二的水柱里，找到了第七种有机分子",
      en: "A seventh organic molecule found in Enceladus's plume",
    },
    summary: {
      zh: "重新分析 2008 年的旧数据后，团队在这颗冰卫星喷出的水柱中确认了一种新的含氮有机物。它不是生命，但它是生命需要的原料之一。",
      en: "Re-analysing data from 2008, a team confirmed a new nitrogen-bearing organic compound in the icy moon's plume. It is not life — it is one of the ingredients life needs.",
    },
    hook: {
      zh: "一份十八年前的数据里还能挖出新发现——那「做实验」和「重读数据」，哪个更像科学？",
      en: "A new finding pulled from eighteen-year-old data — so which is more like science: running the experiment, or re-reading it?",
    },
  },

  // ── 2026-08-29 ────────────────────────────────────────────────────────────
  {
    id: "n-0829-1",
    date: "2026-08-29",
    rank: 1,
    domain: "health",
    weight: 0.89,
    source: "The Lancet",
    keywords: ["近视", "户外光照", "公共卫生"],
    title: {
      zh: "每天多两小时户外，八年后近视率降了一半",
      en: "Two more hours outdoors a day, half the myopia eight years on",
    },
    summary: {
      zh: "一项覆盖六个城市、跟踪八年的对照研究给出结论：起作用的不是「少看屏幕」，而是「多见自然光」。两组孩子的屏幕时间几乎一样。",
      en: "A six-city controlled study followed children for eight years and concluded the protective factor is daylight, not less screen time — both groups used screens about equally.",
    },
    hook: {
      zh: "当大人把问题归咎于手机，而数据指向阳光，谁该改变？",
      en: "When adults blame the phone but the data points at sunlight, who has to change?",
    },
  },
  {
    id: "n-0829-2",
    date: "2026-08-29",
    rank: 2,
    domain: "environment",
    weight: 0.83,
    source: "Science",
    keywords: ["珊瑚", "适应", "热浪"],
    title: {
      zh: "有一小片珊瑚，学会了在热浪里活下来",
      en: "One patch of coral has learned to survive the heatwaves",
    },
    summary: {
      zh: "红海北端的一片珊瑚在连续三次热浪中几乎没有白化。研究者认为它们的共生藻种类不同——但这片珊瑚只有 4 平方公里。",
      en: "A reef at the northern Red Sea barely bleached through three consecutive heatwaves; researchers point to a different symbiotic algae. The reef covers four square kilometres.",
    },
    hook: {
      zh: "一个例外，是希望，还是让我们放松警惕的理由？",
      en: "Is one exception a reason for hope, or a reason to relax too early?",
    },
  },
  {
    id: "n-0829-3",
    date: "2026-08-29",
    rank: 3,
    domain: "education",
    weight: 0.76,
    source: "Educational Researcher",
    keywords: ["提问", "课堂", "等待时间"],
    title: {
      zh: "老师多等三秒，学生的回答长度翻了三倍",
      en: "Teachers who wait three more seconds get answers three times longer",
    },
    summary: {
      zh: "研究复现了一个 1970 年代的经典发现：提问后把等待时间从 0.9 秒延长到 3.5 秒，学生的回答更长、更完整，敢说「我不确定」的人也更多。",
      en: "A replication of a 1970s classic: stretching the pause after a question from 0.9 to 3.5 seconds produced longer, fuller answers — and more students willing to say \"I'm not sure\".",
    },
    hook: {
      zh: "在你自己的对话里，你留给别人的沉默有多长？",
      en: "In your own conversations, how much silence do you leave the other person?",
    },
  },
  {
    id: "n-0829-4",
    date: "2026-08-29",
    rank: 4,
    domain: "tech",
    weight: 0.7,
    source: "MIT Technology Review",
    keywords: ["修理权", "设计", "电子垃圾"],
    title: {
      zh: "一家公司把螺丝钉重新设计了一遍，只为让人能修",
      en: "A company redesigned its screws so people could repair it",
    },
    summary: {
      zh: "新款耳机改用统一规格的三颗螺丝、可拆电池和公开的维修手册，售价高了 12%。首批用户里有 31% 在一年内自己更换过零件。",
      en: "New headphones use three identical screws, a removable battery and a public repair manual, at a 12% higher price. Within a year, 31% of early buyers had replaced a part themselves.",
    },
    hook: {
      zh: "你身边有哪件东西，是被「故意设计成修不好」的？",
      en: "What object near you was deliberately designed to be unfixable?",
    },
  },
  {
    id: "n-0829-5",
    date: "2026-08-29",
    rank: 5,
    domain: "culture",
    weight: 0.64,
    source: "British Library",
    keywords: ["档案", "普通人", "历史"],
    title: {
      zh: "十万封普通人的家书被扫描上线",
      en: "A hundred thousand ordinary letters go online",
    },
    summary: {
      zh: "档案馆公开了 1850–1950 年间普通家庭的通信。最常出现的词不是战争或政治，是「钱」「天气」和「你吃了吗」。",
      en: "An archive released a century of ordinary family correspondence. The most frequent words are not war or politics but money, weather, and \"have you eaten\".",
    },
    hook: {
      zh: "一百年后，如果只剩你的聊天记录，它会说你是个什么样的人？",
      en: "A century from now, if only your chat log survives, what kind of person would it say you were?",
    },
  },

  // ── 2026-08-28 ────────────────────────────────────────────────────────────
  {
    id: "n-0828-1",
    date: "2026-08-28",
    rank: 1,
    domain: "science",
    weight: 0.9,
    source: "Nature",
    keywords: ["抗生素", "耐药", "真菌"],
    title: {
      zh: "一种被放弃了六十年的化合物，突然有用了",
      en: "A compound abandoned sixty years ago suddenly works",
    },
    summary: {
      zh: "1960 年代因毒性被弃用的一种真菌代谢物，经过分子改造后对耐药菌有效。研究者说，最好的新药可能躺在旧的失败记录里。",
      en: "A fungal metabolite dropped in the 1960s for toxicity works against resistant bacteria after molecular redesign. The best new drugs, the authors suggest, may be lying in old failure reports.",
    },
    hook: {
      zh: "「失败」的记录被扔掉了多少？我们因此重复了多少次同样的路？",
      en: "How many failure records get thrown away — and how often do we walk the same road again because of it?",
    },
  },
  {
    id: "n-0828-2",
    date: "2026-08-28",
    rank: 2,
    domain: "economy",
    weight: 0.81,
    source: "OECD",
    keywords: ["四天工作制", "生产力", "试验"],
    title: {
      zh: "四天工作制试验三年后：产出没掉，离职率掉了",
      en: "Three years into the four-day week: output held, quitting fell",
    },
    summary: {
      zh: "追踪 61 家公司三年，营收平均持平，员工离职率下降 57%。但研究者提醒：能参加试验的公司本来就有余裕。",
      en: "Across 61 companies over three years, revenue stayed flat while staff turnover dropped 57%. The researchers caution that companies able to volunteer already had slack.",
    },
    hook: {
      zh: "一个实验只招到「本来就过得不错」的人，它还能证明什么？",
      en: "If only the already-comfortable can join an experiment, what can it still prove?",
    },
  },
  {
    id: "n-0828-3",
    date: "2026-08-28",
    rank: 3,
    domain: "space",
    weight: 0.75,
    source: "NASA JPL",
    keywords: ["火星", "水", "地下"],
    title: {
      zh: "火星地下探到一层「湿沙」",
      en: "A layer of damp sand found under Mars",
    },
    summary: {
      zh: "雷达信号显示，火星中纬度地下 1.5–2.6 公里处可能存在含液态水的多孔岩层。如果成立，那里的水量足以覆盖整个星球 1–2 米深。",
      en: "Radar suggests porous rock holding liquid water 1.5–2.6 km beneath the Martian mid-latitudes. If confirmed, it would be enough water to cover the planet one to two metres deep.",
    },
    hook: {
      zh: "「可能」和「发现」之间隔着什么？谁有资格把前者说成后者？",
      en: "What sits between \"possible\" and \"found\" — and who gets to call one the other?",
    },
  },
  {
    id: "n-0828-4",
    date: "2026-08-28",
    rank: 4,
    domain: "health",
    weight: 0.72,
    source: "JAMA Pediatrics",
    keywords: ["孤独", "青少年", "社交"],
    title: {
      zh: "青少年说「孤独」的时候，往往不是没人陪",
      en: "When teenagers say \"lonely\", they usually aren't alone",
    },
    summary: {
      zh: "一项对 1.2 万名 12–17 岁少年的调查发现，自评孤独与社交次数几乎无关，与「有没有一个能说真话的人」高度相关。",
      en: "A survey of 12,000 adolescents found self-reported loneliness barely tracks how often they socialise, but tracks strongly with having one person they can be honest with.",
    },
    hook: {
      zh: "你有几个能说真话的人？这个数字为什么这么难说出口？",
      en: "How many people can you be honest with? Why is that number so hard to say out loud?",
    },
  },
  {
    id: "n-0828-5",
    date: "2026-08-28",
    rank: 5,
    domain: "tech",
    weight: 0.63,
    source: "ACM",
    keywords: ["界面", "无障碍", "设计"],
    title: {
      zh: "为盲人设计的手势，被所有人用上了",
      en: "A gesture designed for blind users ended up used by everyone",
    },
    summary: {
      zh: "一项原本为视障用户设计的「三指回退」手势，在普通用户中的使用率超过了原生返回键。设计者称之为「无障碍的溢出效应」。",
      en: "A three-finger back gesture built for blind users overtook the standard back button among sighted users too — what its designers call the spillover effect of accessibility.",
    },
    hook: {
      zh: "为最少数人做的设计，为什么常常对所有人更好？",
      en: "Why does designing for the fewest people so often turn out better for everyone?",
    },
  },

  // ── 2026-08-27 ────────────────────────────────────────────────────────────
  {
    id: "n-0827-1",
    date: "2026-08-27",
    rank: 1,
    domain: "tech",
    weight: 0.88,
    source: "arXiv",
    keywords: ["模型", "引用", "核查"],
    title: {
      zh: "让模型给出处，错误率降了，但速度慢了四倍",
      en: "Made to cite sources, the model erred less — and ran four times slower",
    },
    summary: {
      zh: "强制模型为每个事实附上可点开的出处后，事实错误从 11% 降到 2.4%，但响应时间从 1.2 秒变成 5 秒。研究者问：用户愿意等吗？",
      en: "Forcing a citation behind every claim cut factual errors from 11% to 2.4% and stretched response time from 1.2 to 5 seconds. Would users wait, the authors ask.",
    },
    hook: {
      zh: "如果「更可靠」意味着「更慢」，你会为哪一个投票？",
      en: "If more reliable means slower, which one do you vote for?",
    },
  },
  {
    id: "n-0827-2",
    date: "2026-08-27",
    rank: 2,
    domain: "environment",
    weight: 0.82,
    source: "WWF",
    keywords: ["候鸟", "路线", "城市光"],
    title: {
      zh: "候鸟改了飞了三千年的路线",
      en: "Migratory birds changed a route three thousand years old",
    },
    summary: {
      zh: "卫星追踪显示，一支东亚鸻鹬类种群把停歇地整体北移了 340 公里，跟着湿地退化和城市灯光一起挪。",
      en: "Satellite tracking shows an East Asian shorebird population shifted its stopover site 340 km north, moving with wetland loss and city light.",
    },
    hook: {
      zh: "一群鸟改变了三千年的习惯——这是它们适应得好，还是我们逼得太急？",
      en: "A flock broke a three-thousand-year habit. Is that their adaptability, or our speed?",
    },
  },
  {
    id: "n-0827-3",
    date: "2026-08-27",
    rank: 3,
    domain: "culture",
    weight: 0.74,
    source: "故宫博物院",
    keywords: ["修复", "颜料", "耐心"],
    title: {
      zh: "一幅画修了十一年，修复师只补了 3% 的面积",
      en: "Eleven years of restoration touched 3% of the painting",
    },
    summary: {
      zh: "修复团队公开了全过程记录：绝大部分时间用在检测和等待，真正下笔的部分不到画面的百分之三，且全部可逆。",
      en: "The team published its full record: most of the eleven years went to analysis and waiting; under three percent of the surface was actually retouched, all of it reversible.",
    },
    hook: {
      zh: "「什么都不做」也是一种技术吗？你怎么判断该停手了？",
      en: "Can doing nothing be a skill? How do you know when to stop?",
    },
  },
  {
    id: "n-0827-4",
    date: "2026-08-27",
    rank: 4,
    domain: "science",
    weight: 0.69,
    source: "PNAS",
    keywords: ["蚂蚁", "集体", "决策"],
    title: {
      zh: "蚂蚁搬家时会「投票」，而且允许少数派拖延",
      en: "Ants vote when they move house — and let the minority stall",
    },
    summary: {
      zh: "实验发现，蚁群选新巢时并不是多数直接压过少数：少数派的持续反对会让整个决策延后，直到更多个体亲自去看过。",
      en: "Choosing a new nest, colonies do not simply let the majority win: persistent dissenters delay the decision until more individuals have gone and looked for themselves.",
    },
    hook: {
      zh: "一个允许「拖延」的决策机制，是低效，还是更聪明？",
      en: "A decision system that allows stalling — inefficient, or smarter?",
    },
  },
  {
    id: "n-0827-5",
    date: "2026-08-27",
    rank: 5,
    domain: "education",
    weight: 0.61,
    source: "PISA",
    keywords: ["阅读", "纸质", "屏幕"],
    title: {
      zh: "读长文章时，纸和屏幕的差距在「回头看」那一步",
      en: "For long texts, paper beats screen at one thing: going back",
    },
    summary: {
      zh: "对照实验显示，理解简单文本时纸与屏幕没有差别；文本一长、需要往回翻查时，纸质组的理解分高出 14%。",
      en: "On simple texts there is no difference. Once a text is long enough to require flipping back, the paper group scores 14% higher on comprehension.",
    },
    hook: {
      zh: "你上一次「往回翻」是什么时候？屏幕让你少做了这件事吗？",
      en: "When did you last flip back? Has the screen quietly stopped you doing it?",
    },
  },

  // ── 2026-08-26 ────────────────────────────────────────────────────────────
  {
    id: "n-0826-1",
    date: "2026-08-26",
    rank: 1,
    domain: "science",
    weight: 0.87,
    source: "Nature Neuroscience",
    keywords: ["语言", "双语", "大脑"],
    title: {
      zh: "同时说两种语言的孩子，大脑在「切换」上更省力",
      en: "Bilingual children spend less effort switching",
    },
    summary: {
      zh: "脑成像显示，双语儿童在任务切换时前额叶激活更低但成绩相同——不是更努力，是更省。这个差异在成年后变小。",
      en: "Imaging shows bilingual children activate the prefrontal cortex less while performing equally on task-switching — not harder, cheaper. The gap narrows in adulthood.",
    },
    hook: {
      zh: "「省力」和「厉害」是一回事吗？你怎么分辨这两者？",
      en: "Are \"effortless\" and \"good\" the same thing? How would you tell them apart?",
    },
  },
  {
    id: "n-0826-2",
    date: "2026-08-26",
    rank: 2,
    domain: "environment",
    weight: 0.8,
    source: "IEA",
    keywords: ["太阳能", "拐点", "成本"],
    title: {
      zh: "全球新增电力里，太阳能第一次超过一半",
      en: "Solar passes half of all new power for the first time",
    },
    summary: {
      zh: "2025 年全球新增发电装机中，太阳能占 51%。报告同时指出，电网和储能的建设速度只有装机速度的三分之一。",
      en: "Solar made up 51% of new generating capacity added worldwide in 2025. The same report notes grid and storage build-out is running at a third of that pace.",
    },
    hook: {
      zh: "发电容易了，「把电送到」却没跟上——瓶颈换了位置，我们的注意力换了吗？",
      en: "Generating got easy; delivering did not keep up. The bottleneck moved — did our attention?",
    },
  },
  {
    id: "n-0826-3",
    date: "2026-08-26",
    rank: 3,
    domain: "health",
    weight: 0.73,
    source: "BMJ",
    keywords: ["久坐", "运动", "剂量"],
    title: {
      zh: "每坐一小时起来走两分钟，效果好过晚上跑半小时",
      en: "Two minutes an hour beats half an hour at night",
    },
    summary: {
      zh: "在血糖控制这一项指标上，把运动拆散到全天的组别优于集中在晚上的组别，尽管两组总运动量相同。",
      en: "On blood-sugar control, spreading movement across the day outperformed the same total done in one evening block.",
    },
    hook: {
      zh: "同样多的努力，换个分布方式就不一样——你的学习是不是也这样？",
      en: "Same effort, different spacing, different result. Does your studying work the same way?",
    },
  },
  {
    id: "n-0826-4",
    date: "2026-08-26",
    rank: 4,
    domain: "space",
    weight: 0.68,
    source: "Astronomy & Astrophysics",
    keywords: ["星空", "光污染", "观测"],
    title: {
      zh: "全球夜空每年亮 9.6%，肉眼可见的星星在减少",
      en: "The night sky brightens 9.6% a year; visible stars are vanishing",
    },
    summary: {
      zh: "依靠五万名普通人上传的目视记录，研究者估算出夜空亮度的年增长率。一个出生在今天的孩子，十八岁时看到的星星只有出生时的四分之一。",
      en: "Built from 50,000 naked-eye reports by ordinary people, the study puts sky-brightening at 9.6% a year. A child born today will see a quarter as many stars at eighteen.",
    },
    hook: {
      zh: "五万个普通人的观察，能不能算科学数据？凭什么算？",
      en: "Fifty thousand ordinary observations — are those scientific data? On what grounds?",
    },
  },
  {
    id: "n-0826-5",
    date: "2026-08-26",
    rank: 5,
    domain: "economy",
    weight: 0.6,
    source: "Nikkei",
    keywords: ["二手", "消费", "耐用"],
    title: {
      zh: "二手交易额首次超过新品，在一个你想不到的品类里",
      en: "Second-hand overtakes new — in a category you wouldn't guess",
    },
    summary: {
      zh: "不是衣服，是童书。家长把「用三年就闲置」的东西优先转手，出版社开始按「可转手性」重新设计装帧。",
      en: "Not clothes — children's books. Parents resell what goes idle after three years, and publishers have begun designing bindings for resale.",
    },
    hook: {
      zh: "当「转手」变成设计目标，一件东西会长成什么样？",
      en: "When resale becomes a design goal, what shape does an object take?",
    },
  },
];

/** The five planets for a date, ranked. */
export function newsForDate(date: string): NewsItem[] {
  return NEWS.filter((n) => n.date === date).sort((a, b) => a.rank - b.rank);
}

export function newsById(id: string): NewsItem | undefined {
  return NEWS.find((n) => n.id === id);
}

/** 「8月30日 周日」 — the date rail's label. */
export function labelForDate(date: string, lang: Lang0 = "zh"): string {
  const d = new Date(`${date}T00:00:00`);
  const weekZh = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()];
  const weekEn = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][d.getDay()];
  return lang === "zh"
    ? `${d.getMonth() + 1}月${d.getDate()}日 ${weekZh}`
    : `${weekEn} ${d.getMonth() + 1}/${d.getDate()}`;
}

type Lang0 = "zh" | "en";
