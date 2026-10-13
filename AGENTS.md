# AGENTS.md

## 项目概述

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。这些没有计算机背景的用户过度信任 AI，因此该 AI Agent 桌面应用必须代替用户承担安全守门人的角色。

**核心优先级**：防呆 > 安全 > 可恢复 > 功能 > 性能。

任何可能使用户误操作或陷入不可恢复状态的设计均视为不合格。

**所有的设计都是针对我们编写的软件来说的，要求软件实现这些功能；在代码编写中并不需要遵循这样的安全设计。在代码编写过程中，应该在不确定的时候，询问用户的意见。**

所有的方案应该采用工业级、长久性、大型项目使用的方案，不要先用简单方案替代；想想如果你要跟他打交道很长时间，你会选择的方案。

**uv.lock 依赖纪律**：不允许更改 uv.lock 的源，因为这样会导致 Github Actions 无法拉取到包。pyproject.toml 中禁止配置 `[tool.uv] index-url` 之类的镜像（镜像属个人网络环境偏好；历史上仓库内的清华源配置导致任何不带 `--frozen` 的 uv 命令在重锁时把整个 lock 改写为镜像地址）。因此：
- 一切 uv 命令必须带 `--frozen`（只运行工具不安装依赖时用 `--no-sync`）
- 提交前若发现 uv.lock 存在与本任务无关的 diff，先 `git checkout -- agent/uv.lock` 还原，再排查触发重锁的原因

---

## 架构概览

### 总体架构：WebSocket v2（JSON-RPC 信封）+ gRPC 双向流 + LLM 流式调用 + 多 Agent + 快照

```
Frontend (Electron + React + TypeScript)
    │
    │  WebSocket（JSON-RPC v2 信封）
    │  握手：initialize（请求）→ initialized（通知）
    │  之后：thread/*、turn/*、snapshot/*、system/lock、response/<requestId>
    ▼
Backend (Go)
    ├── WebSocketHandler (internal/handler/ws.go)  ← /ws 唯一入口
    │       └── 升级连接 → wsv2.Router（注册 thread/turn + snapshot/system 方法）
    ├── wsv2.Router (internal/wsv2/)
    │       ├── Envelope: {id, method, params, result, error}
    │       ├── 四类消息：request（id+method）/ notification（仅 method）/ response（id+result）/ error（id+error）
    │       ├── 连接状态机：Connected → Initialized → Active（未握手的方法调用被拒）
    │       ├── 有界入站队列（容量 128）背压：队满回 -32001
    │       └── 单条消息上限 8MB；错误码 -326xx / -320xx
    ├── thread.Manager / thread.Handler (internal/thread/)
    │       ├── Thread 注册表（id/agentId/cwd/createdAt/archived）+ 多连接 fanout
    │       ├── Turn 生命周期：idle / active / aborting
    │       ├── 方法：thread/{start,list,read,archive,delete}、turn/{start,interrupt,steer}
    │       ├── Scheduler：按作用域序列化（global / global-shared-read / thread）
    │       ├── ServerRequest：反向请求表（审批），120s 超时、非法决定 fail-closed
    │       └── EventConverter：gRPC AgentEvent → WS 通知
    ├── AgentHandler (internal/handler/agent.go)：gRPC 服务
    │       ├── SyncChannel：stream AgentEvent ↔ stream ServerCommand
    │       ├── CallLLM: 服务端流式 RPC → 调用 LLM Client
    │       ├── ExecuteTool: 工具执行代理
    │       └── ListAgents / GetAgent: 向 Python 提供智能体定义
    │
    ├── LLM Client (internal/llm/)
    │       ├── HTTP POST → OpenAI 兼容 API；SSE 流式解析 (sse.go)
    │       ├── 错误分类 (error.go)；URL 规范化 (url.go)
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
    │       ├── 状态经全局 WebSocket 广播（snapshot_status / snapshot_message）
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
    ▼ gRPC 双向流（端口从 50051 起动态发现）
Python Agent (agent/src/agent/)
    ├── FlowPartnerClient (grpc_client.py): gRPC 客户端 + 命令分发
    │       ├── call_llm_via_go: 消费 LLMResponse 流（LLMChunk / LLMStreamError / UsagePayload）
    │       │       └── 重建 tool_calls delta（丢弃参数非法的调用）
    │       ├── execute_tool: 通过 gRPC 代理执行工具（含越权审批等待）
    │       ├── list_agents/get_agent: 拉取智能体定义
    │       └── connect_and_listen: 双向流事件循环
    │               （分发 start_chat / permission_response / agents_changed / graceful_stop / force_stop）
    ├── core/turn_engine.py: TurnEngine — 回合生命周期（prepare → 采样循环 → finalize / abort）
    │       └── 采样循环：drain steering → 组装 prompt → call_llm → ParallelToolRuntime → 续轮判定
    ├── core/steer.py: TurnContext / SteerQueue（中断事件 + 插话队列）
    ├── core/tool_runtime.py: ParallelToolRuntime（并发上限、写工具串行化、可中断）
    ├── core/react_agent.py: ReactAgent（兼容入口，内部构建 TurnEngine）+ 默认系统提示词
    ├── core/subagent_runner.py: SubAgentRunner — 子智能体派发与 subagent_* 事件转发
    ├── core/agent_registry.py: AgentRegistry — 智能体定义缓存（TTL 兜底 + agents_changed 失效）
    └── tools/: registry.py + file_ops.py + agent_tool.py + context.py
            （工具 read/write/bash/edit/trash/purge 全部通过 Go 代理执行；
              agent__<name> 为子智能体调度入口；session_id_var 传递会话上下文）
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
useWsV2 hook (frontend/src/hooks/useWebSocket.ts)
    │ 连接 ws://localhost:{port}/ws
    │ 握手：sendRequest('initialize', {clientInfo}) → sendNotification('initialized')
    │ 自动重连（最多 5 次，间隔 3s）、请求超时 30s、安全端口校验（1024-65535）
    │ 方法封装：startThread / startChat(turn/start) / steer / interrupt /
    │           respondToApproval(response/<id>) / triggerSnapshot / restoreSnapshot / systemLock
    │ 回调：onRequestApproval / onServerRequestResolved / onThreadEvent / onGlobalEvent
```

