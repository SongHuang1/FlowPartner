# AGENTS.md

## 项目概述

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。这些没有计算机背景的用户过度信任 AI，因此该 AI Agent 桌面应用必须代替用户承担安全守门人的角色。

**核心优先级**：防呆 > 安全 > 可恢复 > 功能 > 性能。

任何可能使用户误操作或陷入不可恢复状态的设计均视为不合格。

**所有的设计都是针对我们编写的软件来说的，要求软件实现这些功能；在代码编写中并不需要遵循这样的安全设计。在代码编写过程中，应该在不确定的时候，询问用户的意见。**

所有的方案应该采用工业级、长久性、大型项目使用的方案，不要先用简单方案替代；想想如果你要跟他打交道很长时间，你会选择的方案。

不允许更改uv.lock的源，因为这样会导致Github Actions无法拉取到包。

---

## 架构概览（当前实际状态）

### 总体架构：WebSocket + gRPC 双向通信 + LLM 流式调用 + 多 Agent + 快照

```
Frontend (Electron + React + TypeScript)
    │
    │  WebSocket (JSON: {action: "start_chat", content: "..."} 等六种 action)
    ▼
Backend (Go)
    ├── WebSocketHandler (internal/handler/ws.go)
    │       ↓ 注册 session，生成 sessionId
    │       ├── start_chat / permission_response / cancel_task（聊天与审批）
    │       └── manual_snapshot / system_lock / restore（快照控制）
    ├── bridge.Manager (internal/bridge/manager.go)
    │       ├── sessions map: sessionId → WebSocket conn
    │       └── CmdChan: 发往 Python 的指令通道
    │       ↓ gRPC bidirectional stream（端口从 50051 起动态发现）
    ├── AgentHandler (internal/handler/agent.go)
    │       ├── SyncChannel: 接收 Python 事件 → 转发到 WebSocket
    │       ├── CallLLM: 服务端流式 RPC → 调用 LLM Client
    │       ├── ExecuteTool: 工具执行代理
    │       └── ListAgents / GetAgent: 向 Python 提供智能体定义
    │
    ├── LLM Client (internal/llm/client.go)
    │       ├── HTTP POST → OpenAI 兼容 API
    │       ├── SSE 流式解析 (internal/llm/sse.go)
    │       ├── 错误分类 (internal/llm/error.go)
    │       └── URL 规范化 (internal/llm/url.go)
    │
    ├── ModelConfig Handler (internal/handler/model_config.go)
    │       ├── CRUD: /api/model_configs
    │       ├── Activate: /api/model_configs/{id}/activate
    │       └── 加密存储 API Key (internal/crypto)
    │
    ├── AgentDef Handler (internal/handler/agent_def.go)   ← 多 Agent
    │       ├── CRUD: /api/agents（内置主智能体 id 固定为 "main"）
    │       ├── 变更后经 gRPC 广播 agents_changed 失效通知
    │       └── 持久化到 internal/storage/agents.go（agents.json 单一真相源）
    │
    ├── Snapshot 子系统 (internal/snapshot/, internal/handler/snapshot_handler.go)
    │       ├── 触发：工作区文件变更防抖 60s / 15min 周期兜底 / 锁屏 flush / 手动 / 还原前预快照
    │       ├── capture + exclude（敏感文件默认排除、单文件 100MB 上限）+ manifest
    │       ├── restore（先自动预快照保证可逆）+ retain（保留 30 天 / 5GB）
    │       ├── 状态经 WebSocket broadcast（snapshot_status / snapshot_message）
    │       └── REST: /api/snapshots
    │
    ├── Keystore (internal/keystore/keystore.go)
    │       ├── TryActivate: 解密 + 解锁（带速率限制）
    │       ├── SwitchKey: 原子切换密钥
    │       └── GetKey: 供 LLM Client 使用
    │
    ├── Tools Executor (internal/tools/)
    │       ├── executor.go: 工具调度（read/write/bash/edit/trash/purge）
    │       ├── approval.go: 权限审批管理（一次性令牌 + 会话级信任）
    │       ├── path_guard.go: 双层路径验证（词法 + 符号链接解析）
    │       ├── delete_guard.go: 拦截 shell 删除类命令（rm/del/Remove-Item 等）
    │       ├── trash.go / purge.go: 可恢复删除（回收站）/ 强制审批的永久删除
    │       └── 执行结果通过 gRPC ExecuteTool 返回
    ▼
Python Agent (agent/src/agent/)
    ├── FlowPartnerClient (grpc_client.py): gRPC 客户端
    │       ├── call_llm_via_go: 处理服务端流式响应
    │       │       ├── 逐 chunk 接收 SSE JSON
    │       │       ├── 重建 tool_calls delta（丢弃参数非法的调用）
    │       │       └── 发送 llm_chunk 事件到前端
    │       ├── execute_tool: 通过 gRPC 代理执行工具（含越权审批等待）
    │       ├── ListAgents/GetAgent: 拉取智能体定义（TTL 缓存 + agents_changed 失效）
    │       └── connect_and_listen: 双向流事件循环
    ├── core/react_agent.py: ReAct 循环（思考→行动→观察，带护栏）
    ├── core/subagent_runner.py: 子 agent 派发与事件转发（subagent_* 事件）
    ├── core/agent_registry.py: 智能体定义缓存
    └── tools/: registry.py + file_ops.py + agent_tool.py
            （工具 read/write/bash/edit/trash/purge 全部通过 Go 代理执行；
              agent__<name> 为子智能体调度入口）
```

