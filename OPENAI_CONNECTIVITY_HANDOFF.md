# OpenAI 请求连通性排查交接

## 背景

账号管理中测试 OpenAI 账号时，选择会请求 `https://chatgpt.com/backend-api/codex/responses` 的测试路径，曾报错：

```text
Request failed: Post "https://chatgpt.com/backend-api/codex/responses": http: server gave HTTP response to HTTPS client
```

初步定位是 OpenAI OAuth 图像测试分支使用普通 `HTTPUpstream.Do`，而文本/compact Codex 测试分支使用 `DoWithTLS`。在代理环境下，两条路径的代理/TLS 处理不一致。

## 已完成改动

### 账号测试路径

- `backend/internal/service/account_test_service.go`
  - `testOpenAIImageOAuth` 从 `httpUpstream.Do` 改为 `httpUpstream.DoWithTLS`。
  - 使 OpenAI OAuth 文本、compact、图像测试都走一致的代理/TLS 处理。
  - 增加 nil-safe `resolveTLSProfile(account)`，账号测试和 OpenAI API Key responses capability probe 均通过该 helper 解析 TLS profile。

- `backend/internal/service/account_test_service_openai_image_test.go`
  - 增加断言：OAuth 图像测试走 `DoWithTLS`。

- `backend/internal/service/openai_oauth_passthrough_test.go`
  - 测试 recorder 增加 `usedTLS` 标记，便于验证是否使用 `DoWithTLS`。

### OpenAI 网关请求路径

- `backend/internal/service/openai_gateway_service.go`
  - `OpenAIGatewayService` 增加 `tlsFPProfileService` 依赖。
  - 增加 nil-safe helper：`resolveTLSProfile(account)`。
  - OpenAI responses 主转发和 passthrough 从 `Do` 改为 `DoWithTLS`。

- `backend/internal/service/openai_gateway_chat_completions.go`
  - chat completions bridge 改为 `DoWithTLS`。

- `backend/internal/service/openai_gateway_chat_completions_raw.go`
  - raw chat completions 改为 `DoWithTLS`。

- `backend/internal/service/openai_gateway_messages.go`
  - Anthropic messages bridge 改为 `DoWithTLS`。

- `backend/internal/service/openai_images.go`
  - OpenAI API Key images 请求改为 `DoWithTLS`。

- `backend/internal/service/openai_images_responses.go`
  - OpenAI OAuth images via Codex `/responses` 改为 `DoWithTLS`。

### OpenAI 用量快照探测

- `backend/internal/service/account_usage_service.go`
  - `AccountUsageService` 增加 `httpUpstream` 依赖。
  - OpenAI Codex usage snapshot probe 优先走 `HTTPUpstream.DoWithTLS`。
  - 保留 `httppool.GetClient(...).Do(req)` 作为 nil fallback，用于测试或旧构造路径。
  - 复用 nil-safe `resolveTLSProfile(account)`，避免旧构造或测试场景中 `TLSFingerprintProfileService` 为空时 panic。

### 依赖注入

- `backend/cmd/server/wire_gen.go`
  - `NewAccountUsageService` 注入 `httpUpstream`。
  - `NewOpenAIGatewayService` 注入 `tlsFingerprintProfileService`。

- 相关测试构造补充末尾 `nil` 参数：
  - `backend/internal/handler/openai_gateway_handler_test.go`
  - `backend/internal/service/openai_gateway_record_usage_test.go`
  - `backend/internal/service/openai_ws_protocol_forward_test.go`

## 覆盖的请求类型

已收敛到 `DoWithTLS` 的 OpenAI HTTP 请求类型：

- 账号测试：text / compact / image。
- OpenAI OAuth：`chatgpt.com/backend-api/codex/responses`。
- OpenAI API Key：`/v1/responses`。
- Responses passthrough。
- chat completions bridge。
- raw chat completions。
- Anthropic messages bridge。
- images API Key：`/v1/images/generations` / `/v1/images/edits`。
- images OAuth via Codex `/responses`。
- OpenAI API Key responses capability probe。
- OpenAI Codex usage snapshot probe。

静态扫描结果：

```powershell
rg "httpUpstream\.Do\(" backend\internal\service -g "openai*.go" -n
```

当前无命中。

剩余命中：

```powershell
rg "client\.Do\(req\)|httpUpstream\.Do\(" backend\internal\service -g "openai*.go" -g "account_usage_service.go" -n
```

仅剩：

```text
backend\internal\service\account_usage_service.go:657: resp, err = client.Do(req)
```