### WebSocket v2 协议（internal/wsv2/）

信封（`envelope.go`）：

```json
{ "id": "req-1", "method": "turn/start", "params": {...}, "result": {...},
  "error": { "code": -32001, "message": "服务器繁忙，请稍后重试", "data": null } }
```

- 四类消息：request（有 `id` + `method`）、notification（仅 `method`）、response（有 `id` + `result`）、error（有 `id` + `error`）。
- 握手：客户端发 `initialize`（request，`params.clientInfo`）→ 服务端回 `{protocolVersion:"2", platformOs, userAgent, fpHome}`；客户端再发 `initialized`（notification）→ 连接进入 Active。
- 状态机：Connected（仅接受 `initialize`）→ Initialized（仅接受 `initialized`）→ Active。未握手即调用其他方法返回 `-32600`。
- 背压：入站请求进有界队列（容量 128），队满立即回 `-32001`，防止内存被灌爆。
- 单条消息上限 8MB（`MaxPayloadSize`）。

错误码（`router.go`）：

| 码 | 含义 |
|----|------|
| -32600 | 未完成握手 |
| -32601 | 未知方法 |
| -32602 | 参数错误 |
| -32001 | 服务器繁忙（入站队列满） |
| -32002 | 回合冲突（同一 thread 并发 turn/start） |
| -32003 | 待处理审批被中断/锁定释放 |
| -32004 | 引擎未就绪 |
| -32005 | 无活跃回合 |
| -32006 | 单条消息超限 |
| -32007 | 内部错误 |

客户端 → 服务端方法：

