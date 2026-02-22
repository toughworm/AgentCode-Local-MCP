# 工具使用指南

本文档提供 opencode-go-mcp 所有 MCP 工具的详细使用说明、参数解释和实际示例。

## 🗂️ 文件工具

### opencode.read_file

读取项目中的文件内容。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 文件路径（相对于项目根目录） |
| `project` | string | 否 | 项目标识，默认使用配置中的 `default_project` |
| `max_bytes` | integer | 否 | 最大读取字节数，默认 1MB |

**返回**:
```json
{
  "content": [
    {
      "text": "文件内容（base64 编码或明文）",
      "encoding": "utf-8"
    }
  ],
  "truncated": false
}
```

**示例**:

```json
{
  "tool": "opencode.read_file",
  "params": {
    "path": "cmd/server/main.go",
    "max_bytes": 51200
  }
}
```

**注意事项**:
- 启用低功耗模式时，`max_bytes` 会被限制为 64KB
- 缓存命中时返回速度快，适合重复读取

---

### opencode.write_file

写入或创建文件。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 文件路径 |
| `content` | string | **是** | 新文件内容 |
| `project` | string | 否 | 项目标识 |
| `allow_create` | boolean | 否 | 是否允许创建新文件（默认 true） |
| `message` | string | 否 | 变更说明（用于审计日志） |

**返回**:
```json
{
  "success": true
}
```

**示例**:

```json
{
  "tool": "opencode.write_file",
  "params": {
    "path": "README.md",
    "content": "# New Title\n\nUpdated content...",
    "message": "Update README with new features"
  }
}
```

**注意事项**:
- 写入敏感文件（如 `.env`）会被拦截
- 低功耗模式下单次写入限制 128KB

---

### opencode.list_directory

递归列出目录内容。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 目录路径 |
| `project` | string | 否 | 项目标识 |
| `depth` | integer | 否 | 递归深度（默认 1，-1 表示无限） |
| `include` | []string | 否 | 包含模式（glob） |
| `exclude` | []string | 否 | 排除模式（glob） |

**返回**:
```json
{
  "files": [
    {
      "path": "main.go",
      "is_dir": false,
      "size": 2048
    },
    {
      "path": "internal",
      "is_dir": true,
      "size": 0
    }
  ]
}
```

**示例**:

```json
{
  "tool": "opencode.list_directory",
  "params": {
    "path": ".",
    "depth": 2,
    "exclude": ["node_modules", ".git", "vendor"]
  }
}
```

---

## 🔍 搜索工具

### opencode.search_code

全文本搜索代码。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `query` | string | **是** | 搜索关键词（支持简单正则） |
| `project` | string | 否 | 项目标识 |
| `path` | string | 否 | 搜索范围（默认整个项目） |
| `limit` | integer | 否 | 最大返回条数（默认 50） |

**返回**:
```json
{
  "results": [
    {
      "path": "internal/server.go",
      "line": 42,
      "preview": "func (s *Server) Start() {"
    }
  ]
}
```

**示例**:

```json
{
  "tool": "opencode.search_code",
  "params": {
    "query": "HandleError",
    "path": "internal",
    "limit": 20
  }
}
```

---

### opencode.search_symbols

语义化搜索 Go 符号（结构体、接口、函数、方法）。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `query` | string | **是** | 符号名称（支持模糊匹配） |
| `project` | string | 否 | 项目标识 |
| `limit` | integer | 否 | 最大返回条数（默认 50） |

**返回**:
```json
{
  "symbols": [
    {
      "name": "UserService",
      "type": "struct",
      "file_path": "internal/service/user.go",
      "line": 15,
      "package": "service"
    },
    {
      "name": "GetUser",
      "type": "method",
      "file_path": "internal/service/user.go",
      "line": 25,
      "package": "service"
    }
  ]
}
```

**示例**:

```json
{
  "tool": "opencode.search_symbols",
  "params": {
    "query": "Auth",
    "limit": 30
  }
}
```

**支持的类型**: `struct`, `interface`, `func`, `method`, `const`, `var`

