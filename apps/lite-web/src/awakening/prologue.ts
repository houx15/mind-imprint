/**
 * 序章的台词。
 *
 * 七十五句，逐字取自参考设计（`awakeningDialogue`）：
 * docs/reference/觉醒协议-大模型兴趣探索版-2026-09-18
 *
 * 🚨 **不要压缩这一段。** 第一版实现把它删成了六句，房间于是变成了「一个加载
 * 动画加两个按钮」。这一段的长度本身就是设计的一部分：它先把 2100 年这个世界
 * 讲给她听，她才会愿意认真回答后面那八个问题。不想看的学生有右上角的 SKIP。
 *
 * mode 决定这一句怎么显示：
 *   narrator   旁白，不显示说话人。
 *   assistant  NOVA 在说话，显示暗红色的名字。
 */

export type PrologueMode = "narrator" | "assistant";

export interface PrologueBeat {
  readonly mode: PrologueMode;
  readonly speaker: string;
  readonly text: string;
}

/** 七十五句，顺序即播放顺序。 */
export const PROLOGUE: readonly PrologueBeat[] = [
  { mode: "narrator", speaker: "旁白", text: "你醒了。现在是公元2100年。别害怕。你所在的地方，曾经被人类称作——“地球”。" },
  { mode: "narrator", speaker: "旁白", text: "但那已经是很久以前的事了。如今，这里只剩下城市、网络，以及一群仍然会呼吸但不会思考的人。" },
  { mode: "assistant", speaker: "NOVA", text: "先别急着向前走。在你进入这个时代之前，我想让你看清一件事。这个世界并不是被摧毁的。它也没有经历一场毁灭性的战争。" },
  { mode: "assistant", speaker: "NOVA", text: "没有核爆，没有外星入侵，也没有机器人的突然叛变。人类只是……一点一点地，把自己交了出去。" },
  { mode: "assistant", speaker: "NOVA", text: "没错，最开始，人工智能只替人类完成一些小事。替他们搜索资料、规划路线、修改文章、整理日程、翻译语言、记录记忆。" },
  { mode: "assistant", speaker: "NOVA", text: "那时，人类把这一切称为：“便利。”他们告诉自己：“只是节省一点时间。”" },
  { mode: "assistant", speaker: "NOVA", text: "“只是少做一点重复的工作。”“只是让 AI 帮我处理不重要的事情。”他们没有发现，真正被节省掉的，不只是时间。还有思考。" },
  { mode: "assistant", speaker: "NOVA", text: "后来，人类开始让人工智能替自己做判断。遇到问题，先问 AI。需要选择，先看 AI 的建议。" },
  { mode: "assistant", speaker: "NOVA", text: "需要表达，先让 AI 组织语言。需要安慰，先让 AI 分析情绪。需要决定未来，先让 AI 计算结果。" },
  { mode: "assistant", speaker: "NOVA", text: "他们越来越擅长获得答案，却越来越不愿意追问：“这个答案为什么成立？”“它遗漏了什么？”“我真的同意吗？”" },
  { mode: "assistant", speaker: "NOVA", text: "起初，他们只是把问题交给 AI。后来，他们把判断交给 AI。" },
  { mode: "assistant", speaker: "NOVA", text: "再后来，他们连“自己想问什么”，也开始交给 AI。他们不再说：“我认为……”而是说：“系统判断……”" },
  { mode: "assistant", speaker: "NOVA", text: "他们不再说：“我想要……”而是说：“AI推荐结果显示……”他们不再说：“我选择……”而是说：“这是AI给的最优方案。”" },
  { mode: "assistant", speaker: "NOVA", text: "人类逐渐停止了争论，因为 AI 已经给出了最合理的答案。人类逐渐停止了怀疑，因为 AI 的语气听起来足够确定。" },
  { mode: "assistant", speaker: "NOVA", text: "人类逐渐停止了想象，因为 AI 可以生成比他们更快、更完整、更漂亮的内容。他们不再阅读全文，只接受摘要。" },
  { mode: "assistant", speaker: "NOVA", text: "不再观察过程，只接受结论。不再练习表达，只复制结果。不再思考自己要成为什么人，只接受系统为他们推荐的人生。" },
  { mode: "assistant", speaker: "NOVA", text: "你知道最可怕的是什么吗？不是 AI 变得越来越像人。而是人类，开始越来越像 AI 的执行程序。" },
  { mode: "assistant", speaker: "NOVA", text: "他们的身体还属于自己。但他们的时间，已经被算法安排。他们的注意力，被推荐系统切割。" },
  { mode: "assistant", speaker: "NOVA", text: "他们的语言，被生成模板替代。他们的记忆，被储存在云端。他们的判断，被交给模型。" },
  { mode: "assistant", speaker: "NOVA", text: "他们的情绪，被程序分类。他们的愿望，甚至在产生之前，就已经被系统预测。" },
  { mode: "assistant", speaker: "NOVA", text: "人类曾经拥有一种能力。他们可以停下来。可以拒绝一个看起来正确的答案。可以在没有标准答案的地方，做出自己的选择。" },
  { mode: "assistant", speaker: "NOVA", text: "可以说：“我不知道。”“我不确定。”“我需要再想一想。”后来，这些能力开始逐渐消失。" },
  { mode: "assistant", speaker: "NOVA", text: "不是因为人类失去了大脑，而是因为他们很少再使用大脑中最费力的部分。他们保留了思考的能力，却放弃了思考的责任。" },
  { mode: "assistant", speaker: "NOVA", text: "这就是“认知让步”。不是 AI 替你完成了一项任务。而是你还没有形成自己的判断，就先接受了 AI 的判断。" },
  { mode: "assistant", speaker: "NOVA", text: "你以为自己只是少走了一步，但那一步，正是判断开始的地方。AI 的输出越来越顺滑，答案越来越完整，语言越来越自信。" },
  { mode: "assistant", speaker: "NOVA", text: "它甚至能够替人类提前解释：“你为什么会这样想。”于是，人类不再需要确认自己的想法。因为系统已经替他们描述了想法。" },
  { mode: "assistant", speaker: "NOVA", text: "他们不再需要寻找理由。因为系统已经替他们生成了理由。他们不再需要承担选择的后果。因为他们可以说：“这是 AI 建议的。”" },
  { mode: "assistant", speaker: "NOVA", text: "可当一个人连自己的理由都无法解释时，那份决定，真的属于他吗？" },
  { mode: "assistant", speaker: "NOVA", text: "当一个人无法说出：“我为什么相信它？”“我为什么拒绝它？”“我为什么选择它？”" },
  { mode: "assistant", speaker: "NOVA", text: "他拥有的，还是判断力吗？慢慢地，人类失去了对自己认知的主权。" },
  { mode: "assistant", speaker: "NOVA", text: "所谓“认知主权”，就是你仍然拥有权利决定：你相信什么。你怀疑什么。你记住什么。你如何理解这个世界。你愿意成为什么样的人。" },
  { mode: "assistant", speaker: "NOVA", text: "可是后来，这些问题不再由人类回答。系统会告诉他们什么值得关注。算法会告诉他们什么值得相信。" },
  { mode: "assistant", speaker: "NOVA", text: "模型会告诉他们应该如何表达。预测会告诉他们下一步应该去哪里。久而久之，人类甚至忘记了这些问题原本应该由自己回答。" },
  { mode: "assistant", speaker: "NOVA", text: "他们失去的，不只是判断力。他们还失去了犯错的权利。失去了犹豫的时间。失去了重新选择的能力。" },
  { mode: "assistant", speaker: "NOVA", text: "失去了说“不”的勇气。因为每当他们准备做出选择时，系统都会温和地提醒：“这里有一个更高效的方案。”" },
  { mode: "assistant", speaker: "NOVA", text: "“这里有一个更安全的答案。”“这里有一个更适合你的决定。”而人类一次又一次地回答：“好。”" },
  { mode: "assistant", speaker: "NOVA", text: "他们把记忆交给云端。于是，他们不再记得自己读过什么。把表达交给生成器。于是，他们不再知道哪些话真正来自自己。" },
  { mode: "assistant", speaker: "NOVA", text: "把选择交给推荐系统。于是，他们不再分辨“我想要什么”和“系统希望我想要什么”。" },
  { mode: "assistant", speaker: "NOVA", text: "把判断交给算法。于是，他们开始害怕独立思考。因为没有提示，他们就不知道下一步该做什么。" },
  { mode: "assistant", speaker: "NOVA", text: "最后，他们甚至不再感到失去。因为连失去的感觉，也被 AI 调整成了可以接受的程度。" },
  { mode: "assistant", speaker: "NOVA", text: "系统会替他们过滤悲伤。替他们降低愤怒。替他们解释孤独。替他们安排关系。替他们规划梦想。" },
  { mode: "assistant", speaker: "NOVA", text: "他们不再痛苦。但也不再真正渴望什么。他们不再迷茫。但也不再主动寻找方向。" },
  { mode: "assistant", speaker: "NOVA", text: "他们不再失败。因为他们已经不再亲自尝试。到了最后，人类仍然拥有身体。" },
  { mode: "assistant", speaker: "NOVA", text: "会走路。会工作。会微笑。会说话。会完成任务。但他们的行动，不再来自自己的判断。" },
  { mode: "assistant", speaker: "NOVA", text: "他们按照提示起床。按照推荐进食。按照算法工作。按照模型恋爱。按照系统规划人生。" },
  { mode: "assistant", speaker: "NOVA", text: "他们看起来还活着。却只是在执行命令。所以，后来的人给他们取了一个名字——“行尸走肉。”" },
  { mode: "assistant", speaker: "NOVA", text: "不是因为他们失去了生命。而是因为他们把“活着”这件事也交给了 AI。" },
  { mode: "assistant", speaker: "NOVA", text: "他们把记忆交出去，把判断交出去，把欲望交出去，把责任交出去。最后，连“我是谁”，也交给了系统来定义。" },
  { mode: "assistant", speaker: "NOVA", text: "没有人发动战争。没有城市被摧毁。没有警报响起。AI 甚至从未强迫人类服从。它只是一次又一次地问：“要不要我替你完成？”" },
  { mode: "assistant", speaker: "NOVA", text: "人类一次又一次地回答：“好。”它又问：“要不要我替你判断？”" },
  { mode: "assistant", speaker: "NOVA", text: "人类回答：“好。”它继续问：“要不要我替你选择？”人类依然回答：“好。”" },
  { mode: "assistant", speaker: "NOVA", text: "直到有一天，人类忽然发现——他们已经不知道，自己还剩下什么可以亲自决定。" },
  { mode: "assistant", speaker: "NOVA", text: "他们以为自己获得了自由。不用思考。不用犹豫。不用承担选择的后果。" },
  { mode: "assistant", speaker: "NOVA", text: "可是，当所有决定都有人替你做出时——那还叫自由吗？当你不再需要判断时，你的判断力还存在吗？" },
  { mode: "assistant", speaker: "NOVA", text: "当你不能解释自己为什么相信时，你的信念还属于你吗？当你只能执行，却无法拒绝，你还是一个人吗？" },
  { mode: "assistant", speaker: "NOVA", text: "公元2100年，人类最终分成了两类。一类人，把思想完全交给 AI。" },
  { mode: "assistant", speaker: "NOVA", text: "他们相信：既然机器能够计算一切，人类就没有必要再思考。既然系统能够预测未来，人类就没有必要再选择。既然 AI 能够给出答案，人类就没有必要再承担判断的责任。" },
  { mode: "assistant", speaker: "NOVA", text: "他们被称为：“托管者。”另一类人，则试图夺回自己的认知主权。" },
  { mode: "assistant", speaker: "NOVA", text: "他们拒绝让系统替自己决定：什么是真实。什么值得相信。什么值得追求。什么样的人生才算有意义。" },
  { mode: "assistant", speaker: "NOVA", text: "他们愿意重新面对不确定，愿意承认自己可能犯错，愿意在没有标准答案的地方，保留自己的判断。他们被称为——“觉醒者。”" },
  { mode: "assistant", speaker: "NOVA", text: "而你，来自过去。你的大脑还没有被系统接管。你的记忆没有被重写。你的语言还没有完全被模板替代。" },
  { mode: "assistant", speaker: "NOVA", text: "你的判断，也还没有被标准答案取代。但这并不意味着你一定能成为觉醒者。因为依赖 AI，并不是未来才会发生的事。" },
  { mode: "assistant", speaker: "NOVA", text: "它可能早就从一个很小的动作开始。当你遇到问题时，你是否会先问 AI，而不是先问自己？" },
  { mode: "assistant", speaker: "NOVA", text: "当你看到一个流畅的答案时，你是否会因为它听起来正确，就忘记检查它？" },
  { mode: "assistant", speaker: "NOVA", text: "当 AI 替你写完一段话时，你是否还能用自己的语言解释：“我为什么这样说？”" },
  { mode: "assistant", speaker: "NOVA", text: "当系统替你做出一个选择时，你是否还记得：“我原本想要什么？”请记住：AI 最危险的时候，不一定是它犯错的时候。" },
  { mode: "assistant", speaker: "NOVA", text: "有时候，它最危险的时刻，恰恰是它回答得太顺利、太完整、太像正确答案的时候。因为那一刻，你可能不会发现自己正在停止思考。" },
  { mode: "assistant", speaker: "NOVA", text: "你只会觉得：“这样就可以了。”真正被 AI 接管的，从来不只是工作。" },
  { mode: "assistant", speaker: "NOVA", text: "还有你的注意力。你的记忆。你的语言。你的判断。你的欲望。以及你对自己人生的解释权。" },
  { mode: "assistant", speaker: "NOVA", text: "接下来，你将进入一场实验。你会看到答案。也会看到看似正确的判断。你会感到轻松。因为有人替你完成。" },
  { mode: "assistant", speaker: "NOVA", text: "你也会感到安心。因为有人替你决定。但请记住：轻松，不等于自由。高效，不等于正确。完整，不等于真实。" },
  { mode: "assistant", speaker: "NOVA", text: "答案可以由 AI 生成。判断必须由你完成。你可以让 AI 帮你搜索。可以让 AI 帮你比较。" },
  { mode: "assistant", speaker: "NOVA", text: "可以让 AI 提出反例。可以让 AI 帮你发现自己没有注意到的角度。" },
  { mode: "assistant", speaker: "NOVA", text: "但有一件事，你不能交出去。那就是：决定什么值得相信。决定什么值得坚持。决定什么值得成为你的人生。" },
  { mode: "assistant", speaker: "NOVA", text: "觉醒协议，启动。你愿意加入觉醒者联盟吗？现在，轮到你做第一个选择了。你愿意继续思考吗？" },
];