### 前端启动流程

```
Electron main.cjs
    │ 启动后端子进程，读取就绪信号 __FP_BACKEND_READY__ HTTP=:%d gRPC=:%d
    │ 保存 backendPort
    ↓
preload.cjs
    │ 暴露 window.flowPartner.fetchBackendPort() → backendPort
    │ 暴露 window.flowPartner.onSystemLock(cb) / onSystemFocus(cb) → 系统锁屏/焦点监听
    │ 暴露 onCloseAction / sendCloseAction / updateCloseBehavior（关闭行为）
    │ 暴露 openExternal(url)、selectFolder()、getVersion()
    ↓
main.tsx bootstrap
    │ await window.flowPartner.fetchBackendPort()
    │ initApi(port) → 设置 BASE = http://localhost:{port}/api
    │ 渲染 React 应用
    ↓
useWebSocket hook
    │ 连接 ws://localhost:{port}/ws
    │ 支持自动重连（最多 5 次，间隔 3s）、处理超时（300s）、安全端口校验（1024-65535）
```

### 当前代码状态

**main.go 已正常工作**：
- `backend/cmd/server/main.go` 注入 `bridge.Manager` + `ApprovalManager` + 快照管理器，同时启动 HTTP server 和 gRPC server
- HTTP server 注册了 REST 路由（settings、settings/clear_api_key、history、unlock/lock/lock_status、model_configs、snapshots、agents）和 WebSocket 端点（`/ws`）
- gRPC server 注册了 `AgentHandler`，与 Python Agent 通过双向流通信
- 端口通过 `server.FindAvailablePort` 动态发现（绑定 127.0.0.1，HTTP 默认端口来自 FP_HTTP_PORT，gRPC 从 :50051 起探索并排除 HTTP 端口），就绪信号格式：`__FP_BACKEND_READY__ HTTP=:%d gRPC=:%d`
- 启动时按已保存设置应用快照配置（含过期清理，后台执行）
- 优雅关闭：先 gRPC GracefulStop（2s 超时强制 Stop），再断开 WebSocket，最后关闭 HTTP（2s 超时）

**HTTP handlers 已接入**：
- `internal/handler/settings.go` — settings CRUD（`/api/settings`），支持 ModelConfigs 迁移 + SSRF 防护 + 回收站/快照目录校验
- `internal/handler/history.go` — 历史 API（`/api/history`、`/api/history/{session_id}`）
- `internal/handler/unlock.go` — API Key 解锁/锁定（`/api/unlock`、`/api/lock`、`/api/lock_status`），带速率限制
- `internal/handler/model_config.go` — 模型配置 CRUD（`/api/model_configs`），加密存储 + 唯一名称生成
- `internal/handler/agent_def.go` — 智能体定义 CRUD（`/api/agents`），内置主智能体 id 为 "main"；变更后经 bridge 广播 `agents_changed`
- `internal/handler/snapshot_handler.go` — 快照 REST API（`/api/snapshots`：列表/详情/手动触发/还原）
- `internal/handler/ws.go` — WebSocket handler（`/ws`），处理 `start_chat`、`permission_response`、`cancel_task`、`manual_snapshot`、`system_lock`、`restore`