---

## 🔧 修改工具

### opencode.apply_patch

应用 unified diff 格式的补丁。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 目标文件路径 |
| `patch` | string | **是** | 补丁内容（unified diff 格式） |
| `project` | string | 否 | 项目标识 |
| `dry_run` | boolean | 否 | 预览模式（不实际写入） |

**返回**（dry_run=false）:
```json
{
  "success": true,
  "applied": 1
}
```

**返回**（dry_run=true）:
```json
{
  "success": true,
  "dry_run": true,
  "preview": "@@ -1,3 +1,4 @@\n+// comment\n package main\n..."
}
```

**示例**:

```json
{
  "tool": "opencode.apply_patch",
  "params": {
    "path": "main.go",
    "patch": "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@\n+// AI-generated change\n package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
    "dry_run": true
  }
}
```

**补丁格式说明**:
- 使用标准 unified diff 格式 (`diff -u` 生成)
- 支持多个文件修改（但工具当前只应用单个 `path` 的补丁）

---

### opencode.get_file_context

读取文件及其本地依赖的上下文。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `path` | string | **是** | 主文件路径 |
| `project` | string | 否 | 项目标识 |
| `max_depth` | integer | 否 | 递归深度（默认 2） |

**返回**:
```json
{
  "content": [
    {
      "path": "main.go",
      "text": "..."
    },
    {
      "path": "config/config.go",
      "text": "..."
    }
  ]
}
```

**示例**:

```json
{
  "tool": "opencode.get_file_context",
  "params": {
    "path": "internal/server/server.go",
    "max_depth": 3
  }
}
```

**工作机制**:
1. 读取主文件内容
2. 解析 Go import 语句，提取本地依赖路径
3. 递归读取依赖文件（最多 `max_depth` 层）
4. 检测循环依赖，避免无限递归

---

## 🏗️ 构建工具

### opencode.run_build

执行构建或测试命令。

**参数**:

| 名称 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `command` | string | **是** | 命令名称（如 `go`） |
| `args` | []string | **是** | 参数列表 |
| `project` | string | 否 | 项目标识（工作目录） |

**返回**:
```json
{
  "success": true,
  "output": "go: downloading...\nbuild successful",
  "duration_ms": 5432,
  "errors": []
}
```

或失败时：
```json
{
  "success": false,
  "output": "...",
  "duration_ms": 1234,
  "errors": [
    {
      "file": "main.go",
      "line": 10,
      "message": "undefined: User"
    }
  ]
}
```

**示例**:

```json
{
  "tool": "opencode.run_build",
  "params": {
    "command": "go",
    "args": ["test", "-v", "./..."]
  }
}
```

**允许的命令**（默认）:
- `go`
- `go build`
- `go test`
- `go vet`
- `go mod tidy`
- `go run`

可通过 `allowed_build_commands` 配置自定义。

---

## ❤️ 健康检查

### opencode.health

返回服务状态和性能统计。

**参数**: 无

**返回**:
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "tools": [
    "opencode.read_file",
    "opencode.write_file",
    "..."
  ],
  "stats": {
    "read_file_calls": 1523,
    "cache_hits": 1203,
    "cache_misses": 320,
    "cache_hit_ratio": 0.79
  }
}
```

**示例请求**:

空参数对象：
```json
{
  "tool": "opencode.health",
  "params": {}
}
```

---

## 💡 使用技巧

### 1. 减少重复读取

`read_file` 内置缓存，对同一文件多次读取基本无额外开销。

### 2. 搜索优化

- `search_code`：适合关键词搜索，速度快
- `search_symbols`：适合查找函数/结构体定义，更精确

### 3. 补丁安全

始终先 `dry_run: true` 审查补丁内容，确认无误后再应用。

### 4. 构建失败分析

`run_build` 返回的结构化 `errors` 数组可直接用于定位问题，优先修复第一条错误。

### 5. 上下文聚合

面对复杂修改时，先用 `get_file_context` 获取相关依赖，避免因缺少上下文导致的修改错误。

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
