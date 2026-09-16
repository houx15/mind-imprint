// Team biographies and public portrait keys migrated from apps/site.
export const team: {
  name: [string, string];
  role: [string, string];
  bio: [string, string];
  /** media key under the site CDN; absent means the monogram */
  photo?: string;
  /** shown when there is no photo; a Latin initial reads better in English */
  initial: [string, string];
}[] = [
  {
    name: ["陈玉洁", "Yujie Chen"],
    role: ["创始人", "Founder"],
    photo: "team/chen-yujie-v1.webp",
    initial: ["陈", "C"],
    bio: [
      "教育学、心理学与社会学的跨学科背景，在德国、美国、澳大利亚求学与工作。曾任职于联合国训练研究所（UNITAR）纽约，从事媒介与信息素养、可持续发展教育与 AI 课程设计。她长期在不同国家、文化与教育体系之间做课程协作，擅长把复杂议题转化为可学习、可评估、可行动的教育方案。",
      "An interdisciplinary background in education, psychology and sociology, with study and work across Germany, the United States and Australia. At UNITAR in New York she worked on media and information literacy, education for sustainable development and AI curriculum design. Years of collaboration across countries, cultures and education systems have made her expert at turning a complex issue into something that can be learned, assessed and acted on.",
    ],
  },
  {
    name: ["侯煜欣", "Yuxin Hou"],
    role: ["创始人", "Founder"],
    photo: "team/hou-yuxin-v1.webp",
    initial: ["侯", "H"],
    bio: [
      "清华大学毕业，工学、教育学、心理学与社会学的跨学科背景，在 AI + 教育领域从业十余年。曾作为联合创始人与 CTO 研发全国首个自适应 AI 教师，也曾在联合国儿童基金会（UNICEF）参与社会情感学习课程设计。她持续研究的问题只有一个：AI 时代真正的核心竞争力是什么，以及它到底怎么习得。",
      "A Tsinghua graduate with an interdisciplinary background spanning engineering, education, psychology and sociology, and more than a decade in AI and education. She was co-founder and CTO of China's first adaptive AI teacher, and worked on social-emotional learning curriculum design at UNICEF. One question runs through all of it: what the core competence of the AI era actually is, and how it is actually acquired.",
    ],
  },
  {
    name: ["杨欣松", "Xinsong Yang"],
    role: ["技术顾问", "Technical adviser"],
    initial: ["杨", "Y"],
    bio: [
      "计算机科学与社会科学的跨学科背景，多年软件工程与互联网产品研发经验，曾任职于 Tubi、爱奇艺等科技公司，参与大规模互联网产品与数据系统的研发。近年持续关注生成式 AI 如何改变人的学习、思考与决策方式，并探索 AI 原生产品的设计与开发。",
      "An interdisciplinary background in computer science and the social sciences, with years of software engineering and product work at Tubi and iQIYI on large-scale consumer products and the data systems behind them. Lately his attention has been on how generative AI changes the way people learn, think and decide — and on what an AI-native product should therefore be.",
    ],
  },
  {
    name: ["周晓卓", "Xiaozhuo Zhou"],
    role: ["课程实习生", "Curriculum intern"],
    photo: "team/zhou-xiaozhuo-v1.webp",
    initial: ["周", "Z"],
    bio: [
      "北京大学国际中文教育硕士，语言学、认知科学与心理学的跨学科背景，研究方向涵盖二语习得、实验语用学与心理语言学。曾任高校辩论队队长与学生心理协会会长，有中文教学、跨文化教育与逻辑思维训练的实践经验。她关心的是怎么把复杂知识变成清晰、可参与的学习体验。",
      "A master's in international Chinese-language education from Peking University, with an interdisciplinary background in linguistics, cognitive science and psychology — second-language acquisition, experimental pragmatics, psycholinguistics. She has captained a university debate team, chaired its student psychology society, and taught Chinese across cultures. Her question is how complex knowledge becomes a learning experience clear enough to take part in.",
    ],
  },
  {
    name: ["郭浩然", "Haoran Guo"],
    role: ["研究实习生", "Research intern"],
    photo: "team/guo-haoran-v1.webp",
    initial: ["郭", "G"],
    bio: [
      "清华大学博士生在读，专业为管理科学与工程。",
      "A PhD candidate in management science and engineering at Tsinghua University.",
    ],
  },
];
