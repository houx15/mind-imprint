# 分级阅读库的内容流水线

四十八篇报道，每篇五个难度版本。这个目录是从导出的 markdown 到线上可读文章
之间的全部步骤。

图有两个数，别把它们看成对不上：`make_images.py` 处理**语料里的全部** 127 张，
`articles.json` 引用其中 **125** 张 —— 差的两张属于 `holds` 里那篇没上线的文章。
上传是按处理结果传的，所以传上去的比引用的多，这是对的。

**依赖**：`make_images.py` 要 Pillow（`pip install Pillow`）。其余脚本只用标准库。
第 5、6 步要 `.deploy-local/`（gitignore 的部署凭据），没有的话找 owner 要。

上一批怎么上的，见文末「上一批（2026-09-10）实际发生了什么」——
那一节是这份流水线目前为止踩过的坑的清单。

## 上一批新语料，做这几件事

```bash
# 0. 在 sources.json 的 batches 里加一条，指向新语料目录（相对仓库根）。
#    形状不重要：脚本递归找 *.md 和叫 images 的目录，分档看文件名后缀。

# 1. 先只解析，不出片，看看这批长什么样
#    （dist/ 是 gitignore 掉的中间产物目录，第一次跑要先建出来）
mkdir -p deploy/reading-library/dist
python3 deploy/reading-library/parse.py > deploy/reading-library/dist/parsed.json
#    先跑这一条：它是三把尺子里最准的一把（同一张图的图注在几档里对不上，
#    就是坏掉的地方）。三把都是「报给人看」，都不会让构建失败。
python3 deploy/reading-library/audit_captions.py < deploy/reading-library/dist/parsed.json
python3 deploy/reading-library/audit_units.py    < deploy/reading-library/dist/parsed.json
python3 deploy/reading-library/audit_names.py    < deploy/reading-library/dist/parsed.json

# 2. 给每篇写 tags.json（中文标题 / 一句话理由 / 学科），
#    确实要放弃的写进 tags.json 的 holds

# 3. 图：原图 -> 1600px WebP + 一份对象键清单
python3 deploy/reading-library/make_images.py

# 4. 目录：markdown + tags + corrections + 图清单 -> articles.json
python3 deploy/reading-library/build.py

# 5. 图传到 OSS（键是确定的，articles.json 里已经写死了它们）
READING_OSS_ENV=.deploy-local/env.prod deploy/upload-reading-images.sh --dry-run
READING_OSS_ENV=.deploy-local/env.prod deploy/upload-reading-images.sh

# 6. 测试、部署、线上走一遍
(cd apps/api && CGO_ENABLED=0 go test ./internal/library/... ./internal/api/... -timeout 1800s)
.deploy-local/deploy.sh api
deploy/reading-library/smoke_prod.sh                       # 书架 → 开一篇 → 正文 + 图
deploy/reading-library/probe_recommend.sh                  # 新标的学科真的能被推出来
READING_OSS_ENV=.deploy-local/env.prod \
  deploy/reading-library/probe_cdn.sh web/reading/v1/<新批的某张图>.webp
# 最后用眼睛看一眼（断言过了和页面好看是两回事）：
(cd apps/lite-web && node e2e/shootRoom.mjs /tmp/shots <新批的某个 slug> 2)
```

改内容改的是 `sources.json` / `tags.json` / `corrections.json` 和源 markdown，
**不是手改 `apps/api/internal/library/articles.json`** —— 那是生成物。

## 源在哪

`sources.json` 说了算。目前两批，都在 `docs/reference/` 下面，
**整个 `docs/reference/` 是 gitignore 掉的**：原图上百 MB，而我们要的产物
已经在库里了。

两批的目录形状不一样，脚本不在乎：

```
batch-1        reading-database/*.md              +  reading-database/images/
batch-0910     <slug>/<slug>-930L.md              +  <slug>/images/
               <slug>/MD/<slug>-930L.md           +  <slug>/MD/images/     ← 也行
```

一篇文章的 slug 和它的难度都来自**文件名**（`<slug>-930L.md` / `<slug>-MAX.md`），
放在第几层没有关系。文件名里没有难度后缀就直接报错，不悄悄跳过 ——
那正是一份导坏了的语料长的样子。

> **在 worktree 里跑**：`docs/reference/` 只存在于主检出。先把两个目录软链进来，
> 否则 `parse.py` 第一句就报 "no source corpus at …"。

