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
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── bridge/       # WebSocket ↔ gRPC 桥接
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

## 贡献

参见 [CONTRIBUTING.md](./CONTRIBUTING.md)。

## 安全

参见 [SECURITY.md](./SECURITY.md)。