**LLM 集成已完成**：
- `internal/llm/client.go` — HTTP 流式客户端，支持 SSE 解析、自动重试（仅首 chunk 前）、空闲超时
- `internal/llm/error.go` — 错误分类（401/403/404/429/500/502/503 + 网络/超时），中文错误信息 + 猜测原因
- `internal/llm/sse.go` — SSE 事件解析器（1MB 最大 token）
- `internal/llm/url.go` — BaseURL 规范化（拼接 `/chat/completions`）
- `internal/handler/agent.go` — `CallLLM` 是服务端流式 RPC，从 keystore 获取 API Key，调用 LLM Client

**工具执行层已完成**：
- `internal/tools/executor.go` — 工具调度（read/write/bash/edit/trash/purge），审批上下文传播
- `internal/tools/approval.go` — 内存中审批管理（Create/Resolve/Consume/CancelSession），一次性令牌 + 会话级信任
- `internal/tools/path_guard.go` — 双层路径验证（词法 + 符号链接解析），Windows 大小写不敏感
- `internal/tools/read.go` — 文件读取（10MB 限制，10K 字节截断，UTF-8 验证）
- `internal/tools/write.go` — 文件写入（自动创建父目录）
- `internal/tools/bash.go` — Shell 执行（30s 超时，路径逃逸验证，Windows `cmd /c` 支持，删除类命令经 delete_guard 拦截）
- `internal/tools/edit.go` — 搜索替换（要求恰好 1 个匹配）
- `internal/tools/delete_guard.go` — 识别并拦截 shell 删除命令（rm/rmdir/del/rd/deltree/Remove-Item 等），引导改用 trash
- `internal/tools/trash.go` — 移入回收站（可恢复）；单路径越权可走审批，paths 数组模式要求全部源路径在工作区内
- `internal/tools/purge.go` — 永久删除回收站条目（不可逆，无论路径一律强制审批；写 trash_audit.log 审计）
- `internal/tools/errors.go` — 工具错误码定义

**快照子系统已完成**：
- `internal/snapshot/manager.go` — 触发编排：fsnotify 变更防抖 60s / 15min 周期兜底 / 锁屏 flush / 手动 / 还原前预快照
- `capture.go` + `exclude.go` + `manifest.go` — 捕获工作区文件（敏感文件默认排除，单文件 100MB 上限），manifest 最后原子写入
- `restore.go` — 还原前自动预快照保证可逆；不同项目路径的快照禁止还原
- `retain.go` — 保留策略：30 天 / 5GB
- 状态经 WebSocket broadcast（`snapshot_status` / `snapshot_message`）；REST 见 snapshot_handler.go

**多 Agent 已完成**：
- `internal/handler/agent_def.go` + `internal/storage/agents.go` — 智能体定义 CRUD 与 agents.json 持久化（单一真相源）
- `agent/src/agent/core/agent_registry.py` — Python 侧定义缓存（TTL 兜底 + agents_changed 失效）
- `agent/src/agent/core/subagent_runner.py` — 子 agent 派发（深度限制、事件转换为 subagent_* 转发到根会话）
- `agent/src/agent/tools/agent_tool.py` — `agent__<name>` 工具注册，供主智能体调度子智能体

**Keystore 已增强**：
- `internal/keystore/keystore.go` — `TryActivate`（原子操作：速率限制检查 + 解密 + 切换密钥）、`SwitchKey`、`Unlock`、`Lock`、`GetKey`、`VerifyPassword`、`GetLockStatus`
- 速率限制：5 次失败 → 30s 锁定
- API Key 在锁/切换时清零

**Proto 已更新**：
- `LLMResponse` 改为 `is_error` + `json_response` + `message_id`
- `CallLLM` 改为 `returns (stream LLMResponse)`（服务端流式）
- `ToolRequest` 改为 `session_id` + `tool_name` + `arguments` + `approval_id`
- `ToolResponse` 改为 `success` + `result` + `error_code` + `needs_permission` + `request_id`
- 新增多 Agent 定义：`AgentDef`/`AgentId`/`AgentDefList` + `ListAgents`/`GetAgent` RPC

