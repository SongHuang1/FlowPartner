# AGENTS.md

## 项目概述

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。这些没有计算机背景的用户过度信任 AI，因此该 AI Agent 桌面应用必须代替用户承担安全守门人的角色。

**核心优先级**：防呆 > 安全 > 可恢复 > 功能 > 性能。

任何可能使用户误操作或陷入无法恢复状态的设计均视为不合格。

**所有的设计都是针对我们编写的软件来说的，要求软件实现这些功能；在代码编写中并不需要遵循这样的安全设计。在代码编写过程中，应该在不确定的时候，询问我的意见。**

所有的方案应该采用工业级、长久性、大型项目使用的方案，不要先用简单方案替代；想想如果你要跟他打交道很长时间，你会选择的方案。

---

## 架构概览（当前实际状态）

### 预期架构：WebSocket + gRPC 双向通信

```
Frontend (Electron + React + TypeScript)
    │
    │  WebSocket (JSON: {action: "start_chat", content: "..."})
    ▼
Backend (Go)
    ├── WebSocketHandler (internal/handler/ws.go)
    │       ↓ 注册 session，生成 sessionId
    ├── bridge.Manager (internal/bridge/manager.go)
    │       ├── sessions map: sessionId → WebSocket conn
    │       └── CmdChan: 发往 Python 的指令通道
    │       ↓ gRPC bidirectional stream (port 50051)
    ├── AgentHandler (internal/handler/agent.go)
    │       ├── SyncChannel: 接收 Python 事件 → 转发到 WebSocket
    │       └── CallLLM: 代理调用大模型（当前为 mock）
    ▼
Python Agent (agent/src/agent/)
    ├── FlowPartnerClient (grpc_client.go): gRPC 客户端
    ├── core/react_agent.py: ReAct 循环（思考→行动→观察）
    └── tools/: read_file, write_file, list_directory
```

### 前端启动流程

```
Electron main.cjs
    │ 启动后端子进程，读取就绪信号 __FP_BACKEND_READY__ HTTP=:%d gRPC=:%d
    │ 保存 backendPort
    ↓
preload.cjs
    │ 暴露 window.flowPartner.fetchBackendPort() → backendPort
    │ 暴露 window.flowPartner.onBackendPortChanged(cb) → 端口变化通知
    ↓
main.tsx bootstrap
    │ await window.flowPartner.fetchBackendPort()
    │ initApi(port) → 设置 BASE = http://localhost:{port}/api
    │ 渲染 React 应用
    ↓
useWebSocket hook
    │ 连接 ws://localhost:{port}/ws
    │ 支持自动重连（最多 5 次，间隔 3s）、处理超时（60s）、安全端口校验（1024-65535）
    │ 监听 onBackendPortChanged → 端口变化时重连
```

### 当前代码状态

**main.go 已正常工作**：
- `backend/cmd/server/main.go` 注入 `bridge.Manager`，同时启动 HTTP server 和 gRPC server
- HTTP server 注册了 REST 路由（settings、conversation、unlock）和 WebSocket 端点（`/ws`）
- gRPC server 注册了 `AgentHandler`，与 Python Agent 通过双向流通信
- 端口通过 `server.FindAvailablePort` 动态发现，就绪信号格式：`__FP_BACKEND_READY__ HTTP=:%d gRPC=:%d`

**HTTP handlers 已接入**：
- `internal/handler/settings.go` — settings CRUD（`/api/settings`）
- `internal/handler/conversation.go` — 对话存储（`/api/conversation`）
- `internal/handler/unlock.go` — API Key 解锁/锁定（`/api/unlock`、`/api/lock`、`/api/lock_status`）
- 这些 handlers 通过 `registerRoutes` 注册到 HTTP server，已可正常使用

**README.md 部分过时**：部分描述与实际状态有出入，判断项目状态以 AGENTS.md 为准。

目前为止，所有的测试文件都不值得信任，经过大量的更改之后，这些测试文件已经几乎不可用。

---

