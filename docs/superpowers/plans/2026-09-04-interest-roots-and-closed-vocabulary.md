# 实施计划 · 兴趣树的根 + 领域闭表 + 采集改任务

Spec: `docs/superpowers/specs/2026-09-04-interest-roots-and-closed-vocabulary-design.md`
分支 `worktree-interest-roots`。四期顺序做，每期跑一次测试再进下一期。

---

## B1 · 领域词表

- [ ] `packages/contracts/interests/interests.json` —— 约 240 条，七根主枝分布不均
      （formal 薄，making / arts 厚）。每条：`id` `field` `zh` `en` `aliases`
      `disciplines[]` `asks`。
- [ ] `apps/api/internal/interests/interests.json` —— 逐字节副本。
- [ ] `apps/api/internal/interests/interests.go` —— `go:embed`，`ByID` / `All` /
      `IDs` / `Exists` / `PromptList`。
- [ ] 测试：
      - `TestEmbeddedCopyMatchesSourceOfTruth`（逐字节）
      - `TestEveryDisciplineIDIsReal`（打错 id 不会编译报错）
      - `TestIDsAndAliasesAreUnique`
      - `TestEveryFieldIsReal`
      - `TestEveryEntryHasDisciplines`（至少 2 条边）

**验收**：`go test ./internal/interests/...` 绿。其余一行没改，行为零变化。

---

## B2 · 采集换闭表

- [ ] `internal/interest/harvest.go`：prompt 换成「从表里选」，`Harvested` 加
      `InterestID`，`ParseHarvestReply` 加「id 必须在表里」。
- [ ] `internal/interest/router.go` + 其测试 **删除**（`RouteCheap` / `Known` /
      `BuildRoutePrompt` / `ParseRouteReply`）。
- [ ] `internal/api/interest.go`：`routeKeywords` / `routeByModel` 换成查表写边
      （`how='catalog'`、`confidence=1`）；`plantKeywords` 从词表取 `zh/en/field`，
      不再信模型给的。
- [ ] `internal/api/interest_harvest.go`：`ClassCompose` 改 `ClassDigest`。
- [ ] `internal/api/interest_quiz.go`：同一条种词路径，跟着换。
- [ ] 迁移 `0134_interest_vocabulary.sql`：`how` 的 CHECK 加 `'catalog'`；
      `interest_keyword` 加 `interest_id text`；三张表归档 + 清空；
      `atom.interest_harvested_at` 置 NULL。
- [ ] `queries/interest.sql` 跟着改，跑 sqlc（**pin `@v1.27.0`**）。

**验收**：`go test ./internal/interest/... ./internal/api/...` 绿。

---

## B3 · 采集改任务

- [ ] `internal/jobs/`：river client 封装、`HarvestAtomArgs` + worker、
      `PeriodicJob`（2 分钟）。
- [ ] `queries/interest.sql`：`ListPendingHarvestAtoms`（30 天窗口 +
      每学生 3 个 + 整轮 60 个）。
- [ ] `cmd/api/main.go`：起 river，失败只 log；关服时 `Stop`。
- [ ] `finishReading` / `finishWritingAtom` / `finishProject` 后面挂入队。
- [ ] `getInterestTree` 里的 `harvestPending` 删掉。
- [ ] 测试：配额与窗口的取数、nil client 不 panic、同一 atom 不重复采。

**验收**：`go test ./... -timeout 1800s`（`CGO_ENABLED=0`）绿。

---

## B4 · 树加根

- [ ] `src/tree/geometry.ts`：`ROOT_CURVES`、`SUB_ROOTS`、`pointOnRoot`、
      `leafShape`、`disciplineLayout`（42 个位置，按学科表顺序）。
- [ ] `src/tree/TreeView.tsx`：viewBox 撑到 1100，加根层与学科节点，
      光珠换叶子，加 pick 状态与连线，底部面板。
- [ ] 词表与学科表构建期 import。
- [ ] 逻辑测试：`leafShape` 的叶柄真的落在枝上、`disciplineLayout` 不撞、
      共用学科的计数。
- [ ] **Playwright 看三个状态**：多领域 / 一个领域 / 零领域，各一张截图。

**验收**：`pnpm -F lite-web test` 绿、`build` 过、三张截图人眼确认。

---

## 收尾

- [ ] `git merge` 到 main、push origin。
- [ ] `.deploy-local/deploy.sh` 部署。
- [ ] 线上验证：登录冒烟账号 → `/tree` 看根 → 点叶子 → 点学科。
      🚨 接口在 `mind-api.uni-robot.cn` 那台，别用相对路径打静态站。