**前端启动流程已完成**：
- Electron main.cjs 启动 Go 后端 + Python Agent 子进程（dev 模式自动设置 PYTHONPATH=src;src/agent）
- 解析就绪信号提取端口，保存到内存
- preload.cjs 暴露 `window.flowPartner` API（fetchBackendPort、系统锁屏/焦点监听、关闭行为、openExternal、selectFolder、getVersion 等）
- React 应用通过 `useWebSocket` hook 连接 WebSocket，支持自动重连（5 次，3s 间隔）
- App 挂载 `useWindowState`：Electron 主进程锁屏通知 → system-lock DOM 事件 → useSnapshot/useLock 触发快照 flush 与锁定
- 前端已有多 Agent 支持（AgentSelector、SubAgentCard/SubAgentDrilldown）、快照设置（SnapshotSettings/useSnapshot）、Agent 管理（AgentsManager/AgentSettings）



---

## 项目结构

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml (Go + TS + Python), release.yml (Electron 构建)
├── agent/                # Python Agent 层 (uv 管理)
│   ├── proto/            # proto 文件副本（与 backend/proto/ 逐字节相同，见下方说明）
│   ├── src/agent/        # 源码 (main.py, grpc_client.py, core/, tools/)
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go    # 入口
│   ├── internal/
│   │   ├── bridge/manager.go # WebSocket ↔ gRPC 桥接（核心）
│   │   ├── handler/          # HTTP handlers + WebSocket/gRPC handlers
│   │   │   ├── agent.go      #   gRPC handler (SyncChannel, CallLLM, ExecuteTool, ListAgents, GetAgent)
│   │   │   ├── ws.go         #   WebSocket handler (/ws)
│   │   │   ├── settings.go   #   Settings CRUD (/api/settings)
│   │   │   ├── history.go    #   History API (/api/history)
│   │   │   ├── unlock.go     #   API Key 解锁/锁定 (/api/unlock, /api/lock, /api/lock_status)
│   │   │   ├── model_config.go #  模型配置 CRUD (/api/model_configs)
│   │   │   ├── agent_def.go  #   智能体定义 CRUD (/api/agents)
│   │   │   └── snapshot_handler.go # 快照 REST API (/api/snapshots)
│   │   ├── tools/            # 工具执行层（通过 gRPC 代理）
│   │   │   ├── executor.go   #   工具调度（read/write/bash/edit/trash/purge）
│   │   │   ├── approval.go   #   权限审批管理（内存中）
│   │   │   ├── path_guard.go #   路径验证（双层：词法 + 符号链接解析）
│   │   │   ├── read.go       #   文件读取（10MB 限制，UTF-8 验证）
│   │   │   ├── write.go      #   文件写入（自动创建父目录）
│   │   │   ├── bash.go       #   Shell 执行（30s 超时，路径转义验证）
│   │   │   ├── edit.go       #   搜索替换（要求恰好 1 个匹配）
│   │   │   ├── delete_guard.go # 拦截 shell 删除类命令
│   │   │   ├── trash.go      #   移入回收站（可恢复删除）
│   │   │   ├── purge.go      #   永久删除回收站条目（强制审批 + 审计日志）
│   │   │   └── errors.go     #   工具错误码定义
│   │   ├── snapshot/         # 快照子系统
│   │   │   ├── manager.go    #   触发编排（防抖/周期/锁屏/手动/还原前预快照）
│   │   │   ├── capture.go    #   工作区捕获
│   │   │   ├── exclude.go    #   排除规则（敏感文件、超大文件）
│   │   │   ├── manifest.go   #   快照清单（最后原子写入）
│   │   │   ├── restore.go    #   还原（先自动预快照保证可逆）
│   │   │   └── retain.go     #   保留策略（30 天 / 5GB）
│   │   ├── config/           # 配置加载（环境变量：FP_HTTP_PORT, FP_DEV_MODE, FP_FRONTEND_DIR）
│   │   ├── crypto/           # API Key 加密/零化（AES-256-GCM + Argon2id）
│   │   ├── keystore/         # API Key 内存管理（带速率限制）
│   │   ├── llm/              # LLM HTTP 流式客户端
│   │   │   ├── client.go     #   HTTP 流式调用 + 重试（仅在首 chunk 前）
│   │   │   ├── error.go      #   错误分类（401/403/404/429/500/502/503 + 网络/超时）
│   │   │   ├── sse.go        #   SSE 事件解析器（1MB 最大 token）
│   │   │   └── url.go        #   URL 规范化（拼接 /chat/completions）
│   │   ├── response/         # 标准响应格式（自动 request_id + 时间戳，错误码范围）
│   │   ├── sanitize/         # 错误信息净化（7 个正则模式，防止凭证泄露）
│   │   ├── server/           # 端口发现（绑定 127.0.0.1，Windows WSAEADDRINUSE 处理）
│   │   ├── static/           # 前端静态文件服务（SPA 回退）
│   │   └── storage/          # JSON 文件原子写入 + 历史 JSONL 格式 + agents.go（智能体定义持久化）
│   └── proto/                # proto 定义 + 生成的 .pb.go 文件
├── docs/                   # 目前为空（仅剩空的 specifications/multi-agent 子目录）
├── frontend/               # Electron + React + TypeScript + Tailwind
│   ├── electron/
│   │   ├── main.cjs          # Electron 主进程（启动 Go + Python，系统托盘，窗口状态持久化）
│   │   └── preload.cjs       # preload（暴露 fetchBackendPort、系统锁屏监听、关闭行为等）
│   ├── src/
│   │   ├── components/       # chat（含 SubAgentCard/SubAgentDrilldown/AgentSelector）、
│   │   │                     # layout、settings（含 SnapshotSettings/AgentsManager/AgentSettings）、ui
│   │   ├── hooks/            # useConversation, useLock, useSettings, useSnapshot,
│   │   │                     # useWindowState, useWebSocket
│   │   ├── lib/              # api.ts, utils.ts, validation.ts
│   │   └── types/            # TypeScript 类型定义
│   ├── electron-builder.yml  # 打包配置（extraResources 包含 bin/）
│   └── package.json
├── Makefile               # build/test/gen-proto 等目标（20 个，含跨平台编译 cross-build-all）
├── CONTRIBUTING.md
└── SECURITY.md
```

### Proto 文件重复

proto 定义在两个位置存在**逐字节相同**的副本（均含 `go_package` 选项）：
- `backend/proto/agent.proto` — Go 生成的源
- `agent/proto/agent.proto` — Python 生成的源

**修改 proto 时必须同步两处并重新生成桩代码（推荐直接 `make gen-proto`）**。生成文件：
- Go: `backend/proto/agent.pb.go`, `agent_grpc.pb.go`（不要手动编辑）
- Python: `agent/src/agent/agent_pb2.py`, `agent_pb2_grpc.py`（不要手动编辑）

---

## 构建与验证

### 从源码创建可执行程序

#### 前置条件

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.26+ | 后端编译 |
| Node.js | 26+ | 前端 + Electron |
| Python | 3.12+ | Agent 运行时（编译为独立可执行文件） |
| uv | 最新 | Python 依赖管理 |
| protoc | 3.21+ | gRPC 代码生成 |

#### 快速构建（推荐）

```bash
# 构建 Go 后端 + Agent + Electron 安装包
make build-electron

