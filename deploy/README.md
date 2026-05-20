# Sub2API 部署文件

本目录包含在 Linux 服务器上部署 Sub2API 所需的文件。

## 部署方式

| 方式 | 适用场景 | Setup Wizard |
|------|----------|--------------|
| **Docker Compose** | 快速搭建、一体化部署 | 不需要（自动初始化） |
| **Binary Install** | 生产服务器、systemd 托管 | Web 向导 |

## 文件

| 文件 | 说明 |
|------|------|
| `docker-compose.yml` | Docker Compose 配置（named volumes） |
| `docker-compose.local.yml` | Docker Compose 配置（本地目录，便于迁移） |
| `docker-deploy.sh` | **一键 Docker 部署脚本（推荐）** |
| `.env.example` | Docker 环境变量模板 |
| `DOCKER.md` | Docker Hub 文档 |
| `install.sh` | 一键二进制安装脚本 |
| `install-datamanagementd.sh` | datamanagementd 一键安装脚本 |
| `sub2api.service` | Systemd service unit 文件 |
| `sub2api-datamanagementd.service` | datamanagementd systemd service unit 文件 |
| `DATAMANAGEMENTD_CN.md` | datamanagementd 部署与联动说明（中文） |
| `config.example.yaml` | 示例配置文件 |

---

## Docker 部署（推荐）

### 方式 1：一键部署（推荐）

使用自动准备脚本可以最快完成部署：

```bash
# Download and run the preparation script
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash

# Or download first, then run
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh -o docker-deploy.sh
chmod +x docker-deploy.sh
./docker-deploy.sh
```

**脚本会执行：**

- 下载 `docker-compose.local.yml` 和 `.env.example`。
- 自动生成安全密钥（`JWT_SECRET`、`TOTP_ENCRYPTION_KEY`、`POSTGRES_PASSWORD`）。
- 创建包含生成密钥的 `.env` 文件。
- 创建必要的数据目录（`data/`、`postgres_data/`、`redis_data/`）。
- **显示生成的凭据**（`POSTGRES_PASSWORD`、`JWT_SECRET` 等）。

**脚本运行后：**

```bash
# Start services
docker compose -f docker-compose.local.yml up -d

# View logs
docker compose -f docker-compose.local.yml logs -f sub2api

# If admin password was auto-generated, find it in logs:
docker compose -f docker-compose.local.yml logs sub2api | grep "admin password"

# Access Web UI
# http://localhost:8080
```

### 方式 2：手动部署

如果希望手动控制部署过程：

```bash
# Clone repository
git clone https://github.com/Wei-Shaw/sub2api.git
cd sub2api/deploy

# Configure environment
cp .env.example .env
nano .env  # Set POSTGRES_PASSWORD and other required variables

# Generate secure secrets (recommended)
JWT_SECRET=$(openssl rand -hex 32)
TOTP_ENCRYPTION_KEY=$(openssl rand -hex 32)
echo "JWT_SECRET=${JWT_SECRET}" >> .env
echo "TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}" >> .env

# Create data directories
mkdir -p data postgres_data redis_data

# Start all services using local directory version
docker compose -f docker-compose.local.yml up -d

# View logs (check for auto-generated admin password)
docker compose -f docker-compose.local.yml logs -f sub2api

# Access Web UI
# http://localhost:8080
```

### 部署版本对比

| 版本 | 数据存储 | 迁移 | 适用场景 |
|------|----------|------|----------|
| **docker-compose.local.yml** | 本地目录（`./data`、`./postgres_data`、`./redis_data`） | 容易（打包整个目录即可） | 生产环境、需要频繁备份/迁移 |
| **docker-compose.yml** | Named volumes（`/var/lib/docker/volumes/`） | 需要 docker 命令 | 简单搭建、不需要迁移 |

**推荐：** 使用 `docker-compose.local.yml`（由 `docker-deploy.sh` 部署），便于数据管理和迁移。

### 自动初始化如何工作

使用 `AUTO_SETUP=true` 的 Docker Compose 时：

