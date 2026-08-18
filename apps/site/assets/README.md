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
| `banner/course-v1.webp` | `/product/courses` |
| `banner/project-v1.webp` | `/product/projects` |
| `banner/evaluation-v1.webp` | `/product/evaluation` |
| `banner/teacher-v1.webp` | `/product/teacher` |
| `banner/algorithm-v1.webp` | `/algorithm` |
| `banner/evaluation-design-v1.webp` | `/algorithm/evaluation` |
| `banner/multi-agent-v1.webp` | `/algorithm/multi-agent`（暂用 hero v4，CSS 里左右镜像） |
| `banner/about-v1.webp` | `/about` |
| `ability/*-v1.webp` | 首页「四项能力」四张背景插画 |

## 当前待补的图

页面里已经排好版、等素材的槽位（`<Figure name="...">` 里的名字）：

| key 建议 | 内容 |
|---|---|
| `product/course-runtime.png` | 课程播放界面全景 |
| `product/reading-provenance.png` | 溯源链 公众号 → NASA → Nature |
| `product/writing-studio.png` | 写作面全景 |
| `product/evaluation-report.png` | 报告界面：左侧刻度目录 + 双轴可视化 |
| `product/teacher-console.png` | 教师端：叙述版周报 + 实时名单 |
| `evaluation/report-sample.png` | `/algorithm/evaluation` 页尾的完整示例报告 |
| `team/<name>.jpg` | 团队人像，4:5 |

白皮书 `docs/reference/思维印记白皮书.docx` 里内嵌了 30 张图（CRAAP 模型、SIFT×CRAAP、
信源金字塔、WEF 图表、地球变绿的几张截图），其中不少可以直接拿来用。
