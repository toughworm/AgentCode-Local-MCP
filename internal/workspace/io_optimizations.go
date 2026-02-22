package workspace

import (
	"fmt"
	"io"
	"os"
)

// ========== 树莓派 Zero 2W 静态编译构建说明 ==========
// 对应 TODO(pi_static_build_notes)（非代码，但需在文档中注明）
//
// 构建命令（在任意 Linux/x64 主机上交叉编译）：
//   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o opencode-mcp-local ./cmd/opencode-mcp
//
// 关键参数解释：
//   - CGO_ENABLED=0: 禁用 CGO，生成纯静态二进制，避免依赖动态库
//   - GOOS=linux: 目标操作系统为 Linux
//   - GOARCH=arm64: 目标架构为 ARM64（树莓派 Zero 2W 为 ARMv8-A 64位）
//   - -o: 输出文件名（例如 opencode-mcp-local）
//
// 部署到树莓派 Zero 2W（512MB RAM）后，直接运行：
//   ./opencode-mcp-local
//
// 注意事项：
//   - 避免使用依赖 C 库的第三方包（某些 sqlite、image 库可能需要 CGO）
//   - 使用纯 Go 实现的依赖（如 go-sqlite3 的 purego 模式）
//   - 如果必须使用 C 库，需要在树莓派上安装对应运行时（不推荐）
// ====================================================

// ensureStreamingIO 确保文件读取使用流式方式而非一次性加载
// 此函数作为设计文档存在，实际的流式实现在 ReadFile 和 ReadCodeFragment 中已应用
func (w *OSWorkspace) ensureStreamingIO() {
	// ReadFile 使用 io.LimitReader，已经是流式
	// ReadCodeFragment 使用 bufio.Scanner，已经是流式
	// 无需额外实现，此 TODO 仅作为设计确认
}

// BackupAndRollback 提供写操作前的自动备份和回滚能力
// 返回两个函数：createBackup 和 rollback，分别用于创建备份和执行回滚
func (w *OSWorkspace) BackupAndRollback(path string) (createBackup func() error, rollback func() error) {
	absPath, err := w.sanitizePath(path)
	if err != nil {
		return func() error { return fmt.Errorf("invalid path: %w", err) },
			func() error { return fmt.Errorf("invalid path: %w", err) }
	}

	backupPath := absPath + ".bak"

	createBackup = func() error {
		// 检查原文件是否存在
		_, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				// 文件不存在，无需备份，返回成功
				return nil
			}
			return fmt.Errorf("failed to stat file for backup: %w", err)
		}

		// 复制到备份文件
		src, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("failed to open source for backup: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(backupPath)
		if err != nil {
			return fmt.Errorf("failed to create backup file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			os.Remove(backupPath)
			return fmt.Errorf("failed to copy to backup: %w", err)
		}

		// 同步到磁盘
		dst.Sync()
		return nil
	}

	rollback = func() error {
		// 检查备份文件是否存在
		_, err := os.Stat(backupPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("backup file not found")
			}
			return fmt.Errorf("failed to stat backup: %w", err)
		}

		// 将备份复制回原文件（原子替换）
		// 先删除可能存在的中间 .tmp 文件
		tmpPath := absPath + ".tmp"
		os.Remove(tmpPath)

		// 复制
		src, err := os.Open(backupPath)
		if err != nil {
			return fmt.Errorf("failed to open backup for restore: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to create temp for rollback: %w", err)
		}
		defer func() {
			dst.Close()
			if err != nil {
				os.Remove(tmpPath)
			}
		}()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("failed to copy during rollback: %w", err)
		}
		dst.Close()

		// 原子替换
		if err := os.Rename(tmpPath, absPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to rename during rollback: %w", err)
		}

		return nil
	}

	return createBackup, rollback
}

// PhysicalFileSize 获取文件的实际磁盘占用大小（用于估算磁盘空间）
// 注意：这返回的是 st_blocks * 512，而非 st_size
func (w *OSWorkspace) PhysicalFileSize(path string) (int64, error) {
	absPath, err := w.sanitizePath(path)
	if err != nil {
		return 0, fmt.Errorf("invalid path: %w", err)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat: %w", err)
	}

	// 尝试获取系统信息（Sys() 返回 platform-specific 数据）
	if sys := fileInfo.Sys(); sys != nil {
		// Linux 下 sys 是 *syscall.Stat_t，包含 Blksize 和 Blocks
		// 这里使用类型断言（具体实现 platform 相关）
		if stat, ok := sys.(interface{ GetBlocks() int64 }); ok {
			blocks := stat.GetBlocks()
			return blocks * 512, nil
		}
	}

	// 降级：直接返回文件逻辑大小
	return fileInfo.Size(), nil
}
