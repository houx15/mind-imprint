# 兴趣树的根 · 领域闭表 · 采集改任务

> 2026-09-04。取代 `2026-09-02-interest-model-and-exploration-design.md` 里
> **采集器的词表**与**采集时机**两节；学科表与其余部分不变。

## 为什么改

线上那棵树上的词是「例外与代表性」「一个结论要多少证据」这种粒度 —— 采集
prompt 明写着要这个粒度，也明写着 `差：珊瑚、环保`。Owner 的判断是这批词
**太零碎**，描述一个人的兴趣该用 **金融 · 裁缝 · 游戏** 这种词。

这是对原设计的反转，不是打补丁。同时反转的还有采集时机：现在是打开树时惰性
补采，改成任务。

## 三件事

1. **词表**：新增 `packages/contracts/interests/interests.json`，一张**闭表**。
   采集器只能选，不能造。
2. **树加根**：树冠一个像素不改。地面（y=748）以下加七根主枝的镜像根，42 门
   学科分布在根上。她的词是**叶子**。
3. **采集改任务**：完成时入队 + 周期扫尾，`GET /interest/tree` 变成纯读。

---

## 1 · 领域词表

### 入表标准

一条词要同时满足四条才进表：

- 能填进「我对 ___ 感兴趣」；
- 一个陌生人一眼认得，不需要解释；
- 2–4 字为主，最多 6 字；
- 是**人类活动的一个领域**（行业 / 手艺 / 媒介 / 现象 / 学问的世俗名字）。

据此：**金融 ✓ 裁缝 ✓ 游戏 ✓ 城市 ✓ 疫苗 ✓ 方言 ✓**；
**珊瑚 ✗**（一篇文章的话题）**环保 ✗**（口号）**例外与代表性 ✗**（思考动作）。

### 结构

```json
{
  "id": "games",
  "field": "making",
  "zh": "游戏",
  "en": "Games",
  "disciplines": ["interaction-design", "probability", "visual-design"]
}
```

- `field` 沿用同样的七根主枝，树的几何与配色因此一行不用改。
- **`disciplines[]` 是作者写好的静态边。** 这是本设计成本最低的一处：一个词
  连哪几门学科不再靠模型判定，查表即可。至少两条 —— 一条边的领域说不出
  「它是几门学问的组合」，而那正是这张表存在的理由。
- 七根枝**分布不均**是预期内的：formal 在世俗词里天然薄，making / arts 天然厚。
  一棵枝叶不均的树说的是她实际住在哪里。实际分布：formal 18 · science 40 ·
  making 45 · society 46 · humanities 34 · arts 39 · self 27，共 249 条。

**只有这五个字段**，每个都有真实读者：`ID` 落在 `interest_keyword` 上并在落库前
过 `Exists`；`Field` 决定叶子挂哪根枝；`Zh`/`En` 显示；`Disciplines` 是根。

早期草稿里还有 `aliases` 与 `asks`，**都删了**。别名唯一的用处是喂 prompt，而把
一千个别名塞进 system 会让每次采集的输入涨三倍，收益是模型本来就能做的词面
匹配；`asks` 则和学科自己的 `asks` 重复，界面上显示的是后者。

真相源是 `packages/contracts/interests/build.py`（条目写成元组，一行一条），跑
它同时写出 `packages/contracts/interests/interests.json` 与 Go 侧副本。

### Go 侧

`apps/api/internal/interests/`，逐字节副本 + `go:embed`，
`TestEmbeddedCopyMatchesSourceOfTruth` 守着 —— 和 `internal/disciplines`、
`internal/vocab` 同一个办法（`go:embed` 出不了包目录）。

`TestEveryDisciplineIDIsReal` 守 `disciplines[]` 里每个 id 都在 42 门里。
打错一个 id 不会编译报错，只会让一片叶子连不到任何根。

### 前端侧

`packages/contracts/interests/interests.json` 和
`packages/contracts/disciplines/disciplines.json` 都由 lite-web **构建期 import**
（`@mind-imprint/contracts` 已经是 workspace 依赖）。学科表是内容不是用户数据，
不进接口返回。

---

