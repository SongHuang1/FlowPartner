# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner 是一款面向非专业用户的 AI Agent 桌面应用。没有计算机背景的用户往往过度信任 AI，所以软件本身必须承担安全守门人的角色，而不是把责任推给用户。

## 核心理念

大多数 AI 工具默认用户知道自己在做什么。FlowPartner 反过来。每一个设计决策都从同一个问题出发：*如果用户盲目信任 AI，会发生什么？*

由此衍生出几个不可妥协的原则：

- **防呆第一。** 任何可能让用户陷入不可恢复状态的设计，直接否决。
- **安全优先于功能。** 危险操作默认拦截。用户可以覆盖，但必须主动、有意识地选择。
- **永远可恢复。**

## 从源码构建

本节说明如何从源码创建可分发的安装包。最终产物是一个可安装的单一文件（`.exe` / `.dmg` / `.deb`）—— 终端用户不需要安装 Go、Node.js、Python 或任何其他工具。

### 前置条件

| 工具 | 版本 | 用途 |
|------|------|---------|
| Go | 1.26+ | 编译后端二进制 |
| Node.js | 26+ | 构建前端 + 打包 Electron 应用 |
| Python | 3.12+ | 将 Agent 编译为独立可执行文件 |
| uv | 最新 | Python 依赖管理 |
| protoc | 3.21+ | gRPC 代码生成 |
| protoc-gen-go | 最新 | Go protobuf 桩代码 |
| protoc-gen-go-grpc | 最新 | Go gRPC 桩代码 |

安装 protoc 插件：
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 第 1 步：克隆

```bash
git clone https://github.com/SongHuang1/FlowPartner.git
cd FlowPartner
```

### 第 2 步：构建 Go 后端

```bash
cd backend
go build -o flowpartner-backend ./cmd/server/
```

输出：`backend/flowpartner-backend`（Windows 下为 `.exe`）。

在生产环境中，Electron 从 `resources/bin/` 启动该二进制。后端就绪时打印 `__FP_BACKEND_READY__ HTTP=:PORT gRPC=:PORT`。

### 第 3 步：将 Python Agent 构建为独立可执行文件

Agent 必须打包为独立可执行文件，这样用户不需要安装 Python。

```bash
cd agent
uv sync --frozen
```

然后编译为独立二进制（需要 PyInstaller）：

```bash
cd agent
uv run pyinstaller --onefile --name flowpartner-agent src/agent/main.py
```

输出：`agent/dist/flowpartner-agent`（Windows 下为 `.exe`）。

**注意：** 如果为目标平台交叉编译，需在目标操作系统上运行 PyInstaller（或使用该平台的 CI  Runner）。

### 第 4 步：构建前端

```bash
cd frontend
npm ci
npm run build
```

### 第 5 步：组装为单一安装包

将编译好的二进制复制到 `frontend/bin/`：

```bash
# 从仓库根目录
cp backend/flowpartner-backend frontend/bin/
cp agent/dist/flowpartner-agent frontend/bin/   # 各平台同理
```

更新 `frontend/electron-builder.yml`，在 `extraResources` 中包含两个二进制：

```yaml
extraResources:
  - from: bin/
    to: bin/
    filter:
      - flowpartner-backend*
      - flowpartner-agent*
```

然后构建 Electron 安装包：

```bash
cd frontend
npm run build:electron
```

输出：
- Windows: `frontend/dist-electron/FlowPartner-Windows-{version}-{arch}.exe`
- macOS: `frontend/dist-electron/FlowPartner-macOS-{version}-{arch}.dmg`
- Linux: `frontend/dist-electron/FlowPartner-Linux-{version}-{arch}.AppImage`

### 快速构建（Makefile）

```bash
make build-go-binary          # 编译 Go 后端 → frontend/bin/
make build-agent              # 编译 Python Agent → frontend/bin/
make build-electron           # 构建前端 + 打包为单一安装包
make cross-build-all          # 为 6 种平台/架构组合交叉编译 Go
```

### 重新生成 protobuf 桩代码

```bash
# Go
cd backend && protoc --go_out=. --go-grpc_out=. proto/agent.proto

# Python
cd agent && uv run python -m grpc_tools.protoc -I ../backend/proto --python_out=src/agent --grpc_python_out=src/agent agent.proto
```

**重要：** `backend/proto/agent.proto` 和 `agent/proto/agent.proto` 必须保持同步。Go 版本有 `go_package`；Python 版本没有。

### 开发工作流

本地开发（热重载，无需打包）：

```bash
# 终端 1: 后端
cd backend && go run cmd/server/main.go

# 终端 2: Agent
cd agent && uv run python -m src.agent.main

# 终端 3: 前端
cd frontend && npm run dev

# 终端 4: Electron（可选）
cd frontend && npm run dev:electron
```

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
│   ├── proto/            # proto 文件（与 backend/proto/ 同步）
│   ├── src/agent/        # main.py, grpc_client.py, core/, tools/
│   └── pyproject.toml
├── backend/              # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── bridge/       # WebSocket ↔ gRPC 桥接
│   │   ├── handler/      # HTTP handlers + WebSocket/gRPC handlers
│   │   ├── crypto/       # API Key 加密/零化
│   │   ├── keystore/     # API Key 内存管理
│   │   ├── llm/          # LLM HTTP 流式客户端（SSE）
│   │   ├── sanitize/     # 错误信息净化
│   │   ├── server/       # 端口发现
│   │   └── storage/      # 原子 JSON 写入
│   └── proto/            # proto 定义 + 生成的 .pb.go
├── frontend/             # Electron + React + TypeScript + Tailwind
│   ├── electron/
│   │   ├── main.cjs      # Electron 主进程（启动 Go + Python）
│   │   └── preload.cjs   # IPC 桥接到渲染进程
│   ├── src/              # React UI
│   ├── electron-builder.yml
│   └── package.json
├── Makefile
└── README.md
```

## 贡献

参见 [CONTRIBUTING.md](./CONTRIBUTING.md)。

## 安全

参见 [SECURITY.md](./SECURITY.md)。
