-- +goose Up
-- Guided-tour demo project (P8 real-scene deepening, Task 2): seed REAL
-- proposal-track guide cards + the student's own snippet text into the demo
-- project (…0200), so the real GuidedWritingCard renders genuine filled boxes
-- instead of an empty/never-started guided track.
--
-- studio_state.proposalTrack persists agent.WritingTrack (apps/api/internal/
-- agent/proposal_track.go:33-39). stepGuides is map[string]string where EACH
-- VALUE is a DOUBLE-ENCODED JSON STRING — an escaped {"prompt","example",
-- "refHint"} object, not a nested jsonb object (confirmed via apps/api/
-- internal/api/proposal_track.go:60,137,167-172, which json.Unmarshal's each
-- string value into guideCardDTO). mode="guided", started=true, stepIndex=0
-- (the "understanding" step) so the server's lazy-generation branch
-- (buildProposalGuideStep) hits the seeded cache and never fires a live model
-- call on GET. The 9 keys are agent.ProposalFixedParts() in order; the
-- research-plan step is a subq-define step with NO card and is intentionally
-- absent from stepGuides.
--
-- Each card.prompt is a Chinese guiding QUESTION applied to the demo research
-- question ("中国是否让地球变得更可持续？") — never an answer. Each
-- card.example is a short ENGLISH paragraph on a DIFFERENT topic (铁律① — it
-- demonstrates the shape of good thinking only, never this project's answer).
-- Each prop:<key> snippet is Phoebe's own first-person text for that part,
-- consistent with the demo's greening/sustainability project (skeptical that
-- satellite greening = sustainability, given China is the top annual CO2
-- emitter) — mirrors the seeding style of 0082:236-246.
--
-- Idempotent: jsonb_set overwrite is a no-op re-run; snippet inserts use
-- ON CONFLICT (id) DO NOTHING (mirrors 0082/0083/0084).

