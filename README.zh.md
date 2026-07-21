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

早期开发阶段。项目已有可运行的 Go backend 和 React frontend，Python Agent 层尚待开发。

**仓库中已有：**

- `backend/` — Go HTTP 服务：配置加载、标准响应格式、健康检查、SPA 静态资源服务
- `frontend/` — React + TypeScript + Tailwind CSS：桌面风格 UI 外壳（标题栏、活动栏、侧边栏、聊天区、状态栏）
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
├── frontend/           # TypeScript 前端（React + Vite + Tailwind）
├── backend/            # Go 后端（HTTP 服务、安全层）
├── agent/              # Python Agent 编排层（即将开发）
├── docs/               # 设计文档（不提交到仓库）
└── Makefile
```

## 本地运行

```bash
# 后端
cd backend && go run cmd/server/main.go

# 前端
cd frontend && npm install && npm run dev
```

## 贡献

项目处于早期开发阶段。架构稳定后会补充贡献指南。如果对设计有想法，欢迎提 issue。

## 许可证

[MIT](./LICENSE) © 2026 SongHuang
