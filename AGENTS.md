# `sub2api` Codex 指南

`sub2api` 是应用源代码仓库，包含 Sub2API 服务的后端、前端、迁移文件和 Docker 镜像输入。生产部署由相邻的 `../sub2api` 运维仓库管理。

## 优先阅读

- `README_CN.md`：项目概览和中文安装说明。
- `README.md` / `README_JA.md`：英文和日文项目入口，保留给对应语言用户。
- `DEV_GUIDE.md`：开发工作流和常见坑点。
- `package.json`：前端脚本、依赖和测试命令。
- `../sub2api/AGENTS.md`：当改动需要部署，或影响生产运维时阅读。

## 架构地图

- `backend/`：Go 后端。
- `backend/cmd/server/`：应用入口和依赖装配。
- `backend/internal/handler/`：HTTP/API handlers 和路由行为。
- `backend/internal/repository/`：持久化、缓存、运维查询和外部集成。
- `backend/internal/pkg/`：上游兼容、OAuth helpers、校验和共享包。
- `backend/migrations/`：SQL 迁移；涉及破坏性变更时必须谨慎审查。
- `backend/ent/`：生成的 Ent 代码；除非本地流程明确要求，不要手工编辑。
- `frontend/`：Vue 3 + Vite 前端。
- `frontend/src/views/`：管理员、用户、认证等页面级视图。
- `frontend/src/components/`：可复用 UI 和功能组件。
- `frontend/src/stores/`：Pinia stores。
- `frontend/src/router/`：路由和守卫。
- `deploy/`：应用级部署示例和 Docker 支持，不是生产运维事实来源。
- `Dockerfile`、`backend/Dockerfile`、`Dockerfile.goreleaser`：镜像构建输入。

## 工具链

- Go 版本：`1.26.4`（见 `backend/go.mod`）。
- 前端：Node + pnpm、Vue 3、Vite、TypeScript、Vitest。
- 后端测试和构建通常在 `backend/` 下运行。
- 前端测试和构建通常在 `frontend/` 下运行。

## 常用命令

仓库根目录：

```powershell
make build
make test
make test-backend
make test-frontend
```

后端：

```powershell
cd backend
go test ./...
```

前端：

```powershell
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run <test files>
pnpm --dir frontend run build
```

## 实现规则

- 添加新抽象前，先遵循现有本地模式。
- 保持 Ent 生成代码和迁移文件与仓库约定一致。
- SQL 迁移避免破坏性语句，除非用户明确要求并已记录影响。
- 如果迁移可能包含 `DROP TABLE`、`DROP COLUMN`、大范围 `DELETE` 或有损类型变更，必须提示运维部署确认。
- 不要把生产密钥放进本仓库。
- 不要把 `sub2api/deploy/docker-compose*.yml` 当作生产事实来源。生产 compose 位于 `../sub2api/deploy/docker-compose.yml`。

## 用户确认流程

当用户提出需求但没有明确要求立即执行时，先给出方案和简短步骤计划，等待用户确认后再行动。

- 这适用于仓库写入、commit、push、pull、reset、stash、依赖变更、服务变更、部署，以及其它会改变本地或远程状态的操作。
- 为制定方案而执行只读检查是允许的，但在请求确认前要说明检查了什么。
- 如果界面支持可点击或悬浮确认选项，优先使用这种确认方式，而不是要求用户自由输入。
- 将“直接执行”“继续”“应用这个改动”“进行修改”或同等含义的中英文表达视为对已描述动作的确认。

## 共享仓库规则

新增或修改非项目专属的协作规则时，需要同时更新两个仓库的指南文件：

- `AGENTS.md`
- `../sub2api/AGENTS.md`

在不降低正确性、工具兼容性或项目功能的前提下，尽量使用中文。这包括解释、计划、commit 信息和面向用户的协作说明。代码标识符、命令、协议名、环境变量以及已有英文项目术语，如果翻译会误导或造成损害，应保持原样。

如果同一份 Markdown 文档存在多语言版本，例如 `README.md`、`README_CN.md`、`README_JA.md`，只修改中文版本，其它语言版本保持不动。

## Git 管理流程

Commit 应表示一个完整意图，而不是一次对话。多次相关对话如果属于同一个逻辑变更，并且可以一起审查或回滚，可以合并为一个 commit。

