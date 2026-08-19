# cpa-quota-panel

CLIProxyAPI 插件：**按厂商维度的只读额度面板**。

读取宿主 `config.yaml`（`config-path`），按 **base-url 白名单**扫描各 AI 提供商的 api-key 条目，
用官方额度接口查询剩余额度并在 CPAMP / 官方管理中心的插件页展示。**不参与请求调度（无 scheduler）、不做用量统计（无 usage_plugin）**——原 pick 机制完全不动。

## 能力

- 只声明 `management_api`（看板资源页），纯只读。
- 厂商分类筛选（展示过滤，不做统计）：默认内置 `opencode / deepseek / minimax`。
- 合并规则：**apikey 完全相同**的条目（跨多个配置段亦然）合并为一条，并展示其关联的 AI 提供商类型标签（如 `codex, claude`）。
- 厂商识别：每个厂商是"一个或多个 base-url 模式"（如 DeepSeek 官方 = `api.deepseek.com` 及 `/anthropic` 变体），命中任一即归入该厂商。
- 额度展示：`percent-windows`（opencode 三窗口进度）、`balance`（deepseek 余额）、`grants`（minimax grant 剩余）。

## 目录结构

```
cpa-quota-panel/
├── main.go / config.go / scanner.go / quota.go / management.go / dashboard.go   # 插件源码
├── *_test.go                                                                    # 单元测试
├── plugins/<GOOS>/<GOARCH>/cpa-quota-panel.<dylib|so>                          # 构建产物
├── scripts/build.sh      # 一键构建（跨平台给出 docker 命令）
├── scripts/abi_smoke.py  # 不装 CPA 直接走 C ABI 自检
└── config.example.cpa-quota-panel.yaml
```

## 构建

依赖：Go 1.26+、CGO、平台工具链。

```bash
./scripts/build.sh                        # 本机构建
# 服务器是 linux/amd64 时，用脚本打印的 docker 命令，或直接在服务器构建：
docker run --rm --platform linux/amd64 -v "$PWD":/src -w /src golang:1.26 \
  sh -c "CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildmode=c-shared \
         -o plugins/linux/amd64/cpa-quota-panel.so . && rm -f plugins/linux/amd64/cpa-quota-panel.h"
```

## 部署

1. 把 `plugins/<GOOS>/<GOARCH>/cpa-quota-panel.so` 放入运行中 CPA 的插件目录（挂载 `plugins` 即可；容器内为 `/CLIProxyAPI/plugins/…`）。
2. 在 `config.yaml` 增加 `plugins.configs.cpa-quota-panel`（见模板），`config-path` 指向宿主 config（容器内 `/CLIProxyAPI/config.yaml`）。
3. 重启 CPA（或热加载），CPAMP 侧边栏出现「额度面板」。

## 说明与边界

- 插件读的是 **config.yaml 中持久化的条目**（含 `codex-api-key / xai-api-key / claude-api-key / gemini-api-key / interactions-api-key / openai-compatibility / vertex-api-key`）；OAuth 文件类账号不在 config 内、且大多没有按 key 的额度接口，不在覆盖范围。
- `openai-compatibility` 是"一个 base-url 下多个 key"（`api-key-entries`），扫描时会展开为每 key 一条。
- miniMax 的额度接口（host/路径/返回结构）在不同账号可能有差异，首次接入用真实 key 校准。
- 看板刷新时重读 config（跟随热加载）并刷新过期额度缓存；`?refresh=1` 强制全量刷新。

## 测试

```bash
go test ./...        # 配置解析/白名单匹配/扫描合并/额度解析
python3 scripts/abi_smoke.py   # C ABI 冒烟
```
