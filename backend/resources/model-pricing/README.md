# 模型价格数据

这个目录保存一份镜像模型价格数据的本地副本，用作 fallback。

## 数据来源

原始文件由 LiteLLM 项目维护，并通过 GitHub Actions 镜像到本仓库的 `price-mirror` 分支：

- 镜像分支（可通过 `PRICE_MIRROR_REPO` 配置）：https://raw.githubusercontent.com/<your-repo>/price-mirror/model_prices_and_context_window.json
- 上游来源：https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json

## 用途

当远程文件因以下原因无法下载时，本地副本作为 fallback：

- 网络限制
- 防火墙规则
- DNS 解析问题
- 某些地区无法访问 GitHub
- Docker 容器网络限制

## 更新流程

`pricingService` 会：

1. 优先尝试从 GitHub 下载最新版本。
2. 如果下载失败，使用这个本地副本作为 fallback。
3. 使用 fallback 文件时记录 warning 日志。

## 手动更新

如果自动化不可用，可以用以下命令手动更新价格数据文件：

```bash
curl -s https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json -o model_prices_and_context_window.json
```

## 文件格式

该文件包含 JSON 格式的模型价格信息，包括：

- 模型名称和标识符
- 输入/输出 token 成本
- 上下文窗口大小
- 模型能力

最后更新：2025-08-10