# 跨平台编译 Go 后端（Windows/macOS/Linux × amd64/arm64）
make cross-build-all
```

#### 分步构建

```bash
# 1. Go 后端
cd backend
go build -o flowpartner-backend ./cmd/server/

# 2. Python Agent → 独立可执行文件
cd agent
uv sync --frozen --no-default-groups --group build
uv run --no-sync pyinstaller flowpartner-agent.spec

# 3. 前端
cd frontend
npm ci
npm run build

# 4. 复制二进制到 frontend/bin/
cp backend/flowpartner-backend frontend/bin/
cp agent/dist/flowpartner-agent frontend/bin/

# 5. Electron 安装包
cd frontend
npm run build:electron
```

#### 开发模式（三层同时运行）

```bash
# 终端 1: 后端
cd backend && go run cmd/server/main.go

# 终端 2: Agent（main.py 使用 "from agent.xxx import ..." 包导入，
# 必须把 src 和 src/agent 都加入 PYTHONPATH；uv 需 --frozen 以免重写 uv.lock 源）
# Windows PowerShell:
cd agent && $env:PYTHONPATH='src;src/agent'; uv run --frozen python -m agent.main
# Linux/macOS:
cd agent && PYTHONPATH=src:src/agent uv run --frozen python -m agent.main

