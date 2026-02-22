package workspace

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TODO(eyes_file_overview):
//  本文件实现“空间感知模块（The Eyes）”：
//  1. TreeNode：用于表示目录树节点，包含 path / is_dir / size / mod_time / children。
//  2. InspectWorkspace：基于 filepath.WalkDir 构建目录树结果，应用内置 ignore 列表和 maxDepth。
//  3. ReadCodeFragment：基于 os.Open + bufio.Scanner 按行号读取代码片段，文件超过 20KB 时强制分页。
//  使用本文件时，请按每个函数上方的 TODO 步骤检查/完善实现。

// TreeNode 目录树节点
type TreeNode struct {
	Path     string      `json:"path"`               // 从工作区根目录开始的相对路径
	IsDir    bool        `json:"is_dir"`             // 是否为目录
	Size     int64       `json:"size"`               // 文件大小（字节），目录为 0
	ModTime  time.Time   `json:"mod_time"`           // 最后修改时间（序列化为 RFC3339 字符串）
	Children []*TreeNode `json:"children,omitempty"` // 子节点列表（目前实现返回扁平列表，children 预留用于扩展）
}

// InspectWorkspace 扫描工作区目录树
// relPath 相对于工作区根的路径，maxDepth 限制递归深度（<=0 使用默认值）
// TODO(eyes_inspect_workspace_impl):
//  1. 使用 w.sanitizePath(relPath) 将用户输入转换为安全的绝对路径 absPath。
//  2. 使用 os.Stat(absPath) 确保其存在且为目录，否则返回错误。
//  3. 如果 maxDepth <= 0，则使用安全默认值 2；在 LowResourceMode 下可考虑进一步收紧。
//  4. 定义 ignoreDirs 和 hiddenPrefixes：
//     - ignoreDirs 用于跳过 .git/node_modules/dist 等典型构建产物或 IDE 目录。
//     - hiddenPrefixes 用于跳过隐藏文件，如 .DS_Store。
//  5. 使用 filepath.WalkDir 遍历：
//     - 每个回调中检查 ctx.Done()，支持取消；
//     - 通过 filepath.Rel(w.root, path) 计算相对路径 rel；
//     - 基于 rel 计算深度（统计分隔符数量），超过 maxDepth 时，目录返回 SkipDir。
//  6. 对每个文件/目录，构建 TreeNode：
//     - 对文件：从 d.Info() 中读取 Size 和 ModTime；
//     - 对目录：Size 固定为 0，ModTime 可取目录本身的修改时间。
//  7. 将节点追加到切片 nodes 中，并在遍历完成后按目录优先 + 路径字典序排序。
func (w *OSWorkspace) InspectWorkspace(ctx context.Context, relPath string, maxDepth int) ([]*TreeNode, error) {
	// 安全检查
	absPath, err := w.sanitizePath(relPath)
	if err != nil {
		return nil, fmt.Errorf("invalid relPath: %w", err)
	}

	// 检查路径是否存在且为目录
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	// 处理 maxDepth 默认值
	if maxDepth <= 0 {
		maxDepth = 2 // 默认递归深度
	}

	// 内置忽略列表（常见开发环境目录）
	ignoreDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		".next":        true,
		"vendor":       true,
		".cache":       true,
		".venv":        true,
		"__pycache__":  true,
		".idea":        true,
		".vscode":      true,
		"coverage":     true,
		".nyc_output":  true,
	}

	// 不需要在 JSON 中输出的隐藏文件/目录（如 .DS_Store）
	hiddenPrefixes := []string{".", ".DS_Store"}

	var nodes []*TreeNode

	// 使用 filepath.WalkDir 进行扫描
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
		// 上下文取消检查
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if walkErr != nil {
			// 遇到不可访问的目录，跳过并记录日志（不直接失败）
			// 实际应通过 logger 输出，这里简化处理
			return nil
		}

		// 计算相对路径
		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			rel = path // 降级处理
		}

		// 计算当前深度
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(os.PathSeparator)) + 1
		}

		// 如果超过 maxDepth，跳过（不再递归其子项）
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 忽略特定目录
		if d.IsDir() {
			dirName := d.Name()
			if ignoreDirs[dirName] || strings.HasPrefix(dirName, ".") {
				return filepath.SkipDir
			}
		}

		// 忽略隐藏文件（点号开头的文件，Unix 风格）
		baseName := d.Name()
		for _, prefix := range hiddenPrefixes {
			if strings.HasPrefix(baseName, prefix) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil // 跳过文件
			}
		}

		// 构建 TreeNode
		node := &TreeNode{
			Path:  rel,
			IsDir: d.IsDir(),
		}

		if !d.IsDir() {
			// 文件：获取大小和修改时间
			if info, err := d.Info(); err == nil {
				node.Size = info.Size()
				node.ModTime = info.ModTime()
			} else {
				node.Size = 0
				node.ModTime = time.Time{}
			}
		} else {
			// 目录：Size 为 0，ModTime 设为当前时间（或由业务决定）
			node.Size = 0
			if info, err := d.Info(); err == nil {
				node.ModTime = info.ModTime()
			}
		}

		// 如果是顶层请求目录或需要递归构建完整树结构，我们只返回当前层级的节点
		// 子节点可通过再次调用 InspectWorkspace 获取
		nodes = append(nodes, node)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk error: %w", err)
	}

	// 按照路径排序结果，便于 Agent 解析
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir == nodes[j].IsDir {
			return strings.ToLower(nodes[i].Path) < strings.ToLower(nodes[j].Path)
		}
		return nodes[i].IsDir // 目录排在前面
	})

	return nodes, nil
}