## 2 · 采集：闭表 + 降一档

### prompt

system 里带上候选词表（约 240 条 zh，约 1.5k token）。要求：从表里选 1–3 个词，
每个词给出**她自己的那句原话**。

三条**一个字不改**：

- `KeepGrounded` —— evidence 必须逐字出现在她真写下的语料里；
- `maxPerHarvest = 3`；
- **阅读只喂她的收获与批注，不喂文章正文**。闭表最大的风险是退化成话题标注
  （文章提到游戏 → 打上「游戏」），这条已有规则挡掉绝大部分。

### 新的可验不变量

`ParseHarvestReply` 加一条：**返回的 id 必须在词表里**，不在就丢。和
`disciplines.IsField` 同一形状。模型编一个 `"id": "esports"` 出来，丢掉，不落库。

### 能力档

`ClassCompose` 改 `ClassDigest`。从 240 条里挑 3 条是长输入短输出的分类，不是创作。

### 路由退休

`internal/interest/router.go`（alias / cooccur / llm 三档）**整个删除**，
连同 `BuildRoutePrompt` / `ParseRouteReply` / `RouteCheap` / `Known` 与其测试。
词表自带学科边，这三档没有调用方了。

- `keyword_discipline.how` 的 CHECK 加 `'catalog'`。
- `how = 'student'` 不变，仍然压过一切。
- `confidence` 对 catalog 边恒为 `1.0`。

---

## 3 · 采集改任务

### 入队 + 扫尾，不是纯 cron

1. **完成时入队**：`finishReading` / `finishWritingAtom` / `finishProject` 在
   翻转之后 `river.Insert` 一条 `HarvestAtomArgs{AtomID}`。**尽力而为**：入队
   失败只 warn，不影响她那次请求，扫尾会捞回来。
2. **周期扫尾**：river `PeriodicJob`，**每 2 分钟**跑一次
   `ListPendingHarvestAtoms`。

纯 cron 不够：她读完一篇立刻打开树会看不到新词。纯入队不够：入队失败、
本次上线之前积压的、以及重采那一批都需要有人捞。

### 扫尾的取数规则

```sql
WHERE a.interest_harvested_at IS NULL
  AND <该 kind 已完成>
  AND a.last_activity_at > now() - interval '30 days'   -- 「最近活跃」这道门
ORDER BY a.last_activity_at DESC
```

外面套一层 `row_number() OVER (PARTITION BY a.user_id ...)`，**每个学生每轮
最多 3 个**，整轮最多 60 个。一个读了二十篇的学生不能饿死其他所有人。

⚠️ **调用量净增**：今天只有打开过树的学生花那次调用，改完之后每个完成任何
一件事的学生都花。30 天这道门加上 `compose` 降 `digest` 抵掉一部分，净增仍是真的。

### 惰性那层删掉

`GET /interest/tree` 里的 `a.harvestPending(...)` 整段删掉，接口变成纯读。
`interest_harvest.go` 顶部那段解释「为什么采集不发生在完成那一刻」的注释一并
改写 —— 它给的两个理由（瞬时翻转不能挂三秒调用、repo 里没有 fire-and-forget
先例）现在都由队列解决了。

`harvestOneAtom` 的 advisory lock **保留**：入队和扫尾可能同时盯上一个 atom。

### river 起不来怎么办

`cmd/api` 里 river 启动失败等于 **log error 后继续服务**。队列是可降级的子系统，
不是正确性不变量；为它拒绝启动会让整个接口下线。测试里 river client 是 nil，
所以 `if a.d.River == nil` 是走得到的分支，不是防御性代码
（同 `a.d.Provider == nil` 那三处）。

---

## 4 · 存量：归档再重采

迁移 `0122`：

1. `CREATE TABLE interest_keyword_v1_archive AS SELECT * FROM interest_keyword;`
   `keyword_source` / `keyword_discipline` 同样各存一份。
2. 清空三张表。
3. `UPDATE atom SET interest_harvested_at = NULL;`

扫尾任务会按新词表把树重新长一遍。归档表让这一步可逆 —— 清掉的是学生看得见
的东西，不该没有退路。

---

## 5 · 树加根

