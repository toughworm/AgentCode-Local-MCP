package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// TODO(shield_file_overview):
//  本文件实现“受控执行模块（The Shield）”：
//  1. secureExec：在 Workspace.Execute 之上增加命令白名单和超时控制逻辑，并对输出做安全截断。
//  2. TruncateOutputString：对 stdout/stderr 做长度截断，只保留头尾，减少大模型上下文占用。
//  使用本文件时，应确保所有命令执行都通过 secureExec 或 Execute，避免绕过安全策略。

// secureExec 执行命令，封装 os/exec 并应用安全策略
// 参数 timeoutSeconds <= 0 则使用配置中的 BuildTimeout
// TODO(shield_secure_exec_impl):
//  1. 首先使用 w.isAllowedCommand(cmd) 校验命令前缀是否在白名单中。
//  2. 计算真正使用的超时时间：
//     - timeoutSeconds > 0 时使用该值；
//     - 否则使用 cfg.BuildTimeoutSeconds。
//  3. 使用 context.WithTimeout 创建 ctxWithTimeout 并传递给 w.Execute。
//  4. 若 w.Execute 返回错误且 ctxWithTimeout.Err() == context.DeadlineExceeded：
//     - 将错误包装为 “timeout after ...” 形式，明确告知调用方。
//  5. 对返回的 stdout/stderr 调用 truncateOutputString 进行截断，默认上限可设为 2000 字符。
//  6. 最终返回截断后的 stdout/stderr 和 exitCode。
func (w *OSWorkspace) secureExec(ctx context.Context, cmd string, args []string, timeoutSeconds int64) (stdout string, stderr string, exitCode int, err error) {
	// 命令白名单校验
	if !w.isAllowedCommand(cmd) {
		return "", "", 0, fmt.Errorf("command not allowed: %s", cmd)
	}

	// 确定超时
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(w.cfg.BuildTimeout) * time.Second
	}

	// 创建带超时的 context
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 构建完整命令字符串（用于白名单校验，args 会附加）
	fullCmd := cmd
	if len(args) > 0 {
		fullCmd = cmd + " " + strings.Join(args, " ")
	}

	// 再次校验完整命令（用于如 "go build" 之类的复合命令）
	if !w.isAllowedCommand(fullCmd) {
		return "", "", 0, fmt.Errorf("command not allowed: %s", fullCmd)
	}

	// 执行命令
	stdout, stderr, exitCode, err = w.Execute(ctxWithTimeout, cmd, args, timeoutSeconds)
	if err != nil {
		// 区分超时错误
		if ctxWithTimeout.Err() == context.DeadlineExceeded {
			return stdout, stderr, exitCode, fmt.Errorf("command timeout after %v: %w", timeout, ctxWithTimeout.Err())
		}
		return stdout, stderr, exitCode, fmt.Errorf("command execution failed: %w", err)
	}

	// 输出截断处理
	stdout = TruncateOutputString(stdout, 2000)
	stderr = TruncateOutputString(stderr, 2000)

	return stdout, stderr, exitCode, nil
}

// TruncateOutputString 截断输出，保留前半和后半，中间用占位符
// TODO(shield_truncate_output_impl):
//  1. 当 maxLen <= 0 或 s 长度不超过 maxLen 时，直接返回 s。
//  2. 计算一半长度 half := maxLen / 2，将 head = s[:half]，tail = s[len(s)-half:]。
//  3. 为避免截断到多字节字符中间，可使用 TrimRightFunc/TrimLeftFunc 做粗略修正：
//     - 将 head 末尾的非 ASCII 字符去掉；
//     - 将 tail 开头的非 ASCII 字符去掉。
//  4. 返回 head + "\n... [TRUNCATED] ...\n" + tail。
func TruncateOutputString(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}

	// 截断标记，固定占位
	marker := "\n... [TRUNCATED] ...\n"
	markerLen := len(marker)

	// 如果 maxLen 太小，无法容纳标记，则只返回标记
	if maxLen <= markerLen {
		return marker[:maxLen]
	}

	// 计算头和尾的可用长度，确保总长度 <= maxLen
	avail := maxLen - markerLen
	half := avail / 2
	head := s[:half]
	tail := s[len(s)-half:]

	// 调整到字符边界，避免切分 UTF-8 序列
	head = strings.TrimRightFunc(head, func(r rune) bool { return r > unicode.MaxASCII })
	tail = strings.TrimLeftFunc(tail, func(r rune) bool { return r > unicode.MaxASCII })

	// 如果调整后总长度可能超过 maxLen（因为移除了部分字节），再次截断
	result := head + marker + tail
	if len(result) > maxLen {
		// 简单处理：再次截断 tail 或 head，但通常不会太大偏差
		excess := len(result) - maxLen
		if len(tail) > excess {
			tail = tail[:len(tail)-excess]
		} else {
			head = head[:len(head)-(excess-len(tail))]
		}
		result = head + marker + tail
	}

	return result
}