## 项目结构

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml (Go + TS), release.yml (Electron 构建)
├── agent/                # Python Agent 层 (uv 管理)
│   ├── proto/            # proto 文件副本（与 backend/proto/ 重复，见下方说明）
│   ├── src/agent/        # 源码 (main.py, grpc_client.py, core/, tools/)
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go    # 入口（当前是坏的）
│   ├── internal/
│   │   ├── bridge/manager.go # WebSocket ↔ gRPC 桥接（核心）
│   │   ├── handler/          # HTTP handlers（旧）+ WebSocket/gRPC handlers（新）
│   │   ├── config/           # 配置加载
│   │   ├── crypto/           # API Key 加密/零化
│   │   ├── keystore/         # API Key 内存管理
│   │   ├── response/         # 标准响应格式
│   │   ├── sanitize/         # 错误信息净化（防止凭证泄露）
│   │   ├── server/           # 端口发现
│   │   └── storage/          # JSON 文件原子写入 (~/.flowpartner/)
│   └── proto/                # proto 定义 + 生成的 .pb.go 文件
├── docs/                   # 空目录
├── frontend/               # Electron + React + TypeScript + Tailwind
│   ├── electron/main.cjs     # Electron 主进程（CommonJS）
│   ├── electron/preload.cjs   # preload（暴露 fetchBackendPort、onBackendPortChanged）
│   ├── src/
│   │   ├── components/       # chat (ChatArea, ConnectionStatus, EventDetail), layout, settings, ui
│   │   ├── hooks/            # useConversation, useLock, useSettings, useWindowState, useWebSocket
│   │   ├── lib/              # api.ts (HTTP 客户端 + 动态端口初始化), utils.ts, validation.ts
│   │   └── types/
│   └── package.json
├── Makefile               # build/test/clean 目标
├── CONTRIBUTING.md
└── SECURITY.md
```

### Proto 文件重复

proto 定义在两个位置存在近乎相同的副本：
- `backend/proto/agent.proto` — 包含 `go_package` 选项，是 Go 生成的源
- `agent/proto/agent.proto` — 无 `go_package` 选项，是 Python 生成的源

**修改 proto 时必须同步两处**。生成文件：
- Go: `backend/proto/agent.pb.go`, `agent_grpc.pb.go`（不要手动编辑）
- Python: `agent/src/agent/agent_pb2.py`, `agent_pb2_grpc.py`（不要手动编辑）

---

## 构建与验证

```bash
# --- Go（backend 层）---
cd backend && go build ./...                 # 编译
cd backend && go test ./...                  # 运行测试
cd backend && go test ./... -race            # 并发安全测试
cd backend && go vet ./...                   # 静态分析
cd backend && golangci-lint run              # Lint（如已安装）

# --- Python（Agent 层）---
cd agent && uv sync --frozen                 # 安装依赖（锁定版本）
cd agent && uv run ruff check .              # Lint
cd agent && uv run mypy . --explicit-package-bases  # 类型检查
cd agent && uv run pytest -v --cov=.         # 测试

# --- TypeScript（Frontend 层）---
cd frontend && npm run build                 # 构建
cd frontend && npm run lint                  # Lint
cd frontend && npm run typecheck             # TS 类型检查
cd frontend && npm run test -- --run         # 运行测试

# --- 全量 ---
make test-all                                # 构建+测试所有层
```

### CI 注意事项

- `agent` 的 CI job 在 `.github/workflows/ci.yml` 中被完全注释掉了
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

- **DO**：文件系统操作、危险操作拦截、gRPC 服务、WebSocket 服务、API Key 加密存储与内存管理、bridge 桥接
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

---

## Git 提交

格式：`<type>(<scope>): <subject>`

**type**：`feat`、`fix`、`refactor`、`security`、`docs`、`test`

**scope**：`ts`、`py`、`go`、`proto`、`ui`、`agent`、`rag`、`crypto`、`keystore`

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
- 基于 chat.go 开发聊天功能 — 旧 HTTP chat 端点已废弃，新架构用 WebSocket（ws.go）
- 使用旧 `sendMessage` HTTP 函数 — 该函数已从 api.ts 移除，聊天通信全部走 WebSocket（useWebSocket hook）


---

## 上下文与效率

- 长度>500 行的文件：先定位，再指定范围
- 生成文件（node_modules/、dist/、*.pb.go、*_pb2.py、*_pb2_grpc.py）：只搜索，不读取
- 不要不必要地重读最近 read 过的文件
- 同一测试不要连续运行两次无改动的版本