| 方法 | 说明 |
|------|------|
| `initialize` / `initialized` | 握手 |
| `thread/start` | 创建 thread，返回 `threadId`/`agentId`/`cwd`/`createdAtMs` |
| `thread/list` | 游标分页列出 thread |
| `thread/read` | 读取 thread 详情与当前 turn |
| `thread/archive` / `thread/delete` | 归档 / 删除（仅元数据） |
| `turn/start` | 启动新回合（`threadId`、`input[]`、`overrides`），返回 `turnId` |
| `turn/interrupt` | 中断活跃回合（标记 aborting、释放待处理审批，超时兜底） |
| `turn/steer` | 对活跃回合插话（校验 turnId 后入队） |
| `response/<requestId>` | 应答服务端反向请求（审批决定） |
| `snapshot/trigger` / `snapshot/restore` | 手动快照 / 还原 |
| `system/lock` | 系统锁屏 flush |

服务端 → 客户端通知（`thread.EventConverter` 把 gRPC `AgentEvent` 转换为通知后 fanout 到该 thread 的所有连接）：

| 通知 | 来源事件 |
|------|---------|
| `turn/started` / `turn/completed` / `turn/interrupted` | turn_started / turn_completed / turn_aborted |
| `item/started` / `item/<type>/delta` / `item/completed` | item_started / item_delta / item_completed |
| `thread/tokenUsage/updated` | usage_update |
| `subagent/<eventType>` | subagent_event |
| `error` | agent_error |
| `item/commandExecution/requestApproval` | permission_request（反向请求，需客户端应答） |
| `serverRequest/resolved` | 反向请求被解析 / 超时 / 中断 |

另有全局广播（非信封，形如 `{event_type, payload}`）：`snapshot_status`、`snapshot_message`。

### thread / turn 模型（internal/thread/）

- **Thread**：会话容器，字段 `id`/`agentId`/`cwd`/`createdAt`/`archived`；维护活跃 turn、待处理反向请求、以及已连接的多个 WebSocket 连接（fanout 目标）。
- **Turn**：`idle` → `active` → `aborting`。`turn/completed` 或 `turn/aborted` 到达时清除活跃 turn；中断确认超时（10s）强制释放，避免线程永久卡死。
- **Scheduler**：按方法作用域序列化（`ScopeGlobal` / `ScopeGlobalSharedRead` / `ScopeThread`），保证同 thread 方法串行。
- **ServerRequest（反向请求）**：审批以服务端发起的 request 下发，客户端用 `response/<id>` 应答；决定必须命中 `availableDecisions`，否则按超时处理（fail-closed）；默认 120s 超时；中断 / 锁定会释放待处理请求。

### gRPC 接口（proto/agent.proto）

- **Python → Go（`AgentEvent`）**：信封 `session_id` + `turn_id` + `timestamp`，payload oneof 为 `turn_started`/`turn_completed`/`turn_aborted`/`item_started`/`item_completed`/`item_delta`/`usage_update`/`permission_request`/`subagent_event`/`agent_error`。
- **Go → Python（`ServerCommand`）**：信封 `session_id`，command oneof 为 `start_chat`/`graceful_stop`/`force_stop`/`permission_response`/`agents_changed`。
- **CallLLM**：`LLMRequest` → `stream LLMResponse`；`LLMResponse` 含 `message_id` + oneof(`chunk`, `error`) + `usage`。
- **ExecuteTool**：`ToolRequest`（`session_id` + `tool_name` + `arguments` + `approval_id`）→ `ToolResponse`（oneof(`success`, `error`, `permission`)）。
- **多 Agent**：`AgentDef`（`id`/`name`/`description`/`system_prompt`/`created_at`/`updated_at`）+ `AgentId`/`AgentDefList` + `ListAgents`/`GetAgent`。

### 子系统构成

**main.go（组合根）**：
- `backend/cmd/server/main.go` 构造 `thread.Manager` + `snapshot.Manager` + `AgentHandler` + `WebSocketHandler`，同时启动 HTTP server 与 gRPC server
- HTTP 路由：settings、settings/clear_api_key、history、unlock/lock/lock_status、model_configs、snapshots、agents、WebSocket（`/ws`）
- gRPC 注册 `AgentHandler`，与 Python 通过双向流通信
- 端口经 `server.FindAvailablePort` 动态发现（绑定 127.0.0.1，HTTP 默认端口来自 FP_HTTP_PORT，gRPC 从 :50051 起探索并排除 HTTP 端口），就绪信号 `__FP_BACKEND_READY__ HTTP=:%d gRPC=:%d`
- 启动时按已保存设置应用快照配置（含过期清理，后台执行）
- 优雅关闭：HTTP Shutdown → 断开 WebSocket → `threadMgr.Close` → gRPC GracefulStop（2s 超时强制 Stop）→ `snapshotMgr.Close`

