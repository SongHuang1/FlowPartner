# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。没有计算机背景的用户往往过度信任 AI，所以软件本身必须承担安全守门人的角色，而不是把责任推给用户。

## 核心理念

大多数 AI 工具默认用户知道自己在做什么。FlowPartner 反过来。每一个设计决策都从同一个问题出发：*如果用户盲目信任 AI，会发生什么？*

由此衍生出几个不可妥协的原则：

- **防呆第一。** 任何可能让用户陷入不可恢复状态的设计，直接否决。
- **安全优先于功能。** 危险操作默认拦截。用户可以覆盖，但必须主动、有意识地选择。
- **永远可恢复。**

## 当前状态

早期开发阶段。项目三层架构已就位：

**仓库中已有：**

- `frontend/` — Electron + React + TypeScript + Tailwind：桌面应用，含系统托盘、原生菜单、活动栏、侧边栏设置面板、聊天区域，通过 WebSocket + REST 持久化数据
- `backend/` — Go：gRPC 服务器、WebSocket 服务器、bridge 管理器（WebSocket↔gRPC）、原子文件存储、API Key 加密与内存管理
- `agent/` — Python：gRPC 客户端、ReAct Agent 循环、工具注册表（read_file, write_file, list_directory）
- `proto/` — gRPC 协议定义及 Go/Python 两侧生成代码

**通信流程：** 前端 → WebSocket → Go bridge → gRPC 双向流 → Python agent → gRPC CallLLM → Go → WebSocket → 前端

**已知问题：**

- 后端入口（`backend/cmd/server/main.go`）尚未完全接线：`bridge.Manager` 未注入，HTTP/WebSocket 服务器未启动。当前仅启动 gRPC 服务器。在修复之前，系统无法端到端服务前端请求。
- 旧 HTTP handlers（`chat.go`、`settings.go`、`conversation.go`、`unlock.go`）来自上一代架构迭代，未接入当前入口。
- `CallLLM` handler 返回 mock 响应 — 真实 LLM API 集成尚未连接。

**尚未实现：**

- 真实 LLM API 集成（当前为 mock）
- 安全机制（危险操作黑名单、自动备份、操作日志）
- 多对话管理
- Agent Skill 系统

## 项目结构

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml (Go + TS), release.yml (Electron 构建)
├── agent/                # Python Agent 层 (uv)
│   ├── proto/            # proto 文件（与 backend/proto/ 同步）
│   ├── src/agent/        # main.py, grpc_client.py, core/, tools/
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── bridge/       # WebSocket ↔ gRPC 桥接（核心）
│   │   ├── handler/      # HTTP（旧）+ WebSocket/gRPC handlers
│   │   ├── config/
│   │   ├── crypto/       # API Key 加密/零化
│   │   ├── keystore/     # API Key 内存管理
│   │   ├── response/     # 标准响应格式
│   │   └── storage/      # 原子 JSON 写入（~/.flowpartner/）
│   └── proto/            # proto 定义 + 生成的 .pb.go
├── docs/
├── frontend/             # Electron + React + TypeScript + Tailwind
│   ├── electron/main.cjs
│   ├── src/
│   │   ├── components/   # chat, layout, settings, ui
│   │   ├── hooks/        # useConversation, useLock, useSettings, useWindowState
│   │   ├── lib/          # api.ts, utils.ts, validation.ts
│   │   └── types/
│   └── package.json
├── Makefile
├── CONTRIBUTING.md
├── SECURITY.md
├── README.md
└── README.zh.md
```

## 贡献

参见 [CONTRIBUTING.md](./CONTRIBUTING.md) 了解贡献指南。

## 安全

参见 [SECURITY.md](./SECURITY.md) 了解安全政策和漏洞报告方式。
