# 分级阅读库的内容流水线

二十篇报道，每篇五个难度版本，六十张照片。这个目录是从导出的 markdown 到线上
可读文章之间的全部步骤。

## 一次完整的重建

```bash
# 1. 图：177 MB 原图 -> 7.3 MB 的 1600px WebP + 一份对象键清单
python3 deploy/reading-library/make_images.py

# 2. 目录：markdown + tags.json + corrections.json + 图清单 -> articles.json
python3 deploy/reading-library/build.py

# 3. 图传到 OSS（键是确定的，articles.json 里已经写死了它们）
READING_OSS_ENV=.deploy-local/env.prod deploy/upload-reading-images.sh

# 4. 后端重新编译（articles.json 是 go:embed 的），部署
.deploy-local/deploy.sh api
```

改内容改的是 `tags.json` / `corrections.json` 和源 markdown，**不是手改
`apps/api/internal/library/articles.json`** —— 那是生成物。

## 源在哪

`docs/reference/reading-database/`（**gitignore 掉的**，100 个 markdown +
60 张原图）。它不进版本库：原图 177 MB，而我们要的产物已经在库里了。

## 每个文件做什么

| 文件 | 做什么 |
|---|---|
| `parse.py` | 100 个 markdown → 20 篇 × 5 档。正文与图分开，图注拆成图注 + 署名，小标题脱掉 `##`，段 id 按 Go 的 `SplitBlocks` 规则算 |
| `tags.json` | 中文标题、一句话理由、学科（必须在 disciplines.json 的闭表里） |
| `corrections.json` | 逐字改掉的错处，每条写清楚为什么，`count` 对不上就构建失败 |
| `make_images.py` | 缩到 1600px 的 WebP，写 `dist/images.json`（对象键 + 尺寸） |
| `build.py` | 把上面几样合成 `apps/api/internal/library/articles.json` |
| `audit_units.py` | 查英制/公制换算对不对（找出了 155 英尺写成 477 米那一处） |
| `audit_names.py` | 查同一篇的五个版本里有没有两种写法的人名地名 |
| `smoke_prod.sh` | 拿一个新注册的学生走一遍线上：书架 → 开一篇 → 正文 + 图 |
| `probe_recommend.sh` | 往线上一个一次性账号的树里种一个词，验推荐真的跟着树走 |
| `probe_cdn.sh` | 验一张图签名能读、不签名 403 |
| `probe_routes.sh` | 分辨「路由没了」和「这个账号的学校不是轻量版」——两者都回 404 |

界面这一侧的走查在 `apps/lite-web/e2e/library-walk.spec.ts`，
截图脚本在 `apps/lite-web/e2e/shootRoom.mjs`。

## 线上跑这些脚本要注意

**注册要用轻量版体验班的 join code `G624-UXFE`。** 线上的 Demo School 是
`edition = 'pro'`，拿 `DEMO-0001` 注册出来的账号打任何一条轻量版的路都回 404 ——
包括 `/api/v1/readings`，看上去像是接口没了。

## 已知的、留着没做的

- **小标题也是一个段。** `SplitBlocks` 不认识 Markdown，所以「The Capsule」在
  服务端和正文段落一样是一个 block，陪练理论上可以挑它当锚点。真发生的话它会
  引一个三个词的「段落」，不好看但不会坏。要修就得给 block 加类型，那会动到
  pro 共用的 `MaterialBlock`。
- **库里的文章没有可以打开的原文链接**（`sourceUrl` 是空的）：我们排的是自己
  的版本，给一条打不开的链接比不给更糟。