**HTTP handlers**：
- `settings.go` — settings CRUD（`/api/settings`），支持 ModelConfigs 迁移 + SSRF 防护 + 回收站/快照目录校验
- `history.go` — 历史 API（`/api/history`、`/api/history/{session_id}`）
- `unlock.go` — API Key 解锁/锁定（`/api/unlock`、`/api/lock`、`/api/lock_status`），带速率限制
- `model_config.go` — 模型配置 CRUD（`/api/model_configs`），加密存储 + 唯一名称生成
- `agent_def.go` — 智能体定义 CRUD（`/api/agents`），内置主智能体 id 为 "main"；变更后广播 `agents_changed`
- `snapshot_handler.go` — 快照 REST API（`/api/snapshots`：列表/详情/手动触发/还原）
- `ws.go` — WebSocket 入口（`/ws`），升级连接后构造 `wsv2.Router`，注册 thread/turn + snapshot/system 方法

**LLM 集成**：
- `llm/client.go` — HTTP 流式客户端，SSE 解析、自动重试（仅首 chunk 前）、空闲超时
- `llm/error.go` — 错误分类（401/403/404/429/500/502/503 + 网络/超时），中文错误信息 + 猜测原因
- `llm/sse.go` — SSE 事件解析器（1MB 最大 token）
- `llm/url.go` — BaseURL 规范化（拼接 `/chat/completions`）
- `handler/agent.go` — `CallLLM` 服务端流式 RPC：从 keystore 取 API Key，调用 LLM Client，逐 chunk 下发

**工具执行层**：
- `tools/executor.go` — 工具调度（read/write/bash/edit/trash/purge），审批上下文传播
- `tools/approval.go` — 内存中审批管理（Create/Resolve/Consume/CancelSession），一次性令牌 + 会话级信任
- `tools/path_guard.go` — 双层路径验证（词法 + 符号链接解析），Windows 大小写不敏感
- `tools/read.go` — 文件读取（10MB 限制，10K 字节截断，UTF-8 验证）
- `tools/write.go` — 文件写入（自动创建父目录）
- `tools/bash.go` — Shell 执行（30s 超时，路径逃逸验证，Windows `cmd /c` 支持，删除类命令经 delete_guard 拦截）
- `tools/edit.go` — 搜索替换（要求恰好 1 个匹配）
- `tools/delete_guard.go` — 识别并拦截 shell 删除命令（rm/rmdir/del/rd/deltree/Remove-Item 等），引导改用 trash
- `tools/trash.go` — 移入回收站（可恢复）；单路径越权可走审批，paths 数组模式要求全部源路径在工作区内
- `tools/purge.go` — 永久删除回收站条目（不可逆，无论路径一律强制审批；写 trash_audit.log 审计）
- `tools/errors.go` — 工具错误码定义

**快照子系统**：
- `snapshot/manager.go` — 触发编排：fsnotify 变更防抖 60s / 15min 周期兜底 / 锁屏 flush / 手动 / 还原前预快照
- `capture.go` + `exclude.go` + `manifest.go` — 捕获工作区文件（敏感文件默认排除，单文件 100MB 上限），manifest 最后原子写入
- `restore.go` — 还原前自动预快照保证可逆；不同项目路径的快照禁止还原
- `retain.go` — 保留策略：30 天 / 5GB
- 状态经全局 WebSocket 广播（`snapshot_status` / `snapshot_message`）；REST 见 snapshot_handler.go