UPDATE project
SET studio_state = jsonb_set(
  studio_state,
  '{proposalTrack}',
  '{
    "mode": "guided",
    "started": true,
    "stepIndex": 0,
    "subQuestions": [],
    "stepGuides": {
      "understanding": "{\"prompt\":\"这个题目里，『让地球更可持续』具体指什么？你打算用哪一两个可核查的指标（比如碳排放、绿化面积、清洁能源投资）来判断，而不是凭直觉下结论？\",\"example\":\"Consider the claim that this city has become more livable. Before judging it, you need to decide what livable means in checkable terms, such as commute time, air quality index, or park space per resident, otherwise the claim stays a slogan no evidence can test.\",\"refHint\":\"对应立题四维框架中的『目标』与『理由』\"}",
      "question-scope": "{\"prompt\":\"你的研究问题目前是『中国是否让地球变得更可持续？』——这个问题的时间范围、地理范围和排除项分别是什么？哪些相关但你不打算展开的角度需要明确排除？\",\"example\":\"Does remote work increase productivity is too broad a question. A workable version narrows the years studied, the industry, and explicitly excludes freelancers who were never in an office to begin with.\",\"refHint\":\"对应研究问题的时间/地理边界与排除项\"}",
      "thesis": "{\"prompt\":\"基于你目前读到的证据，你的暂定论点是什么？它是不是一个『带限定条件』的判断，还是一句可以被单一反例轻易推翻的绝对结论？\",\"example\":\"A weak thesis says coffee ruins sleep. A qualified one says coffee consumed within six hours of bedtime measurably reduces deep sleep for most adults, though the effect size varies by individual tolerance.\",\"refHint\":\"厚论点应能同时吸收支持证据与反例，而不被任一方推翻\"}",
      "resources": "{\"prompt\":\"你计划依赖哪些一手资料、数据库或工具？如果关键数据来自政府或机构统计，你打算怎么核实它没有被夸大或选择性引用？\",\"example\":\"For a paper on a 1980s factory strike, useful primary resources include union meeting minutes, newspaper archives from that year, and an oral history interview, each chosen because it lets you check one specific claim rather than just because it exists.\",\"refHint\":\"区分『可作证据的一手来源』与『只作溯源入口的二手来源』\"}",
      "challenges": "{\"prompt\":\"写这篇论文时，你预见到哪些具体的困难——是数据口径不一致、来源立场偏颇，还是证据之间互相矛盾？你打算怎么处理？\",\"example\":\"A student researching screen time and sleep quality may find that studies define screen time differently, some counting television and some not, which makes direct comparison across studies a real methodological challenge.\",\"refHint\":\"留意不同数据的统计口径（累计 vs. 年度、绝对值 vs. 份额）\"}",
      "method": "{\"prompt\":\"你打算用什么方法收集和处理证据？比如先横向溯源、再交叉验证、最后画论证图——具体步骤是什么，遇到矛盾证据时你打算怎么权衡？\",\"example\":\"A student studying urban heat islands might describe her method as follows: first collect temperature readings from three independent weather stations, then cross check them against satellite thermal imagery, then compare both against a survey of residents reported experience.\",\"refHint\":\"对应 CRAAP 溯源 → 交叉验证 → 论证图三步\"}",
      "feasibility": "{\"prompt\":\"这个研究计划在你现有的时间和资源下可行吗？有没有伦理上需要注意的地方（比如涉及他人隐私、有争议的立场表述）？\",\"example\":\"A project surveying classmates about eating habits needs to think about consent, anonymity, and whether questions about weight or body image could cause discomfort. Feasibility is not just about time, it is also about doing no harm.\",\"refHint\":\"确认所需一手资料可获取，且没有涉及他人隐私的问卷/访谈\"}",
      "expected": "{\"prompt\":\"如果你的假设是对的，你预期会看到什么样的证据？如果证据和预期相反，你会怎么调整论点，而不是硬塞进原来的框架？\",\"example\":\"If a student hypothesizes that more sunlight increases seedling growth, she should state in advance what the growth curve should look like, and decide beforehand what result would count as disproving the hypothesis rather than only confirming it.\",\"refHint\":\"预先写下能证伪你假设的证据长什么样\"}",
      "polish": "{\"prompt\":\"通读全文一遍：每一段是不是都服务于你的论点？有没有哪句话的措辞比证据能支撑的更强（比如用了『证明』『拯救』这类词）？\",\"example\":\"On a final read through, a writer notices she wrote that volunteering completely changed her worldview, a claim her three paragraphs of evidence do not actually support, and revises it to say volunteering shifted how she thinks about a few specific privileges she had taken for granted.\",\"refHint\":\"逐句核对动词强度（『证明』『拯救』）是否被证据支撑\"}"
    }
  }'::jsonb
)
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

