# 工具使用指南 (Tool Usage Guide)

本文档提供 `agentcode-local-mcp` 所有 MCP 工具的详细使用说明、参数解释和实际示例。所有工具均使用 `workspace.` 前缀。

## 🗂️ 文件与目录工具

### workspace.read_file

读取项目中的文件内容。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 文件路径（相对于项目根目录） |
| `maxBytes` | integer | 否 | 最大读取字节数，默认 1MB |

**返回**:
文本内容。如果超过 `maxBytes`，内容将被截断。

**示例**:

```json
{
  "name": "workspace.read_file",
  "arguments": {
    "path": "main.go",
    "maxBytes": 51200
  }
}
```

---

### workspace.write_file

写入或创建文件。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 文件路径 |
| `content` | string | **是** | 新文件内容 |
| `allowCreate` | boolean | 否 | 是否允许创建新文件（默认 false） |

**返回**:
操作成功的确认消息。

---

### workspace.inspect_workspace

扫描目录结构，返回文件列表及基本信息。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 目录路径（默认为 "."） |
| `maxDepth` | integer | 否 | 递归深度（默认 2） |

**返回**:
JSON 数组，包含每个节点的 `path`, `is_dir`, `size`, `mod_time`。

---

### workspace.read_code_fragment

按行范围读取代码片段，适合阅读大文件。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 文件路径 |
| `startLine` | integer | **是** | 起始行号（从 1 开始） |
| `endLine` | integer | **是** | 结束行号（包含） |

---

## 🔧 修改工具

### workspace.apply_unified_diff

应用 Unified Diff 格式的补丁。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `diffText` | string | **是** | Unified diff 内容 |
| `dryRun` | boolean | 否 | 预览模式（不实际写入） |

**返回**:
应用成功的统计信息或预览。

---

### workspace.search_and_replace

在文件中进行精确字符串替换。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 文件路径 |
| `old` | string | **是** | 待搜索的原始文本 |
| `new` | string | **是** | 替换后的新文本 |
| `expectedOccurrences` | integer | 否 | 预期匹配次数（为 0 则仅搜索不替换） |

---

## 🏗️ 执行与安全

### workspace.secure_exec

在白名单限制下执行本地命令。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `command` | string | **是** | 命令（如 `go`） |
| `args` | string[] | 否 | 参数列表 |
| `timeoutSeconds` | integer | 否 | 超时时间（秒） |

**注意**:
仅允许执行配置文件中 `allowedBuildCommands` 定义的命令。

---

### workspace.health

检查服务健康状态。

**返回**:
JSON 对象，包含版本信息、工具列表和运行状态。

---

## 💡 最佳实践

1. **先探测再读取**: 使用 `inspect_workspace` 了解目录结构。
2. **精准定位**: 使用 `read_code_fragment` 定向读取需要修改的代码行。
3. **安全修改**: 在使用 `apply_unified_diff` 之前，先开启 `dryRun: true` 进行验证。
4. **验证变更**: 修改完成后，使用 `secure_exec` 运行测试命令（如 `go test`）。

---

## 🚫 常见错误

| 错误信息 | 原因 | 解决方案 |
|---------|------|----------|
| `path is not in allowed paths list` | 路径不在白名单 | 将目标目录加入 `opencode.allowed_paths` |
| `access to files with extension .xxx is blocked` | 扩展名被拦截 | 在配置中自定义 `blocked_extensions` |
| `invalid argument: path cannot be empty` | 参数缺失 | 检查工具参数是否齐全 |
| `patch conflict` | 补丁冲突 | 重新读取文件，生成新补丁 |
| `build timeout` | 构建超时 | 增加 `build_timeout` |

---

## 📚 相关文档

- [README.md](README.md) - 项目概览
- [CONFIGURATION.md](CONFIGURATION.md) - 配置详解
- [DEVELOPMENT.md](DEVELOPMENT.md) - 开发者指南