1. 首次运行时，系统会自动：
   - 连接 PostgreSQL 和 Redis。
   - 应用数据库迁移（`backend/migrations/*.sql`），并记录到 `schema_migrations`。
   - 生成 JWT secret（如果没有提供）。
   - 创建管理员账号（如果没有提供密码，会自动生成）。
   - 写入 `config.yaml`。

2. 不需要手动 Setup Wizard；配置 `.env` 后启动即可。

3. 如果没有设置 `ADMIN_PASSWORD`，可在日志中查看生成的密码：

   ```bash
   docker compose logs sub2api | grep "admin password"
   ```

### 数据库迁移说明（PostgreSQL）

- 迁移按字典序应用，例如 `001_...sql`、`002_...sql`。
- `schema_migrations` 记录已应用迁移（filename + checksum）。
- 迁移是 forward-only；回滚需要恢复数据库备份或手动执行补偿 SQL。

**验证 `users.allowed_groups` → `user_allowed_groups` 回填**

在 GORM→Ent 增量迁移期间，`users.allowed_groups`（旧的 `BIGINT[]`）会被规范化的关联表 `user_allowed_groups(user_id, group_id)` 替代。

运行以下查询，对比旧数据和新关联表：

```sql
WITH old_pairs AS (
  SELECT DISTINCT u.id AS user_id, x.group_id
  FROM users u
  CROSS JOIN LATERAL unnest(u.allowed_groups) AS x(group_id)
  WHERE u.allowed_groups IS NOT NULL
)
SELECT
  (SELECT COUNT(*) FROM old_pairs)           AS old_pair_count,
  (SELECT COUNT(*) FROM user_allowed_groups) AS new_pair_count;
```

### datamanagementd（数据管理）联动

如需启用管理后台“数据管理”功能，请额外部署宿主机 `datamanagementd`：

- 主进程固定探测 `/tmp/sub2api-datamanagement.sock`。
- Docker 场景下需把宿主机 Socket 挂载到容器内同路径。
- 详细步骤见：`deploy/DATAMANAGEMENTD_CN.md`。

### 常用命令

**本地目录版本**（`docker-compose.local.yml`）：

```bash
# Start services
docker compose -f docker-compose.local.yml up -d

# Stop services
docker compose -f docker-compose.local.yml down

# View logs
docker compose -f docker-compose.local.yml logs -f sub2api

# Restart Sub2API only
docker compose -f docker-compose.local.yml restart sub2api

# Update to latest version
docker compose -f docker-compose.local.yml pull
docker compose -f docker-compose.local.yml up -d

# Remove all data (caution!)
docker compose -f docker-compose.local.yml down
rm -rf data/ postgres_data/ redis_data/
```

**Named volumes 版本**（`docker-compose.yml`）：

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# View logs
docker compose logs -f sub2api

# Restart Sub2API only
docker compose restart sub2api

# Update to latest version
docker compose pull
docker compose up -d

# Remove all data (caution!)
docker compose down -v
```

### 环境变量

| 变量 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `POSTGRES_PASSWORD` | **是** | - | PostgreSQL 密码 |
| `JWT_SECRET` | **推荐** | 自动生成 | JWT secret，固定后可保持会话长期有效 |
| `TOTP_ENCRYPTION_KEY` | **推荐** | 自动生成 | TOTP 加密密钥，固定后可保持 2FA 长期有效 |
| `SERVER_PORT` | 否 | `8080` | 服务端口 |
| `ADMIN_EMAIL` | 否 | `admin@sub2api.local` | 管理员邮箱 |
| `ADMIN_PASSWORD` | 否 | 自动生成 | 管理员密码 |
| `TZ` | 否 | `Asia/Shanghai` | 时区 |
| `GEMINI_OAUTH_CLIENT_ID` | 否 | 内置 | Google OAuth client ID（Gemini OAuth）。留空则使用内置 Gemini CLI client。 |
| `GEMINI_OAUTH_CLIENT_SECRET` | 否 | 内置 | Google OAuth client secret（Gemini OAuth）。留空则使用内置 Gemini CLI client。 |
| `GEMINI_OAUTH_SCOPES` | 否 | 默认值 | OAuth scopes（Gemini OAuth） |
| `GEMINI_QUOTA_POLICY` | 否 | 空 | Gemini 本地配额模拟 JSON 覆盖值（仅 Code Assist） |

所有可用选项见 `.env.example`。

> **说明：** `docker-deploy.sh` 会自动生成 `JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 和 `POSTGRES_PASSWORD`。

