/**
 * recommendations.ts — 今日推荐's seed data.
 *
 * WHAT THIS IS FOR (铁律②): a student who opens the reading room without an
 * article in hand has nothing to do. These four entries are the answer to
 * 「不知道读什么？」 — a short, FIXED shelf she can pick from. They are
 * deliberately NOT a feed: no paging, no "more like this", no ordering by
 * anything she did yesterday, no reason to come back and scroll. Four items,
 * the same four every time, and once she picks one the shelf is out of the
 * way.
 *
 * WHAT REPLACES IT: P2 turns this into a real sample library, and P4 adds the
 * teacher-assigned tasks that will sit ABOVE it. Both arrive as more entries
 * of the same shape, so nothing on the landing page has to change but the
 * source of the array.
 *
 * ON THE TEXTS: every 中文 piece here is written for this shelf — none of it
 * is quoted from an author, so nothing is misattributed. Each one is built
 * to reward the room's lenses: a mechanism, a concession that complicates it,
 * and a distinction that is easy to miss on one pass. The English entry is
 * the Gettysburg Address, verbatim (Bliss copy) and long out of copyright —
 * 272 words is the shortest complete argument in the canon.
 *
 * Blank lines are load-bearing: the server's `SplitBlocks` splits the body on
 * "\n\n", and each resulting block is what a tool card anchors to.
 */

/** Where the article comes from. `url` is unused by the seeds below but is
 *  the shape teacher-assigned tasks (P4) will arrive in, so the landing page
 *  already handles both. */
export type RecommendedSource = { kind: "text"; text: string } | { kind: "url"; url: string };

export interface RecommendedReading {
  id: string;
  title: string;
  /** One line, and only one: why THIS is worth an hour. Never a summary. */
  reason: string;
  /** What kind of reading it is — the only label on the card, because it is
   *  the only thing that changes how she should read it. */
  genre: string;
  lang: "zh" | "en";
  /** Macaron token name (see apps/web tokens) — the shelf reads as different
   *  books rather than four identical grey boxes. */
  tone: "peach" | "matcha" | "lake" | "taro";
  source: RecommendedSource;
}

const HEAT_ISLAND = `夏天的傍晚，如果你从市中心骑车往郊外走，会在某一段路上忽然觉得凉快下来。这不是错觉。气象学家把城市与周边乡村之间的温差叫作「城市热岛」，在很多大城市，这个温差能达到二到五摄氏度，而且夜里比白天更明显。

原因不止一个。柏油和混凝土在白天吸热、夜里慢慢放热，而泥土和植被会把一部分能量花在蒸发水分上，升温慢得多。城市里密集的高楼又把街道变成一条条「峡谷」，热辐射在墙面之间反复弹射，很难散出去。再加上空调、汽车、工厂本身就在往外排热——空调只是把室内的热搬到了室外，并没有让热消失。

于是有人提出：多种树、把屋顶刷白、修更多水面，就能给城市降温。这些做法确实有效，但效果有多大，取决于这座城市原本是什么样子。在一座本来就缺水的城市里种树，需要额外的灌溉用水；把屋顶刷白能反射夏天的阳光，却也可能让冬天的取暖多烧一些煤。

还有一点值得单独拎出来：热岛效应和全球变暖不是同一件事。热岛说的是同一时刻城市与乡村的差别，全球变暖说的是整个地球在几十年里的变化。把两者混为一谈，很容易推出「城市化导致了全球变暖」这种站不住脚的结论。但两者叠加起来时，城市居民真正承受的高温风险，确实被放大了。`;

