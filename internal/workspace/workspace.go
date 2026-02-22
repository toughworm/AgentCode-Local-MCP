package workspace

import "context"

// Workspace 接口抽象了本地代码工作区的核心能力
// 不暴露任何远程或 OpenCode 概念，仅处理本地路径和受控命令
type Workspace interface {
	// ReadFile 读取文件内容，限制最大字节数，返回 []byte 或 error
	ReadFile(ctx context.Context, path string, maxBytes int64) ([]byte, error)

	// WriteFile 写入文件，allowCreate 表示是否允许创建新文件
	WriteFile(ctx context.Context, path string, data []byte, allowCreate bool) error

	// Execute 执行命令，返回 stdout、stderr、exit code 和 error
	Execute(ctx context.Context, cmd string, args []string, timeoutSeconds int64) (stdout string, stderr string, exitCode int, err error)

	// InspectWorkspace 扫描工作区目录树
	InspectWorkspace(ctx context.Context, relPath string, maxDepth int) ([]*TreeNode, error)

	// ReadCodeFragment 按行范围读取文件
	ReadCodeFragment(ctx context.Context, path string, startLine, endLine int) (lines []string, truncated bool, err error)

	// ApplyUnifiedDiff 应用补丁
	ApplyUnifiedDiff(ctx context.Context, diffText string, dryRun bool) (appliedFiles []string, err error)

	// SearchAndReplace 搜索并替换
	SearchAndReplace(ctx context.Context, path, oldStr, newStr string, expectedOccurrences int) (actualOccurrences int, err error)

	// SecureExec 安全执行命令（带白名单和截断）
	SecureExec(ctx context.Context, cmd string, args []string, timeoutSeconds int64) (stdout string, stderr string, exitCode int, err error)

	// PhysicalFileSize 返回文件的物理磁盘占用大小（不需要上下文）
	PhysicalFileSize(path string) (int64, error)
}
