# 文件上传 + 正文解析（PDF / DOCX）— Design

**状态：** 已评审通过（2026-08-15）。**决定：** PDF 用纯 Go（`ledongthuc/pdf`，无 CGO，CPU/内存可控，表格结构尽力）；本期 **PDF + DOCX 都做**；上传上限 30MB。
**目标：** 让「上传文件」真正可读——学生上传 PDF / DOCX，服务端提取正文（含表格尽力保留），切成 blocks，进入阅读室正常打开。**根治 #3 的 422 死路。**

---

## 1. 现状（为什么现在 422）

- 图书馆「上传文件」页签**从不上传文件字节**：`ReadingBlock.tsx:1338` 只用 `file.name` 注册一条 **空 URL、无 material** 的 reference。
- 全应用**没有任何 PDF/DOCX 解析**（`WritingBlock.tsx:1709` 明确「正文解析稍后支持」）。
- 于是点「进入阅读室」落到 `enterReading` 的 `default` 分支（无 material、无 URL）→ **422**。

## 2. 方案总览

复用已有两条成熟管线，只在中间加一个「文件字节 → 正文文本」的解析层：

```
前端选文件
  → 向后端要 OSS 预签名 PUT（新 scope user_doc）
  → 直传 OSS
  → POST .../references/{rid}/ingest-file { objectKey }
      → 服务端下载字节
      → docextract.PDF / docextract.DOCX 提取 (title, text)
      → materialize.Segment(text) → blocks
      → CreateProjectMaterial(Source:"uploaded") + SetReferenceMaterial   ← 与 fetchMaterialForReference 同形
  → 进入阅读室正常打开（material 已就位）
```

**关键：不引入 CGO。** 部署以 `CGO_ENABLED=0` 构建，故解析库必须是纯 Go。

## 3. 解析层 `internal/docextract`（新包）

```go
func PDF(b []byte) (title, text string, err error)
func DOCX(b []byte) (title, text string, err error)
```

- **DOCX（干净、无外部依赖）**：`archive/zip` 解 `word/document.xml`，`encoding/xml` 流式取：
  - `<w:p>` 段落（拼 `<w:t>` 文本）→ 一段；
  - `<w:tbl>` 表格 → **复用 #4 的 `tableToText` 思路**（行 `<w:tr>` 的单元格 `<w:tc>` 用 ` | ` 连成一块）；
  - title 取 `docProps/core.xml` 的 `dc:title`，回退首个非空段。
- **PDF（纯 Go，尽力而为）**：推荐 `github.com/ledongthuc/pdf`（纯 Go，无 CGO）逐页取文本。
  - **已知局限**：纯 Go PDF 提取**基本丢失表格的行列结构**（PDF 里表格是绝对定位的文本片段，没有语义标签）。这是 PDF 本身的难点，非本设计缺陷；表格文字仍会作为文本出现，只是不保证成行。**高保真表格需 MuPDF(go-fitz)=CGO，与构建约束冲突，本期不做。**
  - **扫描/图片 PDF（无文本层）**：提取为空 → 走 §5 的优雅回退（让学生粘贴正文），不报硬错。

## 4. 上传与落库

- **OSS 新 scope `user_doc`**（`oss.go` 的 `ossScopes`）：`gate=gateUser`，`allowedTypes = {application/pdf, application/vnd.openxmlformats-officedocument.wordprocessingml.document}`，`maxBytes ≈ 30<<20`，`prefix = users/{uid}/docs/`。
- **新端点** `POST /api/v1/projects/{id}/references/{rid}/ingest-file`（body `{objectKey}`）：
  - 校验 objectKey 属于该 uid 的 `users/{uid}/docs/` 前缀（防越权读他人对象）。
  - 下载字节（OSS get），按 content-type/扩展名选 `PDF`/`DOCX`。
  - `materialize.Segment` → blocks；空则 422 `empty_body`（前端已有粘贴回退）。
  - `CreateProjectMaterial{Kind:"article", Source:"uploaded", SourceUrl:&objectKey}` + `SetReferenceMaterial` —— 事务与 `fetchMaterialForReference:889-931` 同形。
- **`enterReading` 顺带修一个既存小瑕疵**：无 material 的 422 分支目前写裸 `{"error":...}`，应改成标准 `httpx.WriteError` 信封（`fetch_failed`/`no_readable_content`），让前端的粘贴回退稳定触发（#3 报告里的 envelope 不一致点）。

## 5. 前端「上传文件」页签重做

- 选文件 → `api.presignDocUpload({filename,contentType,size})` → PUT 到 OSS → `api.ingestReferenceFile(projectId, refId, objectKey)`（内部先 `createReference` 再 ingest，或后端一步建 ref+material）。
- 上传中/解析中显示进度；解析为空或失败 → 复用现有 `NoReadableContentError` 的**粘贴正文**回退框（学生可直接贴）。
- 大小/类型前端预校验，与 scope 一致。

## 6. 影响文件

- **新**：`apps/api/internal/docextract/{pdf.go,docx.go,*_test.go}`；`apps/api/internal/api/ingest_file.go`。
- **改**：`oss.go`（`user_doc` scope + `gateUser` 复用）、`api.go`（路由）、`workspace_library.go`（`enterReading` 信封）、`materialize` 可复用。
- **前端**：`ReadingBlock.tsx`（上传页签真正上传）、`api/*`（presign + ingest-file 客户端）。
- **依赖**：`go get github.com/ledongthuc/pdf`（纯 Go）。DOCX 用标准库。

## 7. 明确不做

- 不做 OCR（扫描件无文本层 → 回退粘贴）。
- 不做高保真 PDF 表格重建（需 CGO/MuPDF）；表格文字尽力保留，结构不保证。
- 不做花哨文档编辑（铁律：我们只占「思考」层，不拥有成品文档）。

---

**评审问题：**
1. PDF 库确认用纯 Go 的 `ledongthuc/pdf`（接受表格结构会丢）？还是你愿意为高保真表格接受 CGO/MuPDF（需改构建）？
2. DOCX 是否本期就要（还是先只做 PDF）？
3. 上传大小上限 30MB 可以吗？