# 终端 3: 前端
cd frontend && npm run dev

# 终端 4: Electron（可选）
cd frontend && npm run dev:electron

# 注意：Electron dev 模式 spawn python 时已自动设置 PYTHONPATH（main.cjs），与上面手动命令等价。
```

#### 重新生成 protobuf 桩代码

```bash
# 推荐：一条命令同时生成 Go + Python 两侧
make gen-proto

# 手动生成（与 Makefile gen-proto 等价；Go 侧必须带 paths=source_relative，
# 否则 .pb.go 会生成到错误的嵌套目录）
# Go
cd backend
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/agent.proto

# Python
cd agent
uv run python -m grpc_tools.protoc -I ../backend/proto --python_out=src/agent --grpc_python_out=src/agent agent.proto
```

### 验证命令

```bash
# --- Go（backend 层）---
cd backend && go build ./...                 # 编译
cd backend && go test ./...                  # 运行测试
cd backend && go test ./... -race            # 并发安全测试
cd backend && go vet ./...                   # 静态分析
cd backend && golangci-lint run              # Lint（如已安装）

# --- Python（Agent 层）---
cd agent && uv sync --frozen                 # 安装依赖（锁定版本，勿省略 --frozen）
cd agent && uv run ruff check .              # Lint
cd agent && uv run mypy . --explicit-package-bases  # 类型检查（与 CI 一致）
cd agent && uv run pytest -v --cov=.         # 测试

# --- TypeScript（Frontend 层）---
cd frontend && npm run build                 # 构建
cd frontend && npm run lint                  # Lint
cd frontend && npm run typecheck             # TS 类型检查
cd frontend && npm run test                  # 运行测试（vitest，已是 run 模式）

# --- 全量 ---
make test-all                                # 构建+测试所有层
```

### CI 注意事项

- `agent` 的 CI job 在 `.github/workflows/ci.yml` 中已启用（Python 3.12 + uv）
- 前端 CI 使用 Node 26，Go CI 使用 Go 1.26
- Release workflow 通过 `v*` 标签触发，构建 Electron 安装包

---

## 职责边界

### TypeScript（前端）

- **DO**：UI 渲染、用户交互捕获、状态展示、通过 WebSocket 调用 Go 服务
- **DON'T**：直接操作文件系统、直接连接数据库、包含业务逻辑

### Python（Agent 调度层）

- **DO**：Agent 编排、LLM 调用、通过 gRPC 调用 Go 服务、Skill 逻辑生成
- **DON'T**：直接操作文件系统（工具通过 Go 代理执行）、执行系统命令、处理 UI

### Golang（后端执行层）

- **DO**：文件系统操作、危险操作拦截、gRPC 服务、WebSocket 服务、API Key 加密存储与内存管理、bridge 桥接、LLM HTTP 流式调用
- **DON'T**：包含 AI 推理逻辑、操作前端状态

---

## 编码规则

### 一致性检查（修代码前必须执行）

1. 先用 grep 检索所有引用点，确认变量、接口、类型的现有定义
2. 修改后的名称、参数、返回值须与所有调用处兼容
3. 禁止凭空假设某个函数已存在或某个接口格式正确

### 零冗余

- 禁止未使用的导入、未调用的函数、被注释掉的旧代码
- 禁止废话注释（如 `// 这里开始循环`），注释只解释"为什么"
- 相似逻辑出现两次以上必须提取为公共函数

### 代码风格

**Go**：
- public 函数/类型必须有 doc comment
- 错误处理用 `fmt.Errorf("context: %w", err)` 包装，禁止裸 error
- goroutine 必须有退出条件（监听 `ctx.Done()`），所有指针参数必须判空
- 禁止用 `_` 忽略 error

**Python**：
- 类型注解全覆盖（参数 + 返回值）
- 禁止裸 `except:`，须捕获具体异常
- 用 `pathlib.Path` 处理路径，禁止 `os.path`

**TypeScript**：
- 使用 ES Module (import/export)，禁止 CommonJS (require)
- 异步操作必须用 try-catch 包裹
- 组件用函数式 + Hooks，禁止 class 组件

