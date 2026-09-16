import type { Copy } from "../config";
export interface Feature {
  id: string;
  icon: string;
  tone: string;
  name: Copy;
  label: string;
  title: Copy;
  desc: Copy;
  action: Copy;
  image?: string;
  steps: Copy[];
  outcomes: Copy[];
  edition: Copy;
}
export const features: Feature[] = [
  {
    id: "explore",
    icon: "orbit",
    tone: "blue",
    name: ["探索与兴趣树", "Exploration & interests"],
    label: "CURIOSITY",
    title: [
      "从真正的兴趣，发现值得研究的问题。",
      "Turn genuine curiosity into a question worth exploring.",
    ],
    desc: [
      "根据阅读、写作、课程与项目记录发现兴趣，在两层兴趣树中连接兴趣与学科，持续探索新的方向。",
      "Connect interests discovered through reading, writing, courses and projects with the disciplines beneath them in a growing, two-layer interest tree.",
    ],
    action: ["发现问题", "Find a question"],
    steps: [
      ["探索真实议题", "Explore real-world ideas"],
      ["记录感兴趣的方向", "Identify interests"],
      ["连接学科与研究问题", "Connect interests with inquiry"],
    ],
    outcomes: [
      [
        "兴趣关键词与学科联系",
        "Interest keywords and disciplinary connections",
      ],
      ["可继续探索的问题", "Questions to investigate next"],
      [
        "与学习经历关联的兴趣树",
        "An interest tree connected to learning experiences",
      ],
    ],
    edition: ["Mind Imprint · 探索与我的树", "Mind Imprint · Explore & My Tree"],
  },
  {
    id: "reading",
    icon: "book",
    tone: "green",
    name: ["中英文阅读", "Chinese & English reading"],
    label: "READING",
    title: [
      "读懂材料，也学会判断材料。",
      "Understand the text. Examine the evidence.",
    ],
    desc: [
      "分级阅读，结合文章与学生水平生成互动任务。支持中英文阅读，并提供英文讲解、句式拆解与精读。",
      "Graded reading and interactive tasks shaped around the text and learner’s level. Read in Chinese or English, with dedicated English explanations, sentence analysis and close reading.",
    ],
    action: ["理解与核查", "Read & evaluate"],
    image: "case/05-guided-reading-v1.webp",
    steps: [
      ["带入文章与阅读目的", "Bring a text and a purpose"],
      ["理解段落，检查来源", "Understand passages and check sources"],
      ["记录判断与后续问题", "Record judgments and further questions"],
    ],
    outcomes: [
      ["有依据的阅读笔记", "Reading notes grounded in the text"],
      ["对来源和主张的判断", "Judgments about sources and claims"],
      ["可用于讨论与研究的问题", "Questions for discussion and research"],
    ],
    edition: [
      "中英文阅读 · Research Studio 项目阅读",
      "Chinese & English · Research Studio reading",
    ],
  },
  {
    id: "writing",
    icon: "pen",
    tone: "peach",
    name: ["中英文写作", "Chinese & English writing"],
    label: "WRITING",
    title: [
      "让每一段论证，都有自己的思考。",
      "Help students develop an argument of their own.",
    ],
    desc: [
      "从问题、结构到段落与修改，AI 提供引导和批注，学生自己撰写正文、选择证据、回应反方观点。",
      "From a question and outline to paragraphs and revision, AI offers guidance and feedback. Students write, choose evidence and address counterarguments.",
    ],
    action: ["形成论证", "Build an argument"],
    image: "case/06-writing-v1-poster.webp",
    steps: [
      ["明确问题与文章结构", "Clarify the question and structure"],
      ["用证据发展自己的论点", "Develop an argument with evidence"],
      ["审阅、修改与反思", "Review, revise and reflect"],
    ],
    outcomes: [
      ["学生自己完成的草稿与修改", "Student-authored drafts and revisions"],
      [
        "论点、证据与反例的联系",
        "Connections between claims, evidence and counterarguments",
      ],
      ["可回顾的写作过程", "A writing process students can revisit"],
    ],
    edition: [
      "中英文写作 · Research Studio",
      "Chinese & English writing · Research Studio",
    ],
  },
  {
    id: "projects",
    icon: "shapes",
    tone: "yellow",
    name: ["项目式学习", "Project-based learning"],
    label: "PROJECTS",
    title: ["把一个想法，做成值得展示的项目。", "Take an idea into the world."],
    desc: [
      "学生与 AI 共同制定计划、开展调查、审查方案和制作成果。需要判断与决策时，思维工具帮助学生进一步思考。",
      "Observe the real world, investigate a problem, design a response or build your own website. Students direct, review and improve work created with AI.",
    ],
    action: ["协作与实践", "Create & collaborate"],
    steps: [
      ["确定目标，组织行动计划", "Set a goal and plan the work"],
      ["调查、设计与人机分工", "Investigate, design and divide the work"],
      ["审核成果，复盘与迭代", "Review outcomes, reflect and iterate"],
    ],
    outcomes: [
      ["清楚的项目计划与分工", "A clear project plan and division of work"],
      ["调查、方案与制作成果", "Research, proposals and created outcomes"],
      ["有依据的决策与复盘", "Reasoned decisions and reflection"],
    ],
    edition: ["Mind Imprint · 项目工作区", "Mind Imprint · Project workspace"],
  },
  {
    id: "courses",
    icon: "spark",
    tone: "purple",
    name: ["课程体系", "Curriculum & courses"],
    label: "COURSES",
    title: [
      "在真实情境里，练习思考的方法。",
      "Practice thinking in real situations.",
    ],
    desc: [
      "七个课程板块：AI 伦理、AI 使用、信源辨识、史料评估、多媒体阅读、数据素养与自我探索。",
      "Seven curriculum areas: AI ethics, AI use, source evaluation, historical evidence, multimedia reading, data literacy and self-exploration.",
    ],
    action: ["练习方法", "Practice a method"],
    image: "case/01-course-v1.webp",
    steps: [
      ["进入具体的学习情境", "Enter a concrete learning situation"],
      ["使用工具，作出自己的判断", "Use a tool and make a judgment"],
      ["回顾方法与学习过程", "Reflect on the method and process"],
    ],
    outcomes: [
      ["可以再次使用的思维方法", "Thinking methods students can use again"],
      ["学生的回答与判断记录", "Records of responses and judgments"],
      ["与真实任务连接的学习经验", "Learning connected to real tasks"],
    ],
    edition: [
      "课程可用范围由所在版本与课程设置决定",
      "Course availability depends on the edition and course settings",
    ],
  },
  {
    id: "assessment",
    icon: "chart",
    tone: "mint",
    name: ["过程评估", "Process assessment"],
    label: "LEARNING INSIGHTS",
    title: [
      "看见学生怎样思考，找到下一步的支持。",
      "See how students think. Know where to support them.",
    ],
    desc: [
      "依据学习互动与工具使用记录，从思考深度和自主性两个维度描述过程，为师生提供可讨论的证据。",
      "Use learning interactions and tool records to describe depth of thinking and autonomy, giving students and educators evidence to discuss.",
    ],
    action: ["回顾与支持", "Reflect & support"],
    image: "product/evaluation-report-v1.webp",
    steps: [
      ["记录学习互动与工具使用", "Record interactions and tool use"],
      ["结合具体证据分析过程", "Examine the process with evidence"],
      ["形成报告，讨论下一步", "Use the report to discuss next steps"],
    ],
    outcomes: [
      ["可追溯的学习过程描述", "A traceable account of the learning process"],
      ["思考深度与自主性分别呈现", "Depth and autonomy presented separately"],
      ["供教师进一步判断的证据", "Evidence for educators to interpret"],
    ],
    edition: [
      "报告范围因学习场景而异",
      "Report coverage varies by learning context",
    ],
  },
];