**多 Agent**：
- `handler/agent_def.go` + `storage/agents.go` — 智能体定义 CRUD 与 agents.json 持久化（单一真相源）
- `agent_registry.py` — Python 侧定义缓存（TTL 兜底 + agents_changed 失效）
- `subagent_runner.py` — 子智能体派发（深度限制、事件转换为 subagent_* 转发到根会话）
- `tools/agent_tool.py` — `agent__<name>` 工具注册，供主智能体调度子智能体

**Keystore**：
- `keystore/keystore.go` — `TryActivate`（原子：速率限制检查 + 解密 + 切换密钥）、`SwitchKey`、`Unlock`、`Lock`、`GetKey`、`VerifyPassword`、`GetLockStatus`
- 速率限制：5 次失败 → 30s 锁定
- API Key 在锁/切换时清零

**前端组成**：
- Electron main.cjs 启动 Go 后端 + Python Agent 子进程（dev 模式自动设置 PYTHONPATH=src;src/agent），系统托盘，窗口状态持久化
- preload.cjs 暴露 `window.flowPartner` API（fetchBackendPort、系统锁屏/焦点监听、关闭行为、openExternal、selectFolder、getVersion 等）
- `useWsV2`（useWebSocket.ts）实现 v2 协议客户端与握手；`useConversation` 管理消息与流式内容
- `useWindowState`：Electron 锁屏通知 → system-lock DOM 事件 → `useSnapshot`/`useLock` 触发快照 flush 与锁定
- 多 Agent UI：AgentSelector、SubAgentCard/SubAgentDrilldown、AgentsManager
- 权限审批：PermissionDialog（对应 `item/commandExecution/requestApproval`）

### 迁移状态（临时，迁移完成后删除）

proto 与两侧生成桩代码已更新为新协议，前端 `useWsV2` 已按新协议实现。以下代码仍在从旧协议迁移，尚未对齐新 proto，**当前后端无法编译**：

- `internal/thread/handlers.go` 仍构造旧 `ServerCommand{CommandType, Payload}` 字段
- `internal/thread/events.go` 仍读旧 `AgentEvent.ThreadId` / `AgentEvent_Subagent` / `AgentEvent_Error` / `PermissionRequest.ScopeOptions`
- `internal/handler/agent.go`、`internal/llm/client.go` 仍用旧 `LLMResponse.IsError`、`ToolResponse.NeedsPermission`、`UsagePayload.PromptTokens` 等字段
- `agent/src/agent/grpc_client.py` 仍按旧 `command_type` 分发、仍读旧 `LLMResponse` 字段



---

## 项目结构

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml (Go + TS + Python), release.yml (Electron 构建)
├── agent/                # Python Agent 层 (uv 管理)
│   ├── proto/            # proto 文件副本（与 backend/proto/ 逐字节相同，见下方说明）
│   ├── src/agent/        # 源码（main.py, grpc_client.py；core/: turn_engine、steer、
│   │                     #   tool_runtime、react_agent、subagent_runner、agent_registry；tools/）
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go    # 入口
│   ├── internal/
│   │   ├── wsv2/             # WebSocket v2 协议（信封、连接状态机、方法分发、背压）
│   │   ├── thread/           # thread/turn 管理（Manager、Handler、Scheduler、EventConverter、反向请求）
│   │   ├── handler/          # HTTP handlers + WebSocket/gRPC handlers
│   │   │   ├── agent.go      #   gRPC handler (SyncChannel, CallLLM, ExecuteTool, ListAgents, GetAgent)
│   │   │   ├── ws.go         #   WebSocket 入口 (/ws)，构造 wsv2.Router
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
├── docs/                   # 本地设计资料（未纳入版本控制）：specifications/（特性规格 + 00-总纲）、
│                           # abu/codex/office/opencode/openwork/openworker/pdf（竞品调研与移植决策）
├── frontend/               # Electron + React + TypeScript + Tailwind
│   ├── electron/
│   │   ├── main.cjs          # Electron 主进程（启动 Go + Python，系统托盘，窗口状态持久化）
│   │   └── preload.cjs       # preload（暴露 fetchBackendPort、系统锁屏监听、关闭行为等）
│   ├── src/
│   │   ├── components/       # chat（含 SubAgentCard/SubAgentDrilldown/AgentSelector/PermissionDialog）、
│   │   │                     # layout、settings（APISettings/AgentsManager/SnapshotSettings/
│   │   │                     # SettingsModal/CloseBehaviorSettings）、ui
│   │   ├── hooks/            # useConversation, useLock, useSettings, useSnapshot,
│   │   │                     # useWindowState, useWebSocket
│   │   ├── lib/              # api.ts, history.ts（历史消息→内容块重建）, mention.ts,
│   │   │                     # utils.ts, validation.ts
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