### 简单迁移（本地目录版本）

使用 `docker-compose.local.yml` 时，所有数据都保存在本地目录，迁移很简单：

```bash
# On source server: Stop services and create archive
cd /path/to/deployment
docker compose -f docker-compose.local.yml down
cd ..
tar czf sub2api-complete.tar.gz deployment/

# Transfer to new server
scp sub2api-complete.tar.gz user@new-server:/path/to/destination/

# On new server: Extract and start
tar xzf sub2api-complete.tar.gz
cd deployment/
docker compose -f docker-compose.local.yml up -d
```

这样会迁移整个部署（配置 + 数据）。

---

## Gemini OAuth 配置

Sub2API 支持三种方式连接 Gemini。

### 方式 1：Code Assist OAuth（推荐 GCP 用户）

**不需要配置**，始终使用内置 Gemini CLI OAuth client（公开 client）。

1. 保持 `GEMINI_OAUTH_CLIENT_ID` 和 `GEMINI_OAUTH_CLIENT_SECRET` 为空。
2. 在 Admin UI 中创建 Gemini OAuth 账号，并选择 **"Code Assist"** 类型。
3. 在浏览器中完成 OAuth 流程。

> 说明：即使为 AI Studio OAuth 配置了 `GEMINI_OAUTH_CLIENT_ID` / `GEMINI_OAUTH_CLIENT_SECRET`，Code Assist OAuth 仍会使用内置 Gemini CLI client。

**要求：**

- Google 账号可访问 Google Cloud Platform。
- 一个 GCP project（自动检测或手动指定）。

**如何获取 Project ID（自动检测失败时）：**

