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
| `product/evaluation-report-v1.webp` | `/product/evaluation` 报告 |
| `case/01-course-v1.webp` … `07-evaluation-v1` | `/product` 那条案例主线；`-v1.mp4` + `-v1-poster.webp` 成对的是会动的 |
| `evaluation/report-sample-v1.webp` | `/algorithm/evaluation` 示例报告首页 |
| `evaluation/report-sample-v1.pdf` | 同页「查看完整报告」按钮（12 页） |
| `courses/cover/01-v1.webp` … `33-v1.webp` | 课程库 33 张封面，按课程序号编号（`CoursesPage.astro` 里 `courseList` 的 `n`） |
| `cards/<card-id>-v3.webp` | 图鉴 34 张卡面。id 取自 `packages/contracts/cards/*.json` 的文件名，只用第 1 面（原始素材的 `-1`） |
| `product/card-craap-v1.webp`、`product/card-pee-v1.webp` | `/product/courses`「点开一张卡，看见的是什么」两张卡详情 |
| `product/teacher-week-v1.webp` | `/product/teacher` 班级周报 |
| `product/teacher-student-v1.webp` | `/product/teacher` 单个学生页 |
| `team/chen-yujie-v1.webp` 等 4 张 | `/about` 团队人像，**方形**（页面裁成圆形）。文件名用姓名拼音，不用交付来的原始文件名 |

> 交付来的原图是印刷尺寸（卡面 1024×1536、课程封面 1664×936），页面上只显示 200–400px。
> 上传前一律用 `cwebp -q 80 -resize <2 倍显示宽> 0` 压过：34 张卡面 5.3MB → 1.4MB，
> 33 张封面 5.7MB → 1.3MB。别把印刷尺寸的图直接推上 CDN。

## 当前待补的图

页面里已经排好版、等素材的槽位（`<Figure name="...">` 里的名字）：

| key 建议 | 内容 |
|---|---|
| `team/yang-xinsong-v1.webp` | 杨欣松人像，方形。没有照片时页面显示姓氏字母，不会开天窗 |
| `banner/multi-agent-v2.webp` | 多智能体页横幅（现暂用 hero v4 左右镜像） |

阅读室、教师端与团队人像都已于 2026-08-25 补齐（见上表），页面上不再有空槽。

**桶里还有几个已经没人引用的 key**：`product/course-runtime-v1.webp`、`product/framing-v1.webp`、
`product/warren-map-v1.webp`、`product/writing-v1.webp`。`/product/projects` 的四张图在
2026-08-25 换成了 `case/` 里录到的同一批真实素材（立题 / 阅读室 / 兔子洞 / 写作面），这几张
静态图就此闲置。留着不碍事，但要改这几个页面时别再往它们上面接。

白皮书 `docs/reference/思维印记白皮书.docx` 里内嵌了 30 张图（CRAAP 模型、SIFT×CRAAP、
信源金字塔、WEF 图表、地球变绿的几张截图），其中不少可以直接拿来用。
