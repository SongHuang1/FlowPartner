# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。没有计算机背景的用户往往过度信任 AI，所以软件本身必须承担安全守门人的角色，而不是把责任推给用户。

## 核心理念

大多数 AI 工具默认用户知道自己在做什么。FlowPartner 反过来。每一个设计决策都从同一个问题出发：*如果用户盲目信任 AI，会发生什么？*

由此衍生出几个不可妥协的原则：

- **防呆第一。** 任何可能让用户陷入不可恢复状态的设计，直接否决。
- **安全优先于功能。** 危险操作默认拦截。用户可以覆盖，但必须主动、有意识地选择。
- **永远可恢复。**

---

## 项目亮点

### 工业级安全架构

- **双层路径验证**：所有文件操作经过词法检查 + 符号链接解析，防止路径穿越攻击（`path_guard.go`）
- **AES-256-GCM + Argon2id 加密**：API Key 使用业界标准加密算法，内存中仅存解密态，锁定时显式零化（`crypto/`、`keystore/`）
- **速率限制防暴力破解**：5 次密码错误触发 30 秒锁定，兼顾安全与可用性（`keystore.go`）
- **错误信息脱敏**：7 个正则模式过滤日志中的 API Key、Token、密码等敏感信息（`sanitize/`）
- **删除保护**：shell 删除命令（rm/del/Remove-Item 等）统一拦截，引导使用可恢复的回收站机制（`delete_guard.go`）

### 高可靠通信链路

- **WebSocket + gRPC 双向流**：前端 WebSocket ↔ Go bridge ↔ gRPC 双向流 ↔ Python Agent，全链路异步非阻塞
- **服务端流式 LLM 调用**：SSE 流式解析 + 空闲超时 + 首 chunk 前自动重试，支持 OpenAI 兼容 API（`llm/`）
- **背压控制**：WebSocket 入站队列满时返回过载错误，防止内存溢出（`wsv2/router.go`）
- **优雅关闭**：gRPC GracefulStop → 断开 WebSocket → 关闭 HTTP，2 秒超时兜底（`main.go`）

### 自动快照与可恢复性

- **三路触发**：fsnotify 文件变更防抖 60s + 15min 周期兜底 + 锁屏 flush，确保不丢任何工作状态（`snapshot/manager.go`）
- **还原前预快照**：每次还原自动先拍一张快照，保证还原操作本身也可逆（`snapshot/restore.go`）
- **原子写入**：所有文件写入采用"写临时文件 + rename"策略，防止写入中途崩溃导致数据损坏（`snapshot/capture.go`、`storage/storage.go`）
- **保留策略**：30 天 / 5GB 双维度自动清理，防止存储无限增长（`snapshot/retain.go`）

### 多 Agent 编排

- **主-子 Agent 架构**：主智能体通过 `agent__<name>` 工具调度子智能体，支持深度限制与事件转发（`agent_def.go`、`thread/`）
- **定义热更新**：智能体定义变更经 gRPC 广播 `agents_changed` 失效通知，Python 侧 TTL 缓存自动刷新
- **回合管理**：完整的 thread/turn 生命周期，支持中断（interrupt）和引导（steer）（`thread/handlers.go`）

### 工程化质量

- **CI/CD 全流程**：GitHub Actions 三语言（Go + TypeScript + Python）+ Electron 自动构建发布
- **跨平台编译**：Windows / macOS / Linux × amd64 / arm64 一键交叉编译（`Makefile`）
- **零冗余代码**：未使用的导入、被注释的旧代码、死代码测试一律清除

---

## 当前状态

早期开发阶段。项目三层架构已就位：

- `frontend/` — Electron + React + TypeScript + Tailwind
- `backend/` — Go：gRPC、WebSocket、bridge 管理器、API Key 加密、LLM HTTP 流式客户端
- `agent/` — Python：gRPC 客户端、ReAct 循环、工具注册表

**通信流程：** Electron（端口发现）→ 前端（bootstrap）→ WebSocket → Go bridge → gRPC 双向流 → Python Agent → gRPC CallLLM（服务端流式）→ Go LLM 客户端 → OpenAI 兼容 API → SSE 分块 → 流式回传前端

## 项目结构

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml, release.yml
├── agent/                # Python Agent 层
│   ├── proto/            # proto 文件（与 backend/proto/ 逐字节相同）
│   ├── src/agent/        # main.py, grpc_client.py, core/（react_agent、subagent_runner、agent_registry）, tools/
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── wsv2/        # WebSocket v2 协议（信封、路由器、背压）
│   │   ├── thread/       # 线程/回合管理（Manager、Handler、EventConverter）
│   │   ├── handler/      # HTTP handlers + WebSocket/gRPC handlers
│   │   ├── tools/        # 工具执行层（read/write/bash/edit/trash/purge）
│   │   ├── snapshot/     # 工作区快照子系统（自动捕获 + 还原）
│   │   ├── config/       # 环境配置
│   │   ├── crypto/       # API Key 加密/零化
│   │   ├── keystore/     # API Key 内存管理
│   │   ├── llm/          # LLM HTTP 流式客户端（SSE）
│   │   ├── response/     # 标准响应格式
│   │   ├── sanitize/     # 错误信息净化
│   │   ├── server/       # 端口发现
│   │   ├── static/       # 前端静态文件服务
│   │   └── storage/      # 原子 JSON 写入（历史、智能体定义）
│   └── proto/            # proto 定义 + 生成的 .pb.go
├── frontend/             # Electron + React + TypeScript + Tailwind
│   ├── electron/
│   │   ├── main.cjs      # Electron 主进程（启动 Go + Python）
│   │   └── preload.cjs   # IPC 桥接到渲染进程
│   ├── src/              # React UI（聊天、设置：含智能体管理/快照）
│   ├── electron-builder.yml
│   └── package.json
├── Makefile
└── README.md
```

## 技术栈

| 层级 | 技术 | 用途 |
|------|------|------|
| 前端 | Electron + React + TypeScript + Tailwind | 桌面应用 UI |
| 后端 | Go 1.26+ | 通信桥接、工具执行、安全控制 |
| Agent | Python 3.12+ | AI 编排、LLM 调用、工具注册 |
| 通信 | WebSocket + gRPC 双向流 | 前后端 + 跨语言通信 |
| 序列化 | Protocol Buffers | 跨语言消息定义 |
| 加密 | AES-256-GCM + Argon2id | API Key 加密存储 |
| 构建 | Makefile + GitHub Actions | CI/CD + 跨平台编译 |

## 贡献

参见 [CONTRIBUTING.md](./CONTRIBUTING.md)。

## 安全

参见 [SECURITY.md](./SECURITY.md)。