## 每个文件做什么

| 文件 | 做什么 |
|---|---|
| `sources.json` | 注册了哪几批语料。**加一批只改这里**，其余脚本都从它读 |
| `sources.py` | 遍历那几批：`markdown_files()` 给出 (批, slug, 档, 路径)，`image_files()` 给出图 |
| `parse.py` | markdown → 每篇一条、每条几档。正文与图分开，图注拆成图注 + 署名，小标题脱掉 `#`，署名行摘出来，段 id 按 Go 的 `SplitBlocks` 规则算 |
| `tags.json` | 中文标题、一句话理由、学科（必须在 disciplines.json 的闭表里）；外加 `holds`：语料里有、但先不上线的，写清为什么 |
| `corrections.json` | `replacements` 是逐字改掉的错处（正文 / 图注 / 署名都过一遍，`count` 对不上就失败）；`figure_captions` 是整条换掉一张图的图注 |
| `make_images.py` | 缩到 1600px 的 WebP，写 `dist/images.json`（对象键 + 尺寸）。两批共用一份清单，重名直接失败 |
| `build.py` | 把上面几样合成 `apps/api/internal/library/articles.json` |
| `audit_captions.py` | **三把尺子里最准的一把**：同一张图的图注在几档里对不上，就是坏掉的地方。整行没导出来、两栏被拼串标成「必坏」，剩下的按相似度排，人读完 |
| `audit_units.py` | 查英制/公制换算对不对（找出了 155 英尺写成 477 米那一处） |
| `audit_names.py` | 查同一篇的几个版本里有没有两种写法的人名地名 |
| `smoke_prod.sh` | 拿一个新注册的学生走一遍线上：书架 → 开一篇 → 正文 + 图 |
| `probe_recommend.sh` | 往线上一个一次性账号的树里种一个词，验推荐真的跟着树走 |
| `probe_cdn.sh` | 验一张图签名能读、不签名 403 |
| `probe_routes.sh` | 分辨「路由没了」和「这个账号的学校不是轻量版」——两者都回 404 |

界面这一侧的走查在 `apps/lite-web/e2e/library-walk.spec.ts`，
截图脚本在 `apps/lite-web/e2e/shootRoom.mjs`。

## 构建会在哪些地方直接失败

这些都是故意的：一批语料出问题的方式，几乎都是「照样跑完、结果悄悄少了点东西」。

- 文件名没有难度后缀
- 同一个 slug 出现在两批里
- 文件名说 930L、`**Level:**` 那行说别的
- 两批有同名的图（对象键由文件名派生，会互相覆盖）
- 一篇文章既不在 `articles` 里也不在 `holds` 里
- `tags.json` 里有一条，语料里却没有这篇（sources.json 路径打错、文件没拷全、
  目录被走漏 —— 少了这一条，构建会照样成功，只是书架上少一篇）
- 一篇文章的学科不在 disciplines 闭表里
- `corrections` 的 `count` 与实际命中次数不符
- `figure_captions` 里有一条谁也没命中
- 某一档没有对应的处理后图片
- 原文那一档没有题图（书架封面取的就是它）

## 线上跑这些脚本要注意

**注册要用轻量版体验班的 join code `G624-UXFE`。** 线上的 Demo School 是
`edition = 'pro'`，拿 `DEMO-0001` 注册出来的账号打任何一条轻量版的路都回 404 ——
包括 `/api/v1/readings`，看上去像是接口没了。

## 署名（2026-09-10 上线）

第二批每篇每档都带一行「By Associated Press, adapted by Newsela staff」。
它**必须**从正文里摘出去 —— 不摘就变成 `b1`，会被当成文章第一句引给学生。
摘出来之后摆在阅读室标题下面那行「来源 · …」（那个位置本来就在，一直是空的）。

按档存，不是按篇：原文那一档署的是记者本人，四个简写档署的是媒体加改写方，
两种说法各自对应它所在的那一版。第一批语料的导出里没有署名行，那 20 篇是空串，
界面据此整行不显示。

🚨 **它不存在 `reading_source` 上，是每次现查目录的**（凭 `reading` 上已有的
`library_slug` + `library_tier`）。为一列文本再动一次 `reading_source` 不值得 ——
那张表 pro 也在用。顺带的好处：改一次署名只要重生成 `articles.json`，不必回头
改已经开出去的每一条阅读记录。她自己粘进来的文章没有署名，我们不知道是谁写的，
编一个比不写更糟。