const DELIVERY_FEE = `点一份三十元的外卖，配送费常常显示为四元。很多人默认这四元就是骑手的收入。事实要复杂一些。

平台向消费者收取的配送费，和平台支付给骑手的单价，是两笔分开算的钱。骑手拿到的通常是「基础单价 + 距离补贴 + 时段补贴」：在订单密集的午高峰可能高于四元，在雨天的偏远订单也可能更高。与此同时，平台还要从商家那一侧抽取一笔佣金，用来覆盖调度系统、地图服务、客服与保险等成本。

所以「配送费 = 骑手收入」这个等式，两头都不成立：消费者付的钱不全给骑手，骑手拿到的钱也不全来自这一栏。真正决定骑手收入的，是单位时间内能跑完多少单；而这又取决于算法怎么派单、路线顺不顺、商家出餐快不快——这些都不写在账单上。

讨论「外卖该不该涨价」的时候，如果只盯着账单上那一行数字，很容易把一个复杂的分配问题简化成一个道德问题。要判断谁在承担成本，得先问清楚：这笔钱经过了几只手，每一只手拿走的部分，各自对应着什么服务。`;

const MEMORY = `我们习惯把记忆想象成一盘录像带：事情发生的时候被录下来，回忆的时候被播放出来。心理学的研究却指出，记忆更像是每次都要重新搭一遍的积木。

上世纪七十年代，心理学家 Elizabeth Loftus 做过一组实验：让受试者看同一段车祸录像，然后分别问「两车相撞（hit）时车速多快」和「两车猛撞（smashed）时车速多快」。用了 smashed 这个词的那一组，估出来的车速更高；一周之后，还有更多人「记得」现场有碎玻璃——而录像里根本没有玻璃。

换句话说，提问本身会变成记忆的一部分。人回忆时并不是调出一段完整的录像，而是把残存的片段、常识、以及当下听到的说法拼接起来。拼完之后，主观上的确定感和真正的准确度并不相关：一个人可以非常肯定，同时非常错。

这件事对司法有直接的影响。目击者证词在法庭上分量很重，但如果讯问的措辞、指认的顺序都会改写记忆，那么「他记得清清楚楚」就不足以单独作为定罪的理由。它同样提醒我们，对自己那些反复讲过的往事，保留一点怀疑不是坏事——讲得越多，改写得也越多。`;

const GETTYSBURG = `Four score and seven years ago our fathers brought forth on this continent, a new nation, conceived in Liberty, and dedicated to the proposition that all men are created equal.

Now we are engaged in a great civil war, testing whether that nation, or any nation so conceived and so dedicated, can long endure. We are met on a great battle-field of that war. We have come to dedicate a portion of that field, as a final resting place for those who here gave their lives that that nation might live. It is altogether fitting and proper that we should do this.

But, in a larger sense, we can not dedicate — we can not consecrate — we can not hallow — this ground. The brave men, living and dead, who struggled here, have consecrated it, far above our poor power to add or detract. The world will little note, nor long remember what we say here, but it can never forget what they did here. It is for us the living, rather, to be dedicated here to the unfinished work which they who fought here have thus far so nobly advanced.

It is rather for us to be here dedicated to the great task remaining before us — that from these honored dead we take increased devotion to that cause for which they gave the last full measure of devotion — that we here highly resolve that these dead shall not have died in vain — that this nation, under God, shall have a new birth of freedom — and that government of the people, by the people, for the people, shall not perish from the earth.`;

export const RECOMMENDED_READINGS: RecommendedReading[] = [
  {
    id: "heat-island",
    title: "城市为什么比郊区热？",
    reason: "一个你天天在经历、却很少被解释清楚的温差。",
    genre: "科普",
    lang: "zh",
    tone: "lake",
    source: { kind: "text", text: HEAT_ISLAND },
  },
  {
    id: "delivery-fee",
    title: "一份外卖的配送费，到底付给了谁？",
    reason: "账单上那一行数字，藏着一整条分配链。",
    genre: "时事",
    lang: "zh",
    tone: "peach",
    source: { kind: "text", text: DELIVERY_FEE },
  },
  {
    id: "memory",
    title: "记忆不是一盘录像带",
    reason: "读完你会重新看待「我记得清清楚楚」这句话。",
    genre: "心理",
    lang: "zh",
    tone: "taro",
    source: { kind: "text", text: MEMORY },
  },
  {
    id: "gettysburg",
    title: "The Gettysburg Address",
    reason: "272 个词讲完一场战争的意义，最短的议论范本。",
    genre: "英文",
    lang: "en",
    tone: "matcha",
    source: { kind: "text", text: GETTYSBURG },
  },
];
