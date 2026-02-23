<!--
  AgentCode Local MCP · Local AI Coding Workbench
  双语简介：Chinese + English
-->

# AgentCode Local MCP · 本地 AI 代码工作台  
AgentCode Local MCP · Local AI Coding Workbench

> 给 AI Agent 一套“VS Code 级”的本地开发工具，但只占一个小小的 Go 二进制，能跑在树莓派 Zero 2W 上。  
> A VS Code–like local coding backend for AI agents, packaged into a tiny Go binary that can run even on a Raspberry Pi Zero 2W.

---

**中文简介（Chinese）**  

**AgentCode Local MCP** 是一个用 Go 编写的、完全本地的 MCP 服务器。  
它把“读写代码、应用补丁、搜索替换、受控执行命令”等能力，打包成一组标准 MCP 工具，让任意支持 MCP 的 AI Agent（Claude / OpenClaw 等）都可以像使用本地 IDE 一样操作你的代码仓库。

- 无远程依赖：不绑云端，不绑账号，所有操作都在本地发生
- 面向 AI Agent：接口是 MCP 工具，而不是人类 CLI
- 适配低资源设备：在树莓派 Zero 2W 这类 512MB 内存的小机器上也能跑
- 安全可控：路径沙箱、命令白名单、扩展名黑名单、输出截断、失败自动回滚

**English Overview**  

**AgentCode Local MCP** is a pure local MCP server written in Go.  
It exposes a set of MCP tools for reading/writing files, applying code patches, searching & replacing text, and running controlled commands, so any MCP-capable AI agent (Claude, OpenClaw, etc.) can work with your local codebase as if it were an IDE.

- No remote dependencies: everything runs locally, no cloud backend required  
- Agent-first design: MCP tools instead of human-oriented CLI commands  
- Friendly to low-resource hardware: runs comfortably on devices like Raspberry Pi Zero 2W  
- Safety by design: path sandboxing, command whitelists, extension blacklists, output truncation, and safe rollback on failure

---

## 核心特点

- **纯本地运行**
  - 不需要任何外部 API Key / 模型配置
  - 只关注本地文件系统和命令执行
- **为 Agent 设计的工具集**
  - 读文件、写文件、读代码片段
  - 按目录树扫描工作区（带修改时间）
  - 应用 unified diff 补丁
  - 搜索和替换文本
  - 安全执行命令（白名单 + 超时 + 输出截断）
- **高安全性**
  - 所有路径强制限于 `rootDir`
  - 支持 `allowedPaths` 精细白名单
  - `allowedBuildCommands` 白名单 + 扩展名黑名单 `.exe/.dll/.so/...`
  - 修改前自动备份，失败可回滚
- **为低资源环境优化**
  - 文件读取按块流式读取，支持最大字节数限制
  - 代码片段读取按行分页，避免一次性读大文件
  - 命令输出统一截断，减少大模型上下文占用
- **测试与可靠性**
  - 单元测试覆盖 Workspace 核心路径（读写、Eyes/Hands/Shield）
  - 端到端 e2e 测试真正启动 MCP 进程，通过 JSON-RPC 走完整流程
  - 已验证跨平台（Windows / Linux）构建和测试通过

---

## MCP 工具一览

当前 MCP 工具列表（也是 AI Agent 可调用的能力）：

| 工具名称                     | 作用                         | 关键参数                                                                 |
|-----------------------------|------------------------------|--------------------------------------------------------------------------|
| `workspace.read_file`       | 读取文件                     | `path`, `maxBytes`                                                      |
| `workspace.write_file`      | 写入文件（替换或创建）         | `path`, `content`, `allowCreate`                                       |
| `workspace.inspect_workspace` | 扫描目录树并返回列表       | `path`, `maxDepth`                                                     |
| `workspace.read_code_fragment` | 按行读取代码片段         | `path`, `startLine`, `endLine`                                         |
| `workspace.apply_unified_diff` | 应用 unified diff 补丁   | `diffText`, `dryRun`                                                   |
| `workspace.search_and_replace` | 搜索并替换文本           | `path`, `old`, `new`, `expectedOccurrences`                            |
| `workspace.secure_exec`     | 受控执行命令                 | `command`, `args`, `timeoutSeconds`                                    |
| `workspace.health`          | 健康检查（版本 + 工具清单） | 无参数                                                                  |

---

## 快速开始

### 0. 一键安装（推荐）

在支持 Bash 的环境（Linux/macOS）中，可以通过脚本一键安装：

```bash
curl -fsSL https://raw.githubusercontent.com/toughworm/AgentCode-Local-MCP/refs/heads/main/install-agentcode-mcp.sh -o install-agentcode-mcp.sh
chmod +x install-agentcode-mcp.sh
./install-agentcode-mcp.sh
```

安装完成后，二进制默认位于 `/usr/local/bin/agentcode-mcp`，配置文件位于 `~/.config/agentcode-mcp/config.json`。

大多数 AI Agent 只能访问自己的工作目录，你可以直接把下面这个链接发给 AI，让它在线阅读使用说明（无需本地文件）：