### 工具执行层规范

**路径验证**：
- 所有文件操作必须通过 `path_guard.go` 验证
- 双层验证：词法检查 + 符号链接解析
- Windows 大小写不敏感处理

**权限审批**：
- 工作区外操作需要用户审批
- 一次性令牌（`approval.go`），用后即焚
- 审批超时或会话取消时清理待处理请求（同时清除会话级信任）

**删除保护（防呆核心）**：
- shell 删除类命令（rm/del/Remove-Item 等）经 `delete_guard.go` 拦截，引导改用 trash
- `trash`：移入回收站，可恢复；单路径越权可走审批，paths 数组模式要求全部源路径在工作区内（不支持审批）
- `purge`：永久删除回收站条目，不可逆；无论路径一律强制审批，并写 trash_audit.log 审计

**工具执行**：
- 所有工具通过 Go 代理执行（`executor.go`）
- Python Agent 不直接操作文件系统
- 工具错误码定义在 `errors.go`

**快照可恢复性**：
- 工作区文件变更自动快照（60s 防抖 + 15min 兜底），还原前强制预快照
- 敏感文件默认排除，单文件 100MB 上限

---

## Git 提交

格式：`<type>(<scope>): <subject>`

**type**：`feat`、`fix`、`refactor`、`security`、`docs`、`test`、`chore`

**scope**：`ts`、`py`、`go`、`proto`、`ui`、`agent`、`rag`、`crypto`、`keystore`、`llm`

---

## 代码验证策略

每完成一轮修改，必须确定该修改的验证方式，而不仅仅是"看起来正确"。优先级如下：

1. **代码变更涉及某模块逻辑** → 运行该模块单元测试
2. **API / proto 变更** → 运行相关测试 + 确认桩代码已重新生成（Go 和 Python 两侧）
3. **UI 变更** → 如无法启动前端，至少 `npm run typecheck && npm run lint`
4. **构建 / 配置变更** → `make test-all`

> 如果不能验证 → 明确说明"我无法验证 X，因为 Y"。禁止冒充可工作。

---

## 常见 AI 编程陷阱

| 陷阱 | 具体表现 | 正确做法 |
|------|---------|---------|
| API 幻觉 | 假设某个函数/方法/字段存在 | grep 搜源码确认后再使用 |
| 模式忽略 | 不遵循项目已有模式，自己发明新写法 | grep 找到类似实现再参照编写 |
| 过度抽象 | 为一次性逻辑创建通用框架 | YAGNI — 重复 ≥ 3 次再提取 |
| 信任输出 | 不运行验证就声称"已完成" | 改完必须运行测试/构建验证 |
| 依赖臆测 | 假设某个库已安装或某个函数可用 | grep go.mod / package.json / pyproject.toml |
| 字段猜测 | 猜测 JSON/protobuf 的字段名或类型 | grep 对应的 struct 定义确认 |
| 静默错误 | 用 `//nolint`、`_ = err`、空 catch 隐藏问题 | 每一个 error 必须处理或显式上报 |
| proto 不同步 | 改了 backend/proto 忘了 agent/proto | 修改 proto 后必须同步两侧并重新生成 |

---

## 开发反模式

- 不搜索现有代码就开始新实现 — 先 grep 看是否有现成的
- 不读取导入的源码就猜测其行为 — 先 Read 理解再使用
- 没有运行验证就声称代码可工作 — 用 `make test-all` 自证
- 在等待用户确认前先破坏性操作 — 删除/覆盖后再问来不及
- 修改 proto 后手动编辑生成的 `.pb.go` / `_pb2.py` 文件 — 用 `protoc` 重新生成
- 基于旧 HTTP chat 端点开发聊天功能 — 新架构用 WebSocket（ws.go）
- 使用旧 `sendMessage` HTTP 函数 — 该函数已从 api.ts 移除，聊天通信全部走 WebSocket（useWebSocket hook）


---

## 上下文与效率

- 长度>500 行的文件：先定位，再指定范围
- 生成文件（node_modules/、dist/、*.pb.go、*_pb2.py、*_pb2_grpc.py）：只搜索，不读取
- 不要不必要地重读最近 read 过的文件
- 同一测试不要连续运行两次无改动的版本