// ReadCodeFragment 按行读取代码片段
// startLine 和 endLine 都是从 1 开始计数（1-indexed）
// TODO(eyes_read_code_fragment_impl):
//  1. 校验行号范围：startLine > 0 且 endLine >= startLine，否则返回参数错误。
//  2. 使用 w.sanitizePath(path) 获取 absPath，并使用 w.isBlockedExtension 拦截敏感扩展名。
//  3. 调用 os.Open 打开文件，使用 file.Stat 获取文件大小：
//     - 当大小 > 20KB 时，要求 Agent 分页访问；
//     - 可以通过限制 (endLine - startLine + 1) 的最大值（例如 200 行）实现。
//  4. 使用 bufio.Scanner 按行扫描，维护 currentLine 计数：
//     - 在 [startLine, endLine] 范围内收集 scanner.Text() 到 result 切片；
//     - 读到 endLine 后即可提前退出。
//  5. 扫描过程中定期检查 ctx.Done()，支持取消。
//  6. 如果实际扫描到的行数 < 请求的 endLine，可将 truncated 置为 true 提示 Agent。
func (w *OSWorkspace) ReadCodeFragment(ctx context.Context, path string, startLine, endLine int) (lines []string, truncated bool, err error) {
	// 参数校验
	if startLine <= 0 || endLine < startLine {
		return nil, false, fmt.Errorf("invalid line range: %d-%d", startLine, endLine)
	}

	// 安全检查
	absPath, err := w.sanitizePath(path)
	if err != nil {
		return nil, false, fmt.Errorf("path security check failed: %w", err)
	}

	// 检查文件扩展名
	if w.isBlockedExtension(absPath) {
		return nil, false, fmt.Errorf("file extension is blocked")
	}

	// 打开文件
	file, err := os.Open(absPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	stat, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	// 如果文件大于 20KB，要求分页
	const maxFragmentSize = 20 * 1024
	if fileSize > maxFragmentSize {
		// 检查请求的行范围跨度是否过大（例如超过 200 行）
		lineCount := endLine - startLine + 1
		if lineCount > 200 {
			return nil, false, fmt.Errorf("file size exceeds 20KB and requested line range too large (%d lines), please paginate", lineCount)
		}
	}

	// 流式读取每一行
	scanner := bufio.NewScanner(file)
	currentLine := 1
	var result []string

	for scanner.Scan() {
		// 上下文取消检查
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		default:
		}

		if currentLine >= startLine && currentLine <= endLine {
			result = append(result, scanner.Text())
		}
		if currentLine >= endLine {
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("scan error: %w", err)
	}

	// 返回实际读取的行数少于请求时，标记 truncated
	truncated = currentLine < endLine

	return result, truncated, nil
}