WebSocket v2 协议（信封结构、方法名、通知名）**不在 proto 中定义**，由 `internal/wsv2/`（服务端）与前端 `useWsV2`（客户端）共同约定，改动时两侧需同步。

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
uv run --frozen python -m grpc_tools.protoc -I ../backend/proto --python_out=src/agent --grpc_python_out=src/agent agent.proto
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
cd agent && uv run --frozen ruff check .     # Lint
cd agent && uv run --frozen mypy . --explicit-package-bases  # 类型检查（与 CI 一致）
cd agent && uv run --frozen pytest -v --cov=.  # 测试
# 不带 --frozen 的 uv run/sync 会触发重锁并可能改写 uv.lock 的源（见顶部依赖纪律）

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

- **DO**：文件系统操作、危险操作拦截、gRPC 服务、WebSocket v2 协议与 thread/turn 状态管理、API Key 加密存储与内存管理、LLM HTTP 流式调用
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

### WebSocket v2 协议

- 新增方法 / 通知必须在服务端（`internal/wsv2/` + `internal/thread/`）与前端 `useWsV2` 两侧同步
- 方法名与通知名用 `<domain>/<action>`（如 `thread/start`、`turn/completed`、`item/<type>/delta`）
- 服务端反向请求由客户端用 `response/<requestId>` 应答；决定必须命中 `availableDecisions`，否则按超时处理
- 错误统一走信封 `error.code`（-326xx 协议层 / -320xx 业务层），不要在 `result` 里夹带错误

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
| 锁污染 | 裸跑 `uv run`/`uv sync` 触发重锁，可能把镜像源写进 uv.lock | 一律 `--frozen`；提交前检查 uv.lock 是否有本任务无关的 diff |
| 失效覆盖 | 测试在测试体内手抄生产逻辑而不调用生产代码（生产代码改名/删除后测试照样绿） | 把内联分支提取为可单测方法，让测试调用真实函数 |
| 恒真断言 | 测试断言的按钮/文案/选择器与当前实现不符，查询永远落空（如断言已删除的"查看历史"按钮） | 写断言前 grep 组件源码确认真实渲染内容；UI 重构后同步更新对应测试 |
| 死代码测试 | 为零引用的遗留组件保留测试（UnlockDialog/PasswordDialog 先例） | 删除组件时同步删除其测试；grep 确认无生产引用后再删 |

---

## 开发反模式

- 不搜索现有代码就开始新实现 — 先 grep 看是否有现成的
- 不读取导入的源码就猜测其行为 — 先 Read 理解再使用
- 没有运行验证就声称代码可工作 — 用 `make test-all` 自证
- 在等待用户确认前先破坏性操作 — 删除/覆盖后再问来不及
- 修改 proto 后手动编辑生成的 `.pb.go` / `_pb2.py` 文件 — 用 `protoc` 重新生成
- 基于旧 HTTP chat 端点开发聊天功能 — 新架构用 WebSocket（ws.go）
- 使用旧 `sendMessage` HTTP 函数 — 该函数已从 api.ts 移除，聊天通信全部走 WebSocket v2（useWsV2 hook）
- 测试里手抄生产逻辑而非调用它 — 把逻辑提取为可单测方法，让测试覆盖真实代码路径


---

## 上下文与效率

- 长度>500 行的文件：先定位，再指定范围
- 生成文件（node_modules/、dist/、*.pb.go、*_pb2.py、*_pb2_grpc.py）：只搜索，不读取
- 不要不必要地重读最近 read 过的文件
- 同一测试不要连续运行两次无改动的版本
