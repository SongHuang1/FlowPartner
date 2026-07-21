# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。没有计算机背景的用户往往过度信任 AI，所以软件本身必须承担安全守门人的角色，而不是把责任推给用户。

## 核心理念

大多数 AI 工具默认用户知道自己在做什么。FlowPartner 反过来。每一个设计决策都从同一个问题出发：*如果用户盲目信任 AI，会发生什么？*

由此衍生出几个不可妥协的原则：

- **防呆第一。** 任何可能让用户陷入不可恢复状态的设计，不管多优雅，直接否决。
- **安全优先于功能。** 危险操作默认拦截。用户可以覆盖，但必须主动、有意识地选择。
- **永远可恢复。** 文件被修改或删除前，系统自动备份。一键撤销。

## 当前状态

早期开发阶段。项目已有可运行的 Go backend 和 Electron + React 桌面前端，Python Agent 层尚待开发。

**仓库中已有：**

- `backend/` — Go HTTP 服务：配置加载、标准响应格式、健康检查、SPA 静态资源服务
- `frontend/` — Electron + React + TypeScript + Tailwind：桌面应用，含系统托盘、原生菜单、开发/生产双模式
- `proto/` — gRPC 协议定义（占位，尚未填充）

**尚未实现：**

- Python Agent 编排层
- 业务逻辑与 API 端点
- WebSocket 实时通信
- 安全机制（危险操作黑名单、自动备份、操作日志）

## 项目结构

```
flowpartner/
├── proto/              # gRPC proto 定义
├── frontend/           # Electron + React 前端（TypeScript + Vite + Tailwind）
├── backend/            # Go 后端（HTTP 服务、安全层）
├── agent/              # Python Agent 编排层（即将开发）
├── .github/            # CI 工作流、Issue 模板、PR 模板
├── Makefile            # 构建和测试目标
├── LICENSE             # MIT 许可证
├── SECURITY.md         # 安全政策
└── README.md           # 本文件
```

## 本地运行

### 前置要求

- Go 1.26+
- Node.js 22+
- npm 10+

### 后端

```bash
cd backend && go run cmd/server/main.go
```

### 前端（浏览器开发模式）

```bash
cd frontend && npm install && npm run dev
```

### 前端（桌面开发模式）

```bash
# 终端 1：启动 Go 后端
cd backend && FP_DEV_MODE=true go run cmd/server/main.go

# 终端 2：启动 Electron
cd frontend && npm run dev:electron
```

### 生产构建

```bash
# 构建前端 + 编译 Go 二进制 + 打包安装程序
make build-electron
```

## 运行测试

```bash
# 全量测试（backend + frontend）
make test-all

# 仅后端
cd backend && go test ./...

# 仅前端
cd frontend && npm run test
```

## 贡献

参见 [CONTRIBUTING.md](./CONTRIBUTING.md) 了解贡献指南。

## 安全

参见 [SECURITY.md](./SECURITY.md) 了解安全政策和漏洞报告方式。

## 许可证

[MIT](./LICENSE) © 2026 SongHuang