这是 `AccountUsageService` 在 `httpUpstream == nil` 时的 fallback。正常应用初始化已注入 `httpUpstream`，不会走 fallback。

## 已完成验证

已用 Docker Go 容器执行 gofmt：

```powershell
docker run --rm -v ${PWD}\backend:/app/backend -w /app/backend golang:1.26.3-alpine sh -c "gofmt -w cmd/server/wire_gen.go internal/handler/openai_gateway_handler_test.go internal/service/openai_gateway_record_usage_test.go internal/service/openai_ws_protocol_forward_test.go internal/service/account_usage_service.go internal/service/openai_gateway_service.go internal/service/openai_gateway_chat_completions.go internal/service/openai_gateway_chat_completions_raw.go internal/service/openai_gateway_messages.go internal/service/openai_images.go internal/service/openai_images_responses.go internal/service/account_test_service.go internal/service/account_test_service_openai_image_test.go internal/service/openai_oauth_passthrough_test.go"
```

已通过目标 OpenAI service 测试：

```powershell
docker run --rm -v ${PWD}\backend:/app/backend -w /app/backend golang:1.26.3-alpine sh -c "go test ./internal/service -run 'Test(AccountTestService_OpenAIImageOAuthHandlesOutputItemDoneFallback|AccountTestService_TestAccountConnection_OpenAI|OpenAIGatewayService_|OpenAI)' -count=1"
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/service 30.614s
```

已通过更宽范围测试：

```powershell
docker run --rm -v ${PWD}\backend:/app/backend -w /app/backend golang:1.26.3-alpine sh -c "go test ./internal/service ./internal/handler -count=1"
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/service 46.805s
ok github.com/Wei-Shaw/sub2api/internal/handler 24.184s
```

已从当前源码重新构建本地验证镜像：

```powershell
docker build -t sub2api-verify:local .
```

已用新镜像启动本地验证容器并通过健康检查：

```powershell
GET http://127.0.0.1:18080/health
```

结果：

```text
health=200 body={"status":"ok"}
```

当前验证容器状态：

```text
sub2api-verify-app       Up (healthy) 127.0.0.1:18080->8080/tcp
sub2api-verify-postgres  Up (healthy) 5432/tcp
sub2api-verify-redis     Up (healthy) 6379/tcp
```

静态检查：

```powershell
rg "httpUpstream\.Do\(" backend\internal\service -g "openai*.go" -n
git diff --check
```

结果：均通过；OpenAI service 文件中无普通 `httpUpstream.Do(` 残留。

继续收尾后补充验证：

```powershell
docker run --rm -v ${PWD}\backend:/app/backend -w /app/backend golang:1.26.3-alpine sh -c "gofmt -w internal/service/account_test_service.go internal/service/account_usage_service.go internal/service/openai_apikey_responses_probe.go"
docker run --rm -v ${PWD}\backend:/app/backend -w /app/backend golang:1.26.3-alpine sh -c "go test ./internal/service -run 'Test(AccountTestService_OpenAIImageOAuthHandlesOutputItemDoneFallback|AccountTestService_TestAccountConnection_OpenAI|OpenAIGatewayService_|OpenAI)' -count=1"
docker run --rm -v ${PWD}\backend:/app/backend -w /app/backend golang:1.26.3-alpine sh -c "go test ./internal/service ./internal/handler -count=1"
docker build -t sub2api-verify:local .
```

结果：

```text
ok github.com/Wei-Shaw/sub2api/internal/service 31.489s
ok github.com/Wei-Shaw/sub2api/internal/service 45.267s
ok github.com/Wei-Shaw/sub2api/internal/handler 24.491s
health=200 body={"status":"ok"}
```

最新本地验证容器状态：

```text
sub2api-verify-app       Up (healthy) 127.0.0.1:18080->8080/tcp
sub2api-verify-postgres  Up (healthy) 5432/tcp
sub2api-verify-redis     Up (healthy) 6379/tcp
```

## 风险与注意

- 真实 OpenAI OAuth/API Key 连通性仍依赖有效账号、token、代理和上游状态；当前验证无法替代真实账号测试。
- `DoWithTLS(profile=nil)` 会回退到普通 `Do`，所以默认未启用 TLS fingerprint 的账号行为应保持一致。
- 如果账号启用了 TLS fingerprint，但 `OpenAIGatewayService` 未注入 `TLSFingerprintProfileService`，helper 会返回 nil，避免 panic；应用正常初始化已注入。
- OpenAI WebSocket 路径使用独立 WS dialer，不属于本轮 HTTP `Do`/`DoWithTLS` 收敛范围。
