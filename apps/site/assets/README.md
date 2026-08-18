# 营销站素材 · site media

这个目录里的文件**不进 git**（只有本 README 进）。素材的存放地是
`mind-open` 这个对象存储桶，通过 `mind-assets.uni-robot.cn` 这个 CDN 域名对外提供。

> 注意：这套存储与**学生端完全分开** —— 学生端的课程素材在另一个桶、另一个 CDN 域名、
> 另一对密钥（走 Go API 的 `/oss/*`）。两边不要混用。

## 放一张图上线，三步

```bash
# 1) 把文件放进来。目录结构 1:1 映射成对象 key
#    apps/site/assets/home/course-player.png  ->  home/course-player.png
mkdir -p apps/site/assets/home
cp ~/Desktop/截图.png apps/site/assets/home/course-player.png

# 2) 上传（凭据来自 .deploy-local/site-oss.env，git 忽略）
deploy/upload-site-assets.sh

# 3) 在页面里引用
#    <Figure asset="home/course-player.png" name="..." caption="..." />
```

改完页面提交并 `./.deploy-local/deploy-site.sh` 即可。

## 命名规则

CDN 按 URL 缓存。**要换一张图，请换一个 key**（`hero-v2.png`），不要覆盖同名文件后
干等缓存过期。

## 什么放这里，什么放 `public/`

- **放这里（走 OSS/CDN）：** 图片、视频、PDF —— 内容素材，体积大、会换、和代码无关。
- **放 `apps/site/public/`：** 界面本身的一部分 —— 图标、装饰用的 SVG。首屏
  不能等一次网络往返，所以它们跟着站点一起发布。
  例：`public/media/explorer-v1.svg`（首屏那条船）被构建期内联进页面。

## 已上线的 key

| key | 内容 |
|---|---|
| `home/hero-bg-v3.webp` | 首屏背景（当前使用）。深蓝绿色的水流 |
| `home/hero-bg-v4.webp` | 首屏背景备选。换用改 `HeroImmersive.astro` 里的 `assetUrl()` |

> 首屏背景是**叠加**的：底下那层水是纯 CSS 画的，图片只是盖在上面。CDN 没通、
> 图片没到，首屏依然是完整的，不会开天窗。

## 已经在线的图

| key | 用在哪 |
|---|---|
| `home/hero-bg-v3.webp` | 首屏 |
| `banner/*-v1.webp` | 各页横幅（about / algorithm / evaluation-design / multi-agent / course / project / evaluation / teacher / product-overview） |
| `ability/*-v1.webp` | 首页「四项能力」四张背景插画 |
| `product/course-runtime-v1.webp` | `/product/courses`，以及总览案例第 1 步 |
| `product/framing-v1.webp` | `/product/projects` 立题阶段，以及案例第 2 步 |
| `product/warren-map-v1.webp` | `/product/projects` 兔子洞地图，以及案例第 4 步 |
| `product/writing-v1.webp` | `/product/projects` 写作面，以及案例第 5 步 |
| `product/evaluation-report-v1.webp` | `/product/evaluation`，以及案例第 6 步 |
| `evaluation/report-sample-v1.webp` | `/algorithm/evaluation` 示例报告首页 |
| `evaluation/report-sample-v1.pdf` | 同页「查看完整报告」按钮（12 页） |

## 当前待补的图

页面里已经排好版、等素材的槽位（`<Figure name="...">` 里的名字）：

| key 建议 | 内容 |
|---|---|
| `product/reading-provenance.png` | 阅读室：原文 + 溯源链（公众号 → NASA → Nature）+ 证据地图 |
| `product/teacher-console.png` | 教师端：实时名单 + 学生摘要 + 可点开的证据链接 |
| `team/ceo.jpg`、`team/cto.jpg` | 两位联合创始人人像，4:5 竖版 |
| `banner/multi-agent-v2.webp` | 多智能体页横幅（现暂用 hero v4 左右镜像） |

白皮书 `docs/reference/思维印记白皮书.docx` 里内嵌了 30 张图（CRAAP 模型、SIFT×CRAAP、
信源金字塔、WEF 图表、地球变绿的几张截图），其中不少可以直接拿来用。