```text
https://raw.githubusercontent.com/toughworm/AgentCode-Local-MCP/refs/heads/main/AGENT.md
```



### 1. 安装 Go

确保本机已经安装 Go 1.22+：

```bash
go version
```

### 2. 获取代码（本地构建可选）

```bash
git clone https://github.com/toughworm/AgentCode-Local-MCP.git
cd AgentCode-Local-MCP
```

### 3. 配置

创建配置文件（推荐）：

- 路径：`~/.config/agentcode-mcp/config.json`（Linux/macOS）
- 或 `C:\Users\<you>\.config\agentcode-mcp\config.json`（Windows）

示例：

```json
{
  "logLevel": "info",
  "rootDir": "/path/to/your/workspace",
  "allowedBuildCommands": ["go", "go test", "go build", "go run"],
  "maxFileBytes": 1048576,
  "blockedExtensions": [".exe", ".dll", ".so", ".dylib"],
  "buildTimeout": 60
}
```

说明：

- `logLevel`：`debug` / `info` / `warn` / `error`
- `rootDir`：Agent 允许操作的工作目录根路径
- `allowedBuildCommands`：允许执行的命令前缀（白名单）
- `maxFileBytes`：单次读取文件的最大字节数
- `blockedExtensions`：禁止读写的文件扩展名
- `buildTimeout`：命令执行超时时间（秒）

### 4. 构建

在项目根目录：

```bash
go build -o agentcode-mcp ./cmd/opencode-mcp
```

### 5. 运行

```bash
./agentcode-mcp --config ~/.config/agentcode-mcp/config.json
```

进程会通过 **STDIO** 使用 MCP 协议通讯，等待来自 AI Agent 的 JSON-RPC 请求。

---

## 在 Claude / Cursor 中接入

以 Claude Desktop 为例，在它的配置中增加：

```json
{
  "mcpServers": {
    "agentcode-local": {
      "command": "/absolute/path/to/agentcode-mcp",
      "args": ["--config", "/absolute/path/to/config.json"],
      "env": {}
    }
  }
}
```

完成后，Claude 就能在对话中调用：

- `workspace.read_file`
- `workspace.write_file`
- `workspace.apply_unified_diff`
- `workspace.secure_exec`
- 等一整套工具，直接操作你的本地代码仓。

---

## 适配低资源设备（Raspberry Pi Zero 2W 等）

本项目从一开始就是为“AI Agent + 低资源硬件”设计的：

- 二进制小巧：纯 Go 实现，无额外守护进程
- I/O 流控：
  - `ReadFile` 按块读取，尊重 `maxFileBytes`
  - `ReadCodeFragment` 限制行数，大文件要求分页访问
- 命令执行：
  - 带超时（`buildTimeout`），避免长时间卡死
  - 输出统一走 `TruncateOutputString`，默认最大 2000 字符
- 文件安全：
  - 修改前自动备份 `.bak`，出错可回滚
  - 所有路径都经过 `sanitizePath`，防止目录逃逸
  - 长时间空闲（约 30 分钟无工具调用）时进程会自动退出，可由宿主按需重新拉起

你可以安全地把它部署到树莓派，作为“本地 Agent 专用的开发后端”。

---

## 安全设计一览

- **路径沙箱**
  - 所有操作必须在 `rootDir` 下
  - 使用 `filepath.EvalSymlinks`，防止通过符号链接逃逸
- **命令白名单**
  - 所有执行命令都通过 `isAllowedCommand` 检查
  - 支持前缀匹配（如 `"go"` 匹配 `"go build"`、`"go test"`）
- **扩展名黑名单**
  - 默认禁止对 `.exe`、`.dll`、`.so`、`.dylib` 等二进制文件执行读写
- **输出截断**
  - `TruncateOutputString` 保留头尾，中间用 `[TRUNCATED]` 标记
  - 避免大模型上下文被长日志淹没
- **备份与回滚**
  - 写入前先写 `.tmp`，再 `rename` 原子替换
  - 可选 `.bak` 备份策略，确保失败后可以恢复

---

## 开发与测试

### 运行单元测试

```bash
go test ./internal/workspace/... -v
```

覆盖：

- `os_workspace.go`：读写文件、命令执行、物理文件大小
- `eyes.go`：工作区扫描、代码片段读取
- `hands.go`：unified diff 解析与应用、搜索替换
- `shield.go`：安全执行与输出截断

### 运行端到端测试

```bash
go test ./e2e -v
```

这会：

1. 在临时目录构建 `opencode-mcp` 二进制
2. 启动 MCP 进程
3. 通过 STDIO 发送 `initialize` / `tools/list` / `tools/call(workspace.health)` 请求
4. 验证返回的版本号和工具列表

---

## 适用场景

- 想给 Claude / 其它 Agent 一个**真正有“手”和“眼”的本地开发环境**
- 希望在笔记本、树莓派、小主机上做**完全本地的代码编辑 + 执行**
- 不想把代码仓写权限交给云端服务，想用**本地沙箱 + 简单可审计的安全策略**

---


但对外展示的产品名可以直接用本文标题：  
**AgentCode Local MCP · AI Agent 的本地代码工作台**，一眼就能看出它是干什么的。