## 已知的、留着没做的

- **少一档的文章先不上线。** `spain-wins-the-world-cup…` 源数据只导出了四档。
  分级名（入门/基础/进阶/高阶/原文）与推荐里的档位都按五档算，所以它进了
  `holds`。补齐那一档，把 holds 里那条删掉，重跑 `build.py` 就上线了。
- **小标题也是一个段。** `SplitBlocks` 不认识 Markdown，所以「The Capsule」在
  服务端和正文段落一样是一个 block，陪练理论上可以挑它当锚点。真发生的话它会
  引一个三个词的「段落」，不好看但不会坏。要修就得给 block 加类型，那会动到
  pro 共用的 `MaterialBlock`。
- **`###` 与 `##` 都当成小标题，深度不留。** 第二批用 `###` 分节，`##` 只在
  正反方那一篇上分两半。阅读室只有一种小标题，所以深度没有往下传。
- **库里的文章没有可以打开的原文链接**（`sourceUrl` 是空的）：我们排的是自己
  的版本，给一条打不开的链接比不给更糟。

## 上一批（2026-09-10）实际发生了什么

29 篇进来，28 篇上线。挖出来的东西，下一批大概率还会遇到：

1. **目录形状换了。** 第一批平铺，第二批每篇一个目录，其中一篇还多一层 `MD/`。
   → 脚本改成递归，形状不再是一个参数。
2. **小标题的记号换了。** 第一批 `##`，第二批 `###`。原来的正则只认 `##`，
   于是 373 行小标题会**原样带着 `###` 印在正文里**。
3. **多了一行署名。** 见上一节。
4. **署名标记词多了四种。** 原来只认 `Photo:` / `Map:` / `Graphic:`；这一批还有
   `Photo credit:`、`Image:`、`Art:`、`Drawing:`、`Photo from …`、`(AP Photo/…)`。
   认不出来不会报错，只是署名**粘在图注末尾**，读起来像句子的一部分。
5. **PDF 把两栏拼串了。** 菲尔兹奖那篇有四处：一处图注只剩最后四个词，一处
   整行没导出来，两处把隔壁那栏的半句插了进来。**同一张图在五个难度档里的图注
   应该一致 —— 不一致的地方就是出错的地方**，这是找它们的办法。
6. **一篇文章的两张图只导出了一张**，而四个难度档配的是没导出来那张的图注。
   照原样上线，学生会看到一张晾晒土豆的照片配着「印加古道的高角度俯瞰」。
   **对不上的时候要真的打开图看一眼**，别按多数票定。
7. **署名被盖了两遍**，两遍还不完全一样（`/ Washington Post` 对
   `/The Washington Post`）。
8. **两处 OCR 拼错的专名**（`LeBLancs`、`Cinncinnati`）和**一处人名两种写法**
   （`Taymoor` / `Taymour`）—— `audit_names.py` 报的一百多条里绝大多数是
   单复数和族称，得逐条看上下文。`Hetal Patel` 看着像 `Metal` 的 OCR 错，
   其实是真名，差点被改掉。
9. **`library_test.go` 里写死了「20 篇」**，加一批就红。改成了下限。
10. **第 5 条那个办法值得做成脚本。** 事后补的 `audit_captions.py` 第一次跑，
    就在**第一批已经上线的**语料里翻出两处：`climates corps`（五档错三档）和
    `almost 200- year-old`（连字符后断行，五档错三档）。两处都是通顺的英文，
    单看一档谁也看不出来。

## 下一批可以再往前走一步的地方

- 篇数只有下限，**没有「这一批应该是 N 篇」**。加 15 篇进来出了 14 篇，
  现在没有任何一道闸会响。
- 三把尺子都是「报给人看」，**构建不读它们**。跳过全部三条，构建照样全绿。
- `parse.py` 算了 `title_variants` 和 `source_word_count`，**build.py 一个都没用**。
  后者本可以和重新数出来的词数对一下，抓「正文被截断」。
- 同一篇换个 slug 再来一次，**查不出来**（重复只按 slug 跨批查）。
- `smoke_prod.sh` 只真的下载了第一篇的封面和一篇文章的图；别的文章传坏了，
  要等学生点进去才知道。
