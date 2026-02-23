# Agent Quick Reference

**针对 AI Agent 的 agentcode-local-mcp 使用指南**

当你（AI）连接到此 MCP 服务器时，以下工具可供使用。所有工具通过 JSON-RPC stdio 通信。

---

## 🎯 核心工作流建议

### 典型编码循环

1. **探索项目**: 使用 `workspace.inspect_workspace` 了解结构。
2. **读取上下文**: 使用 `workspace.read_file` 读取目标文件，或用 `workspace.read_code_fragment` 读取大文件的特定部分。
3. **定位修改点**: 确定需要修改的行号或文本块。
4. **实施修改**:
   - 精确替换：用 `workspace.search_and_replace`（建议先设置 `expectedOccurrences: 0` 探测）。
   - 补丁应用：生成 Unified Diff 并调用 `workspace.apply_unified_diff`（建议先 `dryRun: true`）。
5. **验证**: 使用 `workspace.secure_exec` 执行 `go test` 或 `go build` 等验证命令。
6. **循环**: 根据报错信息进一步调整。

---

## 🛠️ 工具速查表

| 工具名 | 关键参数 | 典型场景 |
|--------|----------|----------|
| `workspace.read_file` | `path`, `maxBytes` | 读取完整文件内容 |
| `workspace.write_file` | `path`, `content`, `allowCreate` | 创建新文件或覆盖现有文件 |
| `workspace.inspect_workspace` | `path`, `maxDepth` | 浏览目录树，获取修改时间 |
| `workspace.read_code_fragment` | `path`, `startLine`, `endLine` | 分页读取大文件或特定行 |
| `workspace.apply_unified_diff` | `diffText`, `dryRun` | 应用标准 Unified Diff 补丁 |
| `workspace.search_and_replace` | `path`, `old`, `new`, `expectedOccurrences` | 精确字符串搜索与替换 |
| `workspace.secure_exec` | `command`, `args`, `timeoutSeconds` | 在白名单限制下执行命令 |
| `workspace.health` | (无) | 获取版本及可用工具列表 |

---

## ⚠️ 常见陷阱

### 1. 路径问题
- `path` 是相对于项目根目录的路径，不要使用绝对路径。
- 如果配置了 `allowedPaths`，路径必须在其中。

### 2. 输出与大小限制
- `read_file` 默认 1MB，大文件建议用 `read_code_fragment` 分页。
- `secure_exec` 的输出会被截断为约 2000 字符，保留开头和结尾。

### 3. 补丁应用
- `apply_unified_diff` 需要标准的 Unified Diff 格式。
- **始终先 Dry-Run**: 检查预览是否符合预期。

### 4. 命令权限
- `secure_exec` 只能运行 `allowedBuildCommands` 中列出的命令前缀。
- 如果命令不在白名单，工具会返回错误。

---

## 🔒 安全策略
- **路径沙箱**: 无法访问 `rootDir` 以外的文件。
- **扩展名限制**: 默认拦截对二进制文件（`.exe`, `.dll`, `.so`等）的操作。
- **命令检查**: 只有被明确授权的命令才可以执行。

---

## 🧠 进程生命周期建议
- 当你不再需要继续操作当前代码仓时，应请求上层宿主（如 Claude Desktop、Cursor 或自建框架）主动关闭对应的 MCP 进程，以释放小型设备上的内存资源。
- 作为兜底机制，如果该 MCP 进程空闲超过约 30 分钟（无工具调用），它会自动退出；下次需要时可以由宿主重新拉起。

---

## 🎯 最佳实践 (Best Practices)
1. **最小化读取**: 优先阅读具体的代码片段，不要动辄读取整个仓库。
2. **原子化修改**: 每次只改一个功能点，并立即运行测试验证。
3. **充分利用 Replace**: 对于简单的修改，`search_and_replace` 往往比 `apply_unified_diff` 更稳健。
4. **错误导向**: 如果构建失败，仔细分析输出中的错误行号，精准定位。

---

祝你编码愉快！🤖✨
