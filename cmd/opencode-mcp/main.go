package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"opencode-go-mcp/internal/config"
	"opencode-go-mcp/internal/log"
	"opencode-go-mcp/internal/mcp"
	"opencode-go-mcp/internal/workspace"
)

func main() {
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString(fmt.Sprintf("Error: %v\n", err))
		os.Exit(1)
	}
}

// loadConfigAndLogger 从配置文件和环境变量加载配置，验证并创建日志器。
func loadConfigAndLogger() (*config.Config, log.Logger, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}

	logger := log.NewStdLogger(cfg.LogLevel)
	return cfg, logger, nil
}

// run 初始化并运行服务，处理中断信号。
func run() error {
	cfg, logger, err := loadConfigAndLogger()
	if err != nil {
		return err
	}

	// 打印配置来源（调试用）
	if cfg.ConfigFile != "" {
		logger.Info(context.Background(), "Loaded config from file", "path", cfg.ConfigFile)
	} else {
		logger.Info(context.Background(), "Using environment variables only")
	}

	// 创建本地工作区
	ws, err := workspace.NewOSWorkspace(cfg)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// 创建可取消的上下文，监听中断信号
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 创建 MCP 服务器
	server, err := mcp.NewServer(ws, logger)
	if err != nil {
		return fmt.Errorf("failed to create mcp server: %w", err)
	}

	// 启动 stdio 循环（阻塞直到上下文取消）
	if err := server.RunSTDIO(ctx); err != nil {
		return fmt.Errorf("mcp server error: %w", err)
	}

	return nil
}
