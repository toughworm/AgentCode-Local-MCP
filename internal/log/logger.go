package log

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// Logger 接口定义
type Logger interface {
	Debug(ctx context.Context, msg string, kv ...any)
	Info(ctx context.Context, msg string, kv ...any)
	Warn(ctx context.Context, msg string, kv ...any)
	Error(ctx context.Context, msg string, kv ...any)
}

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// 级别字符串映射
var levelStrings = map[Level]string{
	LevelDebug: "debug",
	LevelInfo:  "info",
	LevelWarn:  "warn",
	LevelError: "error",
}

// 字符串到级别的反向映射（忽略大小写）
var stringLevels = map[string]Level{
	"debug": LevelDebug,
	"info":  LevelInfo,
	"warn":  LevelWarn,
	"error": LevelError,
}

// StdLogger 基于标准库 log 的默认实现
type StdLogger struct {
	level Level
}

// NewStdLogger 解析日志级别字符串，不合法时回退到 info；返回线程安全的 Logger 实例，将日志输出到 stderr。
func NewStdLogger(levelStr string) Logger {
	lvl := parseLevel(levelStr)
	return &StdLogger{
		level: lvl,
	}
}

// parseLevel 解析日志级别字符串（忽略大小写），未知时返回 LevelInfo
func parseLevel(s string) Level {
	s = strings.ToLower(strings.TrimSpace(s))
	if lvl, ok := stringLevels[s]; ok {
		return lvl
	}
	return LevelInfo
}

// shouldLog 判断给定级别是否满足当前日志级别
func (l *StdLogger) shouldLog(level Level) bool {
	return level >= l.level
}

// formatMessage 格式化日志消息，包含键值对
func formatMessage(msg string, kv ...any) string {
	if len(kv) == 0 {
		return msg
	}
	// 确保成对出现，奇数则去掉最后一个
	pairs := len(kv) / 2 * 2
	kv = kv[:pairs]

	// 构建键值对字符串
	parts := make([]string, 0, pairs/2+1)
	parts = append(parts, msg)
	for i := 0; i < pairs; i += 2 {
		key := fmt.Sprintf("%v", kv[i])
		value := fmt.Sprintf("%v", kv[i+1])
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, " ")
}

// Debug 实现
func (l *StdLogger) Debug(ctx context.Context, msg string, kv ...any) {
	if l.shouldLog(LevelDebug) {
		log.New(os.Stderr, "[DEBUG] ", log.LstdFlags).Println(formatMessage(msg, kv...))
	}
}

// Info 实现
func (l *StdLogger) Info(ctx context.Context, msg string, kv ...any) {
	if l.shouldLog(LevelInfo) {
		log.New(os.Stderr, "[INFO] ", log.LstdFlags).Println(formatMessage(msg, kv...))
	}
}

// Warn 实现
func (l *StdLogger) Warn(ctx context.Context, msg string, kv ...any) {
	if l.shouldLog(LevelWarn) {
		log.New(os.Stderr, "[WARN] ", log.LstdFlags).Println(formatMessage(msg, kv...))
	}
}

// Error 实现
func (l *StdLogger) Error(ctx context.Context, msg string, kv ...any) {
	if l.shouldLog(LevelError) {
		log.New(os.Stderr, "[ERROR] ", log.LstdFlags).Println(formatMessage(msg, kv...))
	}
}


