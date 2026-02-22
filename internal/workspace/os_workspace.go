package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"opencode-go-mcp/internal/config"
)

// TODO(logic_workspace_os_imports):
//  1. 该文件只需要依赖 Go 标准库（context/os/os.exec/path.filepath/strings/time/io）。
//  2. 等实现阶段，请由初级开发在本文件顶部显式引入内部配置包：
//     import "opencode-go-mcp/internal/config"
//  3. 严禁重新引入任何 OpenCode SDK 或远程客户端相关包。

// OSWorkspace 基于本地文件系统和 os/exec 的 Workspace 实现
type OSWorkspace struct {
	root            string         // 工作区根目录的绝对路径（优先使用 cfg.RootDir）
	allowedCommands []string       // 允许执行的命令前缀白名单（来自 cfg.AllowedBuildCommands）
	cfg             *config.Config // 全局配置（只使用本地模式字段）
}

// TODO(logic_workspace_os_struct):
//  1. 确认 config.Config 中本地模式字段为：
//     - RootDir string
//     - AllowedBuildCommands []string
//     - MaxFileBytes int64
//     - AllowedPaths []string
//     - BlockedExtensions []string
//     - BuildTimeoutSeconds int64
//  2. OSWorkspace 只保存 root/allowedCommands/cfg 三个字段，避免存临时状态。

// NewOSWorkspace 创建新的 OSWorkspace 实例
func NewOSWorkspace(cfg *config.Config) (Workspace, error) {
	// 1. 处理 root 目录
	var root string
	var err error

	if cfg.RootDir != "" {
		root, err = filepath.Abs(cfg.RootDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve root dir %q: %w", cfg.RootDir, err)
		}
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		root, err = filepath.Abs(wd)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve working dir: %w", err)
		}
	}

	// 2. 处理 allowedCommands
	allowedCommands := make([]string, len(cfg.AllowedBuildCommands))
	copy(allowedCommands, cfg.AllowedBuildCommands)

	// 如果允许的命令列表为空，使用安全默认值
	if len(allowedCommands) == 0 {
		allowedCommands = []string{"go", "go test", "go build", "go run"}
	}

	// 3. 返回实例
	return &OSWorkspace{
		root:            root,
		allowedCommands: allowedCommands,
		cfg:             cfg,
	}, nil
}

// ReadFile 读取文件内容，支持最大字节限制和上下文取消
func (w *OSWorkspace) ReadFile(ctx context.Context, path string, maxBytes int64) ([]byte, error) {
	// 1. 路径安全与扩展名检查
	absPath, err := w.sanitizePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", path, err)
	}
	if w.isBlockedExtension(absPath) {
		return nil, fmt.Errorf("extension blocked for file %q", absPath)
	}
	
	// 2. 确定读取限制
	if maxBytes <= 0 {
		maxBytes = w.cfg.MaxFileBytes
	}
	
	// 3. 打开文件
	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", absPath, err)
	}
	defer file.Close()
	
	// 4. 使用 LimitedReader 限制读取量
	limited := io.LimitReader(file, maxBytes)
	
	// 5. 分块读取，定期检查上下文取消
	const bufSize = 32 * 1024 // 32KB
	buf := make([]byte, bufSize)
	var result []byte
	totalRead := int64(0)
	
	for {
		// 检查上下文
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		n, err := limited.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
			totalRead += int64(n)
		}
		
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read error: %w", err)
		}
		
		if totalRead >= maxBytes {
			break
		}
	}
	
	// 6. 获取文件真实大小，判断是否被截断
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %q: %w", absPath, err)
	}
	
	if fileInfo.Size() > maxBytes {
		return result, fmt.Errorf("file truncated (original size %d bytes, read %d bytes)", fileInfo.Size(), totalRead)
	}
	
	return result, nil
}

// WriteFile 写入文件，原子操作，支持创建/覆盖
func (w *OSWorkspace) WriteFile(ctx context.Context, path string, data []byte, allowCreate bool) error {
	// 1. 路径安全与扩展名检查
	absPath, err := w.sanitizePath(path)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", path, err)
	}
	if w.isBlockedExtension(absPath) {
		return fmt.Errorf("extension blocked for file %q", absPath)
	}
	
	// 2. 确保父目录存在（不允许自动创建，由 Agent 显式处理）
	dir := filepath.Dir(absPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory %q does not exist", dir)
	} else if err != nil {
		return fmt.Errorf("failed to stat parent directory %q: %w", dir, err)
	}
	
	// 3. 原子写入：临时文件 + rename
	tmpPath := absPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file %q: %w", tmpPath, err)
	}
	
	// 检查上下文取消（在写入前）
	select {
	case <-ctx.Done():
		tmpFile.Close()
		os.Remove(tmpPath)
		return ctx.Err()
	default:
	}
	
	// 写入所有数据
	_, err = tmpFile.Write(data)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write data to temp file: %w", err)
	}
	
	// 确保数据落盘
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	
	// 4. 如果 allowCreate=false，检查目标文件是否存在
	if !allowCreate {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			os.Remove(tmpPath)
			return fmt.Errorf("file %q does not exist and allowCreate is false", absPath)
		} else if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to check target file: %w", err)
		}
	}
	
	// 5. 原子替换
	if err := os.Rename(tmpPath, absPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}
	
	return nil
}

