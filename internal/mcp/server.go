package mcp

import (
	"context"
	"fmt"

	"opencode-go-mcp/internal/log"
	"opencode-go-mcp/internal/workspace"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// Server 封装 mcp-golang 服务器
type Server struct {
	ws     workspace.Workspace
	logger log.Logger
	server *mcp.Server
}

// NewServer 创建 MCP 服务器并注册所有工具
func NewServer(ws workspace.Workspace, logger log.Logger) (*Server, error) {
	transport := stdio.NewStdioServerTransport()
	mcpSrv := mcp.NewServer(transport)

	// 注册所有工具
	if err := registerTools(mcpSrv, ws, logger); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return &Server{
		ws:     ws,
		logger: logger,
		server: mcpSrv,
	}, nil
}

// 参数结构体定义（本地模式，无 Project 参数）

type ReadFileArgs struct {
	Path     string `json:"path" jsonschema:"required,description=File path to read"`
	MaxBytes int64  `json:"maxBytes" jsonschema:"description=Maximum bytes to read (default 1MB)"`
}

type WriteFileArgs struct {
	Path        string `json:"path" jsonschema:"required,description=File path to write"`
	Content     string `json:"content" jsonschema:"required,description=Content to write"`
	AllowCreate bool   `json:"allowCreate" jsonschema:"description=Allow creating new file"`
}

type HealthArgs struct{}

// RunSTDIO 启动服务器
func (s *Server) RunSTDIO(ctx context.Context) error {
	// 启动 Serve（非阻塞，内部启动 readLoop）
	go func() {
		if err := s.server.Serve(); err != nil {
			s.logger.Error(ctx, "MCP server error", "error", err)
		}
	}()

	// 等待上下文取消（如收到中断信号）
	<-ctx.Done()
	return nil
}