-- ── 立题·guided proposal track: Phoebe's own text per fixed part ─────────────
INSERT INTO snippet (id, project_id, text, section, position) VALUES
  ('00000000-0000-0000-0000-0000000002c3', '00000000-0000-0000-0000-000000000200',
   '『让地球更可持续』对我来说不能只是「变绿了」这种视觉印象，我打算用两个可核查的指标来判断：一是全球绿叶面积净增里中国的贡献份额（有一手卫星数据可查），二是中国年度碳排放占全球比重的变化趋势（有 Global Carbon Project 的公开核算）。如果只看第一个指标就下结论，我很容易被『卫星证明中国拯救地球』这种反转叙事说服；两个指标放在一起看，才可能得到一个不被单一叙事绑架的判断。',
   'prop:understanding', 0),
  ('00000000-0000-0000-0000-0000000002c4', '00000000-0000-0000-0000-000000000200',
   '我的研究问题聚焦在 2000 年到现在这段时间——因为 Chen et al. 的卫星数据覆盖 2000–2017，而碳排放数据我会延伸到最新年份做趋势对照。地理范围是中国整体，不细分省份。我明确排除『中国是否应该为历史排放负责』这类道德归因问题，也不打算展开讨论其他国家的对比排名，那会把一个可控的问题变成写不完的比较研究。',
   'prop:question-scope', 0),
  ('00000000-0000-0000-0000-0000000002c5', '00000000-0000-0000-0000-000000000200',
   '我的暂定论点是：中国的环境治理呈现真实且持续增强的决心，全球绿叶面积净增中中国贡献最大——但存量碳排放仍居世界第一，因此『趋势变好』不等于『问题已解决』。这是一个带限定条件的判断：它能同时吸收『变绿』和『排放第一』这两组看似矛盾的证据，而不是选择性地只用其中一组来证明自己想要的结论。',
   'prop:thesis', 0),
  ('00000000-0000-0000-0000-0000000002c6', '00000000-0000-0000-0000-000000000200',
   '核心一手资源是 Chen et al. (2019, Nature Sustainability) 的卫星绿化数据论文；用来做治理决心的第二条独立证据是 IEA《World Energy Investment》的中国清洁能源投资数据；用来做反例检验的是 Global Carbon Project 的年度碳排放核算。至于那篇最初带我入门的自媒体文章，我打算只把它当作溯源的起点记进 source log，不会把它当成证据引用进正文。',
   'prop:resources', 0),
  ('00000000-0000-0000-0000-0000000002c7', '00000000-0000-0000-0000-000000000200',
   '我预见到的最大挑战是数据口径不一致：『变绿 5%』是 2000–2017 年的累计增量，『排放占全球 31%』是 2022 年的年度份额，两者时间跨度和统计口径都不一样，直接放在一起比较很容易变成偷换概念。我打算在每次引用数据时都注明口径（累计 vs. 年度、绝对值 vs. 份额），逼自己不把不同口径的数字互相抵消或叠加。',
   'prop:challenges', 0),
  ('00000000-0000-0000-0000-0000000002c8', '00000000-0000-0000-0000-000000000200',
   '我的方法分三步：第一步用 CRAAP 把那篇自媒体文章横向溯源回 Chen et al. 原论文和 NASA 数据；第二步分别就『清洁能源投资』『存量碳排放』两条线各找一手来源交叉验证，尤其主动去找对我的论点不利的证据；第三步用论证图把『主张—证据—推理』连起来，遇到矛盾证据（比如变绿趋势和最大排放国身份）时，用让步段处理，而不是挑一个忽略另一个。',
   'prop:method', 0),
  ('00000000-0000-0000-0000-0000000002c9', '00000000-0000-0000-0000-000000000200',
   '这个题目在我现有的资源下是可行的——需要的一手论文和机构报告都能通过学校图书馆数据库或公开渠道获取，不涉及问卷或访谈，不需要额外的伦理审批。唯一需要注意的伦理点是：我在引用『中国』作为整体行动者时，要避免把复杂的国家治理简化成单一叙事（无论是『拯救地球』还是『头号排放国』），措辞上要留出让政策差异和地区差异存在的空间。',
   'prop:feasibility', 0),
  ('00000000-0000-0000-0000-0000000002ca', '00000000-0000-0000-0000-000000000200',
   '如果我的论点成立，我预期会看到：卫星和投资数据显示中国在变绿和清洁能源上确实领先，但同一时期碳排放总量和煤电装机并没有同步下降。如果证据和预期相反——比如发现煤电产能其实在同步收缩——我会调整论点，承认『结构性转型』的证据比我预想的更强，而不是硬把这个新证据塞进『进行时而非完成时』的原框架里。',
   'prop:expected', 0),
  ('00000000-0000-0000-0000-0000000002cb', '00000000-0000-0000-0000-000000000200',
   '通读全文时，我删掉了『中国拯救了地球』和『证明了可持续』这类措辞过强的句子，因为我的证据只能支撑『绿叶面积净增贡献最大』和『清洁能源投资全球第一』，撑不起『拯救』或『证明』这种级别的断言。我也检查了让步段是不是真的限定了结论，而不是写完就晾在那里没有回应前面的论点——最后把『问题已解决』改成『转型仍在进行中』，让措辞和证据的强度对齐。',
   'prop:polish', 0)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
UPDATE project
SET studio_state = studio_state - 'proposalTrack'
WHERE id = '00000000-0000-0000-0000-000000000200' AND is_demo = true;

DELETE FROM snippet
WHERE project_id = '00000000-0000-0000-0000-000000000200' AND section LIKE 'prop:%';