// Execute 执行命令，支持超时和上下文取消，工作目录固定在 root
func (w *OSWorkspace) Execute(ctx context.Context, cmd string, args []string, timeoutSeconds int64) (stdout string, stderr string, exitCode int, err error) {
	// 1. 命令白名单校验
	if !w.isAllowedCommand(cmd) {
		return "", "", -1, fmt.Errorf("command not allowed: %q", cmd)
	}
	
	// 2. 计算超时时间
	timeout := timeoutSeconds
	if timeout <= 0 {
		timeout = w.cfg.BuildTimeout
	}
	timeoutDuration := time.Duration(timeout) * time.Second
	
	// 3. 创建带超时的上下文
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	
	// 4. 创建命令对象，设置工作目录
	execCmd := exec.CommandContext(ctxWithTimeout, cmd, args...)
	execCmd.Dir = w.root
	
	// 5. 捕获 stdout/stderr
	var stdoutBuf, stderrBuf strings.Builder
	execCmd.Stdout = &stdoutBuf
	execCmd.Stderr = &stderrBuf
	
	// 6. 执行
	runErr := execCmd.Run()
	
	// 获取输出
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	
	// 7. 根据错误类型处理
	if runErr != nil {
		// 超时
		if ctxWithTimeout.Err() == context.DeadlineExceeded {
			exitCode = -1
			err = fmt.Errorf("timeout after %v: %w", timeoutDuration, runErr)
			return
		}
		
		// 命令退出码非零
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = fmt.Errorf("exited with code %d: %w", exitCode, runErr)
			return
		}
		
		// 其他错误（如命令不存在）
		exitCode = -1
		err = fmt.Errorf("execution failed: %w", runErr)
		return
	}
	
	// 8. 成功
	exitCode = execCmd.ProcessState.ExitCode()
	err = nil
	return
}

// --- 辅助函数 ---

// sanitizePath 路径安全检查：归一化 + 确保在 root 内 + AllowedPaths 白名单
func (w *OSWorkspace) sanitizePath(path string) (string, error) {
	// 1. 规范化并检查空
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	path = filepath.Clean(path)

	// 2. 构建绝对路径
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(w.root, path)
	}

	// 3. 解析符号链接
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，absPath 本身有效，继续使用
			realPath = absPath
		} else {
			return "", fmt.Errorf("failed to resolve symlinks for %q: %w", path, err)
		}
	} else {
		absPath = realPath
	}

	// 4. 再次清理并检查是否在 root 内
	absPath = filepath.Clean(absPath)
	rel, err := filepath.Rel(w.root, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path for %q: %w", path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace root", path)
	}

	// 5. 检查 AllowedPaths 白名单（如果配置了）
	if len(w.cfg.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPrefix := range w.cfg.AllowedPaths {
			var absAllowed string
			if filepath.IsAbs(allowedPrefix) {
				absAllowed = filepath.Clean(allowedPrefix)
			} else {
				absAllowed = filepath.Join(w.root, allowedPrefix)
			}
			absAllowed = filepath.Clean(absAllowed)
			// 检查 absPath 是否以 absAllowed 为前缀
			if strings.HasPrefix(absPath, absAllowed) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("path %q not in allowed paths list", path)
		}
	}

	return absPath, nil
}

// isBlockedExtension 检查文件扩展名是否在黑名单中
func (w *OSWorkspace) isBlockedExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}
	for _, blocked := range w.cfg.BlockedExtensions {
		if strings.ToLower(blocked) == ext {
			return true
		}
	}
	return false
}

// isAllowedCommand 检查命令是否在白名单中
func (w *OSWorkspace) isAllowedCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	baseCmd := fields[0]
	
	for _, allowed := range w.allowedCommands {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		// 精确匹配基础命令名
		if allowed == baseCmd {
			return true
		}
		// 支持前缀匹配：例如 allowed="go" 可匹配 "go build" 等
		if strings.HasPrefix(cmd, allowed) {
			// 要确保匹配到完整单词（例如 "go" 应匹配 "go build"，但不匹配 "gomod"）
			// 由于 cmd 以 allowed 开头，再检查后面是空白还是完全相等
			if len(cmd) == len(allowed) || cmd[len(allowed)] == ' ' || cmd[len(allowed)] == '\t' {
				return true
			}
		}
	}
	return false
}

// SecureExec 满足 Workspace 接口，提供安全的命令执行
// 它基于 Execute 添加白名单校验和输出截断
func (w *OSWorkspace) SecureExec(ctx context.Context, cmd string, args []string, timeoutSeconds int64) (stdout string, stderr string, exitCode int, err error) {
	// 先执行基础 Execute（已包含白名单检查和超时）
	stdout, stderr, exitCode, err = w.Execute(ctx, cmd, args, timeoutSeconds)
	// 对输出进行截断，避免大上下文
	const maxOutLen = 2000
	stdout = TruncateOutputString(stdout, maxOutLen)
	stderr = TruncateOutputString(stderr, maxOutLen)
	return
}
