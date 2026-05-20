# 多电脑 Codex 接入与发布说明

本文档说明如何在一台新电脑上获得和当前电脑相同的能力：

- 修改 `sub2api` 源码功能。
- 修改生产部署配置。
- 通过 Codex 自动验证、备份、部署和回滚。

## 仓库分工

源码仓库：

```text
git@github.com:zeroliao/sub2api.git
```

本地建议路径：

```text
C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api-src
```

用途：

```text
Go 后端代码
Vue 前端代码
Dockerfile
测试和构建配置
```

运维仓库：

```text
git@github.com:zeroliao/zero007-sub2api-ops.git
```

本地建议路径：

```text
C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api
```

用途：

```text
docker-compose.yml
部署脚本
备份和回滚脚本
健康检查
服务器 compose 基线
```

不要提交：

```text
.env.ops
.env
密码
token
私钥
数据库数据
```

## 新电脑准备 SSH Key

在新电脑 PowerShell 中生成 SSH key：

```powershell
ssh-keygen -t ed25519 -C "codex-new-pc" -f $env:USERPROFILE\.ssh\codex_new_pc
```

查看公钥：

```powershell
Get-Content $env:USERPROFILE\.ssh\codex_new_pc.pub
```

把公钥添加到 GitHub：

```text
GitHub -> Settings -> SSH and GPG keys -> New SSH key
```

把同一行公钥添加到服务器：

```text
/home/ubuntu/.ssh/authorized_keys
```

服务器上可执行：

```bash
mkdir -p /home/ubuntu/.ssh
chmod 700 /home/ubuntu/.ssh
echo '把新电脑公钥整行放在这里' >> /home/ubuntu/.ssh/authorized_keys
chown -R ubuntu:ubuntu /home/ubuntu/.ssh
chmod 600 /home/ubuntu/.ssh/authorized_keys
```

## 克隆两个仓库

安装 Git 后，在新电脑执行：

```powershell
git clone git@github.com:zeroliao/zero007-sub2api-ops.git C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api
git clone git@github.com:zeroliao/sub2api.git C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api-src
```

如果 SSH key 不是默认 key，为两个仓库绑定专用 key：

```powershell
git -C C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api config core.sshCommand "ssh -i C:/Users/Administrator/.ssh/codex_new_pc -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"

git -C C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api-src config core.sshCommand "ssh -i C:/Users/Administrator/.ssh/codex_new_pc -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
```

## 创建本机 .env.ops

在运维仓库中创建：

```text
C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api\.env.ops
```

内容模板：

```text
SUB2API_SSH_HOST=43.133.231.45
SUB2API_SSH_USER=ubuntu
SUB2API_SSH_PORT=22
SUB2API_REMOTE_DIR=/opt/sub2api-deploy
SUB2API_HEALTH_URL=https://api.zero007.chat/health
SUB2API_PROJECT_NAME=sub2api-deploy
SUB2API_SSH_KEY=C:\Users\Administrator\.ssh\codex_new_pc
```

`.env.ops` 是本机私有文件，不要提交。

## 验证新电脑接入

测试服务器 SSH：

```powershell
ssh -i C:\Users\Administrator\.ssh\codex_new_pc ubuntu@43.133.231.45
```

能登录后退出，再测试运维脚本：

```powershell
cd C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api
.\scripts\sub2api-ops.cmd status
.\scripts\sub2api-ops.cmd diff-server
```

如果 `status` 和 `diff-server` 都成功，新电脑就具备自动运维能力。

## 修改部署配置

修改部署配置时使用运维仓库：

```text
C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api
```

标准流程：

```text
git pull
分配或确认版本号
在运维仓库创建 dev/<version>
修改 deploy/docker-compose.yml 或脚本
提交并同步到 release/<version>
diff-server
本地 Docker 验证或交给有 Docker 的电脑验证
验证通过后标记版本状态为已提测
validate-candidate
backup
start-bluegreen-deploy 或 bluegreen-deploy
healthcheck/logs
成功后合入 main 并打 v<version> tag
失败 rollback，保留 release/<version> 排查
```

## 修改源码功能

修改功能时使用源码仓库：

```text
C:\Users\Administrator\Desktop\code\sub2api-wrap\sub2api-src
```

标准流程：

```text
git pull
分配或确认版本号
创建 dev/<version>
修改后端或前端代码
运行相关测试
同步到 release/<version>
推送 release/<version>，由 GitHub Actions 构建候选镜像
等待 GHCR Image workflow 成功，并从 Summary 记录 immutable image digest
回到运维仓库 release/<version> 更新 compose digest
推送运维仓库 release/<version>
diff-server
本地 Docker 验证或交给有 Docker 的电脑验证
验证通过后标记版本状态为已提测
validate-candidate
backup
start-bluegreen-deploy 或 bluegreen-deploy
healthcheck/logs
成功后合入 main 并打 v<version> tag
如需要镜像版本 tag，运行 Promote Verified Image 标记已验证 digest
失败 rollback，保留 release/<version> 排查
```

源码仓库的 `release/<version>` 触发候选镜像构建，运维仓库使用同一个 digest 完成验证和部署。生产 compose 只使用 `ghcr.io/...@sha256:<digest>`，不要使用 `latest`、`main` 或 `dev` 等可变 tag。

## 节点交接信号

- GitHub 构建镜像完成后，完成信号是 `GHCR Image` workflow 成功，且 Summary 中出现 `ghcr.io/...@sha256:<digest>`。
- 有 Docker 的电脑开始验证前，必须先拉取同一个 `sub2api-src/release/<version>` commit 和 `sub2api/release/<version>` compose commit。
- 本地 Docker 验证完成后，把验证机器、source commit、ops commit、digest 和结果写入版本记录；没有记录不要进入服务器验证。
- `validate-candidate` 成功后再执行 `backup`；`backup` 成功后再执行蓝绿部署。
- 部署成功后再合入 `main`、打 `v<version>` tag、运行发布归档和可选的镜像 tag 提升。

## 多电脑协作规则

每次开始前：

```powershell
git pull
```

每次发布前：

```powershell
.\scripts\sub2api-ops.cmd diff-server
```

如果 `diff-server` 发现服务器 compose 和仓库基线不一致，先停止发布，决定是：

```text
sync-from-server：把服务器当前配置同步回仓库
deploy：用仓库配置覆盖服务器
```

不要在多台电脑上同时执行部署。运维脚本有部署锁，但最好先在沟通上避免并发操作。