1. 打开 [Google Cloud Console](https://console.cloud.google.com/)。
2. 点击页面顶部的 project 下拉框。
3. 从列表中复制 Project ID（不是 project name）。
4. 常见格式：`my-project-123456` 或 `cloud-ai-companion-xxxxx`。

### 方式 2：AI Studio OAuth（普通 Google 账号）

需要自己的 OAuth client credentials。

**步骤 1：在 Google Cloud Console 创建 OAuth Client**

1. 打开 [Google Cloud Console - Credentials](https://console.cloud.google.com/apis/credentials)。
2. 创建新 project 或选择已有 project。
3. **启用 Generative Language API：**
   - 打开 "APIs & Services" → "Library"。
   - 搜索 "Generative Language API"。
   - 点击 "Enable"。
4. **配置 OAuth Consent Screen**（如果尚未配置）：
   - 打开 "APIs & Services" → "OAuth consent screen"。
   - 选择 "External" user type。
   - 填写 app name、user support email、developer contact。
   - 添加 scopes：`https://www.googleapis.com/auth/generative-language.retriever`（可选添加 `https://www.googleapis.com/auth/cloud-platform`）。
   - 添加 test users（你的 Google 账号邮箱）。
5. **创建 OAuth 2.0 credentials：**
   - 打开 "APIs & Services" → "Credentials"。
   - 点击 "Create Credentials" → "OAuth client ID"。
   - Application type 选择 **Web application**（或 **Desktop app**）。
   - Name 示例："Sub2API Gemini"。
   - Authorized redirect URIs 添加 `http://localhost:1455/auth/callback`。
6. 复制 **Client ID** 和 **Client Secret**。
7. **发布到 Production（重要）：**
   - 打开 "APIs & Services" → "OAuth consent screen"。
   - 点击 "PUBLISH APP"，从 Testing 切到 Production。
   - **Testing 模式限制：**
     - 只有手动添加的 test users 可以认证（最多 100 个）。
     - Refresh token 7 天后过期。
     - 需要定期重新添加用户。
   - **Production 模式：** 任意 Google 用户可认证，token 不会过期。
   - 说明：敏感 scopes 可能需要 Google 验证（demo video、privacy policy）。

**步骤 2：配置环境变量**

```bash
GEMINI_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
GEMINI_OAUTH_CLIENT_SECRET=GOCSPX-your-client-secret

# 可选：如需使用 Gemini CLI 内置 OAuth Client（Code Assist / Google One）
# 安全说明：本仓库不会内置该 client_secret，请在运行环境通过环境变量注入。
# GEMINI_CLI_OAUTH_CLIENT_SECRET=GOCSPX-your-built-in-secret
```

**步骤 3：在 Admin UI 创建账号**

1. 创建 Gemini OAuth 账号，并选择 **"AI Studio"** 类型。
2. 完成 OAuth 流程。
   - 同意授权后，浏览器会跳转到 `http://localhost:1455/auth/callback?code=...&state=...`。
   - 复制完整 callback URL（推荐），或只复制 `code` 并粘贴回 Admin UI。

### 方式 3：API Key（最简单）

1. 打开 [Google AI Studio](https://aistudio.google.com/app/apikey)。
2. 点击 "Create API key"。
3. 在 Admin UI 中创建 Gemini **API Key** 账号。
4. 粘贴 API key（以 `AIza...` 开头）。

### 对比表

| 功能 | Code Assist OAuth | AI Studio OAuth | API Key |
|------|-------------------|-----------------|---------|
| 配置复杂度 | 低（无需配置） | 中（需要 OAuth client） | 低 |
| 是否需要 GCP Project | 是 | 否 | 否 |
| 自定义 OAuth Client | 否（内置） | 是（必需） | N/A |
| 速率限制 | GCP quota | 标准 | 标准 |
| 适用场景 | GCP 开发者 | 需要 OAuth 的普通用户 | 快速测试 |

---

## 二进制安装

适用于使用 systemd 的生产服务器。

### 一行安装

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
```

### 手动安装

1. 从 [GitHub Releases](https://github.com/Wei-Shaw/sub2api/releases) 下载最新 release。
2. 解压并将二进制复制到 `/opt/sub2api/`。
3. 将 `sub2api.service` 复制到 `/etc/systemd/system/`。
4. 运行：

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable sub2api
   sudo systemctl start sub2api
   ```

5. 在浏览器中打开 Setup Wizard 完成配置。

### 命令

```bash
# Install
sudo ./install.sh

# Upgrade
sudo ./install.sh upgrade

# Uninstall
sudo ./install.sh uninstall
```

### 服务管理

```bash
# Start the service
sudo systemctl start sub2api

# Stop the service
sudo systemctl stop sub2api

# Restart the service
sudo systemctl restart sub2api

# Check status
sudo systemctl status sub2api

# View logs
sudo journalctl -u sub2api -f

# Enable auto-start on boot
sudo systemctl enable sub2api
```

### 配置

#### 服务地址和端口

安装期间会提示配置服务监听地址和端口。这些设置会作为环境变量存储在 systemd service 文件中。

安装后如需修改：

1. 编辑 systemd service：

   ```bash
   sudo systemctl edit sub2api
   ```

2. 添加或修改：

   ```ini
   [Service]
   Environment=SERVER_HOST=0.0.0.0
   Environment=SERVER_PORT=3000
   ```

3. 重新加载并重启：

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart sub2api
   ```

#### Gemini OAuth 配置

如果 Gemini 账号需要使用 AI Studio OAuth，请将 OAuth client credentials 添加到 systemd service 文件：

1. 编辑 service 文件：

   ```bash
   sudo nano /etc/systemd/system/sub2api.service
   ```

2. 在 `[Service]` 段中添加 OAuth credentials（位于已有 `Environment=` 行之后）：

   ```ini
   Environment=GEMINI_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
   Environment=GEMINI_OAUTH_CLIENT_SECRET=GOCSPX-your-client-secret
   ```

   如需使用“内置 Gemini CLI OAuth Client”（Code Assist / Google One），还需要注入：

   ```ini
   Environment=GEMINI_CLI_OAUTH_CLIENT_SECRET=GOCSPX-your-built-in-secret
   ```

3. 重新加载并重启：

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart sub2api
   ```

> **说明：** Code Assist OAuth 不需要任何配置，它使用内置 Gemini CLI client。详细步骤见上文 [Gemini OAuth 配置](#gemini-oauth-配置)。

#### 应用配置

主配置文件位于 `/etc/sub2api/config.yaml`，由 Setup Wizard 创建。

### 前置要求

- Linux 服务器（Ubuntu 20.04+、Debian 11+、CentOS 8+ 等）
- PostgreSQL 14+
- Redis 6+
- systemd

### 目录结构

```text
/opt/sub2api/
├── sub2api              # Main binary
├── sub2api.backup       # Backup (after upgrade)
└── data/                # Runtime data

/etc/sub2api/
└── config.yaml          # Configuration file
```

---

## 故障排查

### Docker

**本地目录版本：**

```bash
# Check container status
docker compose -f docker-compose.local.yml ps

# View detailed logs
docker compose -f docker-compose.local.yml logs --tail=100 sub2api

# Check database connection
docker compose -f docker-compose.local.yml exec postgres pg_isready

# Check Redis connection
docker compose -f docker-compose.local.yml exec redis redis-cli ping

# Restart all services
docker compose -f docker-compose.local.yml restart

# Check data directories
ls -la data/ postgres_data/ redis_data/
```

**Named volumes 版本：**

```bash
# Check container status
docker compose ps

# View detailed logs
docker compose logs --tail=100 sub2api

# Check database connection
docker compose exec postgres pg_isready

# Check Redis connection
docker compose exec redis redis-cli ping

# Restart all services
docker compose restart
```

### Binary Install

```bash
# Check service status
sudo systemctl status sub2api

# View recent logs
sudo journalctl -u sub2api -n 50

# Check config file
sudo cat /etc/sub2api/config.yaml

# Check PostgreSQL
sudo systemctl status postgresql

# Check Redis
sudo systemctl status redis
```

### 常见问题

1. **端口已被占用**：修改 `.env` 或 systemd 配置中的 `SERVER_PORT`。
2. **数据库连接失败**：检查 PostgreSQL 是否运行，以及凭据是否正确。
3. **Redis 连接失败**：检查 Redis 是否运行，以及密码是否正确。
4. **Permission denied**：二进制安装时，确保文件 ownership 正确。

---

## TLS 指纹配置

Sub2API 支持 TLS 指纹模拟，让请求看起来像来自官方 Claude CLI（Node.js client）。

> **提示：** 访问 [tls.sub2api.org](https://tls.sub2api.org/) 获取不同设备和浏览器的 TLS 指纹信息。

### 默认行为

- 内置 `claude_cli_v2` profile 模拟 Node.js 20.x + OpenSSL 3.x。
- JA3 Hash：`1a28e69016765d92e3b381168d68922c`。
- JA4：`t13d5911h1_a33745022dd6_1f22a2ca17c4`。
- Profile 选择：`accountID % profileCount`。

### 配置

```yaml
gateway:
  tls_fingerprint:
    enabled: true  # Global switch
    profiles:
      # Simple profile (uses default cipher suites)
      profile_1:
        name: "Profile 1"

      # Profile with custom cipher suites (use compact array format)
      profile_2:
        name: "Profile 2"
        cipher_suites: [4866, 4867, 4865, 49199, 49195, 49200, 49196]
        curves: [29, 23, 24]
        point_formats: 0

      # Another custom profile
      profile_3:
        name: "Profile 3"
        cipher_suites: [4865, 4866, 4867, 49199, 49200]
        curves: [29, 23, 24, 25]
```

### Profile 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 显示名称（必需） |
| `cipher_suites` | []uint16 | 十进制 cipher suites。空值表示默认 |
| `curves` | []uint16 | 十进制椭圆曲线。空值表示默认 |
| `point_formats` | []uint8 | EC point formats。空值表示默认 |

### 常用值参考

**Cipher Suites（TLS 1.3）：** `4865` (AES_128_GCM)、`4866` (AES_256_GCM)、`4867` (CHACHA20)

**Cipher Suites（TLS 1.2）：** `49195`、`49196`、`49199`、`49200`（ECDHE variants）

**Curves：** `29` (X25519)、`23` (P-256)、`24` (P-384)、`25` (P-521)
