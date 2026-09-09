# AI 网页设计 Demo Gateway

这是一个不依赖数据库或登录系统的本地服务入口。它复用项目现有的
`internal/gateway` 的 Catalog Provider。默认优先使用阿里云百炼
DashScope，未配置时才使用 `sub2api.zbrain.cn`。

## 配置

在 `apps/api/.env.local` 中配置：

```env
DASHSCOPE_API_KEY=你的阿里云百炼 API Key
DASHSCOPE_MODEL=qwen-plus
WEB_DESIGN_DEMO_PORT=8787

# 默认端点：阿里云百炼 OpenAI 兼容模式
# DASHSCOPE_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1

# 如果没有 DashScope Key，才使用 Sub2API
# SUB2API_API_KEY=你的 Sub2API 密钥
# SUB2API_MODEL=你的 Sub2API 模型 ID
# SUB2API_BASE_URL=https://sub2api.zbrain.cn/v1
```

`.env.local` 已被仓库的 `.gitignore` 排除。密钥不会进入浏览器、接口响应或日志。
模型名取决于 Sub2API 后台给当前密钥开放的模型，因此不在代码里猜测默认值。

## 启动

在 `apps/api` 目录运行：

```powershell
go run ./cmd/web-design-demo
```

另开终端，在 `apps/lite-web` 目录启动 Vite：

```powershell
pnpm vite --host 127.0.0.1 --port 8765
```

然后打开：

```text
http://127.0.0.1:8765/src/eco/index.html
```

## 接口

- `GET /healthz`：返回当前 Provider 和模型，不返回密钥。
- `POST /api/v1/web-design/recommend`：真实 AI 需求拆解及三个设计方向。
- `POST /api/v1/web-design/propose-patch`：真实 AI 局部修改提案。
- `POST /api/v1/web-design/generate-design`：根据 ReferenceSpecV1 生成并编译 VisualDesignSpecV1。
- `POST /api/v1/web-design/propose-design-patch`：生成 PatchSpecV1 局部修改。
- `POST /api/v1/web-design/analyze-reference-image`：分析截图视觉信息。

模型只能返回结构化设计数据和受约束的组件修改。服务端会拒绝重复模板、非法分数、
越过当前 Block/组件作用域的修改、HTML/CSS/JavaScript 以及无效组件宽度。