- 除非用户明确要求 commit，或确认了提交计划，否则不要创建 commit。
- 提交前检查 `git status` 和 `git diff`，按主题归类改动，并说明建议使用一个还是多个 commit。
- Commit message 必须使用中文，并清楚描述改动内容、动机和影响。
- 推荐格式：`类型：简短说明`，正文用要点说明改了什么以及为什么改。
- 无关事项应拆成不同 commit，尤其是文档、部署行为、安全门禁、镜像更新和源代码改动。
- Commit 后不要默认 push，除非用户明确要求或确认 push。
- Push 前说明目标 remote/branch，以及本地历史相对远程是 ahead、behind 还是 diverged。

## 版本与分支管理流程

版本与分支管理以 `../sub2api/docs/version-management.md` 为权威说明。

- 使用 `main` / `dev/<version>` / `release/<version>` / `v<version>` 模型。
- 两个仓库使用同一个版本号；版本可以只改一个仓库，但必须记录两个仓库参与部署的 commit/tag。
- 版本号是两个仓库共享的全局单调递增序列；只要任一仓库的本地/远程 `dev/*`、`release/*`、`v*` tag，或运维仓库 `docs/releases/<version>.md` 使用过，该版本号即占用，后续版本必须使用全局最大已占用版本号 + 1，未参与某版本的仓库不能补用旧号。
- 版本状态只允许：`开发中`、`已提测`、`成功`、`失败`、`取消`。
- 本仓库是 fork；upstream 同步必须由用户主动提出，并作为版本内容进入 `dev/<version>`，不能直接同步到 `main`。
- 应用代码改动必须先进入 `dev/<version>`；`release/<version>` 只接收从 `dev/<version>` 同步过来的候选内容，不作为日常开发入口。
- 同一批逻辑改动只能沿一条提交链从 `dev/<version>` 流向 `release/<version>` 再流向 `main`；不要在多个分支分别重新提交或重复 cherry-pick 同一补丁。
- 如因生产紧急修复直接改了 `release/<version>`，部署成功后必须立即把同一提交回填到 `dev/<version>`，确保开发分支不落后于已上线代码。
- `dev`、`release`、`main` 之间同步优先使用 fast-forward；如果 fast-forward 失败，先检查分叉原因和重复补丁，再决定合并、删除或归档分支。
- 本地 Docker 验证和服务器部署必须使用同一个 release commit、compose commit 和 immutable image digest。
- 生产候选镜像只能从 `release/<version>` 构建；`main` push 不作为生产候选镜像来源。
- 部署成功后的 `v<version>` tag 是归档点，不得用 tag 触发的新构建替换已验证通过的 digest。
- `release.yml` 只做 GitHub Release 和二进制归档，不重新构建 Docker 镜像；镜像版本 tag 使用 `Promote Verified Image` workflow 从已验证 digest 提升。
- `backend/cmd/server/VERSION` 如需更新，必须作为版本内容进入 dev/release 分支；发布归档 workflow 不应在部署后向 `main` 追加 VERSION commit。
- 每个成功版本必须在两个仓库都创建 `v<version>` tag。
- 每个关键阶段必须按运维仓库版本管理文档的检查点执行。
- 部署成功后，如果 `dev/<version>` 分支仍保留，必须将最新 `main` fast-forward 回该开发分支，或明确删除/归档该开发分支。

## 需要部署时

应用代码变更不是通过复制源码到服务器部署。正常路径是：

1. 将本仓库 `dev/<version>` 同步到 `release/<version>`。
2. 推送 `release/<version>`，由 GitHub Actions 构建候选 Docker 镜像并推送到 registry。
3. 等待 `GHCR Image` workflow 成功，从 Summary 复制 immutable image digest，并写入版本记录。
4. 在 `../sub2api/deploy/docker-compose.yml` 中更新到同一个 digest。
5. 使用同一个 release commit、compose commit 和 image digest 完成本地 Docker 验证与服务器验证。
6. 从 `sub2api` 运维仓库执行 Git 驱动的运维部署。

如果新机器上缺少相邻运维仓库，在做生产部署工作前，先将 `git@github.com:zeroliao/zero007-sub2api-ops.git` clone 到本仓库旁边并命名为 `sub2api`。

## 部署确认规则

除非用户在同一请求中明确要求部署，或在被询问后确认部署，否则不要在代码变更后部署。

- 当用户说出“改完并部署”“直接部署”“自动部署”“部署到服务器”或同等含义表达时，可视为直接部署授权。
- 如果用户只是要求实现、修复、优化或准备变更，本地验证后停止，并在运行任何生产部署命令前询问。
- `validate-candidate`、`doctor`、`status`、`active-slot` 和 `logs` 可作为检查命令执行。
- `deploy`、`bluegreen-deploy`、`start-deploy` 和 `start-bluegreen-deploy` 需要明确部署意图或确认。