### 树冠

**一个像素不改。** `BRANCH_CURVES`、树干那条贝塞尔、七根主枝的标签位置、
`placeOnBranch` 的顺序铺开，全部保留。

唯一的变化是**她的词从光珠变成叶子**：叶柄落在枝上 `bez(branch, t)`，叶尖沿
法向伸出 `max(56, |spread| * 1.35)`，叶片是两段三次贝塞尔围成的梭形，中间一条
叶脉。方向沿用原来的 `spread` 正负，所以左右交替的布局语义不变。

### 根

`ROOT_CURVES`：七根主枝在地下的镜像，起点在树干 y=760 到 830，浅的铺得宽，
`formal` 是扎得最深的主根。每根主根上再分两条副根，根系才是一团而不是七条线。

42 门学科按 `field` 分布在对应的根上，每根六个，沿曲线 `t` 从 0.34 到 0.94
铺开，法向交替偏移 —— 和树冠上 `placeOnBranch` 同一个办法，同一个理由
（哈希会撞）。

**位置由学科表里的顺序决定**，因此每个学生看到的根系是同一个。这是对的：
学科表是世界，不是她的数据。

### 连线不常驻

四个词乘三门学科等于十二条线穿过树干，是一团乱。所以：

- **点一片叶子**，它的学科亮起，线顺着树干往下长到那几门；其余全部压到 0.13。
- **点一门学科**，共用它的叶子亮起，线往上长。这是根系比枝干多出来的那件事：
  「你的游戏和金融，底下是同一根 —— 概率。」
- **点空白**，清除。

被两个以上领域共用的学科节点带一圈细环，不点也看得出来。

### 底部面板

| 状态 | tag | 标题 | 说明 | 正文 | 脚注 |
|---|---|---|---|---|---|
| 未选 | 请点选 | 我的树 | 树冠是领域，根是学科 | 三句操作说明 | 领域 N · 学科 42 |
| 选了叶子 | 领域 | 游戏 | 它的三门学科 | 你写过：「…」 | 来源 |
| 选了学科 | 学科 | 概率 | 它问什么 | 她的原话，或共用它的领域 | IB 编号 |

「怎么连上的」**用她自己的原话**，不写授权解释。省掉 240 乘 3 句手写文案，
而且说的是真的。

### 零个和一个领域

不尴尬，而且这是根系比原来那棵树强的地方：**学科表是固定的 42 门，土里本来
就有东西。** 零个领域时树冠和根都在，只是没有叶子、学科也不具名（学科在有
叶子连到它之前只是一个暗点）。空状态的入口（兴趣测试 / 去阅读）不变。

原来那棵树没有词就是七根空枝。

---

## 6 · 测什么

按「只写逻辑测试，不堆前端渲染测试」：

- 词表 embed 副本与真相源逐字节一致；
- `disciplines[]` 里每个 id 都是真学科；
- 词表 id 唯一、`field` 合法、别名不跨词重复；
- `ParseHarvestReply` 丢掉表外 id、丢掉空 evidence、去重、截断到 3；
- `KeepGrounded` 不变；
- 扫尾取数：每学生配额、30 天窗口、已采过的不再捞；
- 入队与扫尾对同一 atom 只采一次（advisory lock）；
- catalog 边写入 `how='catalog'`、`confidence=1`，且 `how='student'` 不被覆盖；
- 一次 `LIVE_LLM=1` 真模型跑，看它是不是真的只从表里选。

前端**不写渲染断言**。用 Playwright 截图看，包括零领域和一个领域两个状态。

---

## 7 · 分期

| 期 | 内容 |
|---|---|
| B1 | 词表 + Go 包 + 一致性测试。零行为变化 |
| B2 | 采集换闭表、降 digest、退休 router、`how='catalog'`、迁移 0122 归档重采 |
| B3 | river client + worker + 完成入队 + 周期扫尾；树接口变纯读 |
| B4 | 树加根：叶子、根系、点击连线、面板 |

四期都能独立合并。真实依赖只有 B2 依赖 B1、B3 依赖 B2；B4 只依赖接口已有的
`keyword.disciplines[]`，和前三期并行也成立。
