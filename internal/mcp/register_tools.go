package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opencode-go-mcp/internal/log"
	"opencode-go-mcp/internal/workspace"

	mcp "github.com/metoro-io/mcp-golang"
)

// registerTools 注册所有 MCP 工具（本地模式，无 Project 参数）
func registerTools(srv *mcp.Server, ws workspace.Workspace, logger log.Logger, onActivity func()) error {
	// workspace.read_file tool
	if err := srv.RegisterTool("workspace.read_file", "Read a file from local workspace", func(args ReadFileArgs) (*mcp.ToolResponse, error) {
		onActivity()
		maxBytes := args.MaxBytes
		if maxBytes <= 0 {
			maxBytes = 1024 * 1024
		}
		data, err := ws.ReadFile(context.Background(), args.Path, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("read_file: %w", err)
		}
		content := string(data)
		truncated := int64(len(data)) >= maxBytes
		result := fmt.Sprintf("File: %s\nTruncated: %v\n\n%s", args.Path, truncated, content)
		return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
	}); err != nil {
		return fmt.Errorf("failed to register read_file: %w", err)
	}

	// workspace.write_file tool
	if err := srv.RegisterTool("workspace.write_file", "Write content to a file", func(args WriteFileArgs) (*mcp.ToolResponse, error) {
		onActivity()
		err := ws.WriteFile(context.Background(), args.Path, []byte(args.Content), args.AllowCreate)
		if err != nil {
			return nil, fmt.Errorf("write_file: %w", err)
		}
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Wrote: %s", args.Path))), nil
	}); err != nil {
		return fmt.Errorf("failed to register write_file: %w", err)
	}

	// workspace.health tool
	if err := srv.RegisterTool("workspace.health", "Health check with tool list", func(args HealthArgs) (*mcp.ToolResponse, error) {
		onActivity()
		tools := []string{
			"workspace.read_file", "workspace.write_file", "workspace.inspect_workspace",
			"workspace.read_code_fragment", "workspace.apply_unified_diff", "workspace.search_and_replace",
			"workspace.secure_exec", "workspace.health",
		}
		result := map[string]interface{}{
			"version": "0.3.0-local",
			"tools":   tools,
			"status":  "ok",
		}
		jsonResult, _ := json.Marshal(result)
		return mcp.NewToolResponse(mcp.NewTextContent(string(jsonResult))), nil
	}); err != nil {
		return fmt.Errorf("failed to register health: %w", err)
	}

	// Eyes: workspace.inspect_workspace
	if err := srv.RegisterTool("workspace.inspect_workspace", "Inspect workspace directory structure", func(args InspectWorkspaceArgs) (*mcp.ToolResponse, error) {
		onActivity()
		maxDepth := args.MaxDepth
		if maxDepth <= 0 {
			maxDepth = 2
		}
		relPath := args.Path
		if relPath == "" {
			relPath = "."
		}

		osw, ok := ws.(*workspace.OSWorkspace)
		if !ok {
			return nil, fmt.Errorf("workspace does not support InspectWorkspace")
		}
		nodes, err := osw.InspectWorkspace(context.Background(), relPath, maxDepth)
		if err != nil {
			return nil, fmt.Errorf("inspect_workspace: %w", err)
		}

		type SerializableNode struct {
			Path    string `json:"path"`
			IsDir   bool   `json:"is_dir"`
			Size    int64  `json:"size"`
			ModTime string `json:"mod_time"`
		}

		result := make([]SerializableNode, len(nodes))
		for i, n := range nodes {
			result[i] = SerializableNode{
				Path:    n.Path,
				IsDir:   n.IsDir,
				Size:    n.Size,
				ModTime: n.ModTime.Format(time.RFC3339),
			}
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
	}); err != nil {
		return fmt.Errorf("failed to register inspect_workspace: %w", err)
	}

	// Eyes: workspace.read_code_fragment
	if err := srv.RegisterTool("workspace.read_code_fragment", "Read a code fragment by line range", func(args ReadCodeFragmentArgs) (*mcp.ToolResponse, error) {
		onActivity()
		osw, ok := ws.(*workspace.OSWorkspace)
		if !ok {
			return nil, fmt.Errorf("workspace does not support ReadCodeFragment")
		}
		lines, truncated, err := osw.ReadCodeFragment(context.Background(), args.Path, args.StartLine, args.EndLine)
		if err != nil {
			return nil, fmt.Errorf("read_code_fragment: %w", err)
		}
		content := strings.Join(lines, "\n")
		if truncated {
			content += "\n... [TRUNCATED] ..."
		}
		return mcp.NewToolResponse(mcp.NewTextContent(content)), nil
	}); err != nil {
		return fmt.Errorf("failed to register read_code_fragment: %w", err)
	}

	// Hands: workspace.apply_unified_diff
	if err := srv.RegisterTool("workspace.apply_unified_diff", "Apply a unified diff patch", func(args ApplyUnifiedDiffArgs) (*mcp.ToolResponse, error) {
		onActivity()
		osw, ok := ws.(*workspace.OSWorkspace)
		if !ok {
			return nil, fmt.Errorf("workspace does not support ApplyUnifiedDiff")
		}
		applied, err := osw.ApplyUnifiedDiff(context.Background(), args.DiffText, args.DryRun)
		if err != nil {
			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Error: %s", err.Error()))), nil
		}
		msg := fmt.Sprintf("Applied to %d files: %v", len(applied), applied)
		if args.DryRun {
			msg = fmt.Sprintf("Dry-run: patch would be applied to %d files: %v", len(applied), applied)
		}
		return mcp.NewToolResponse(mcp.NewTextContent(msg)), nil
	}); err != nil {
		return fmt.Errorf("failed to register apply_unified_diff: %w", err)
	}

	// Hands: workspace.search_and_replace
	if err := srv.RegisterTool("workspace.search_and_replace", "Search and replace exact string", func(args SearchAndReplaceArgs) (*mcp.ToolResponse, error) {
		onActivity()
		osw, ok := ws.(*workspace.OSWorkspace)
		if !ok {
			return nil, fmt.Errorf("workspace does not support SearchAndReplace")
		}
		actual, err := osw.SearchAndReplace(context.Background(), args.Path, args.Old, args.New, args.ExpectedOccurrences)
		if err != nil {
			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Error: %s", err.Error()))), nil
		}
		msg := fmt.Sprintf("Replaced %d occurrences in %s", actual, args.Path)
		if args.ExpectedOccurrences == 0 {
			msg = fmt.Sprintf("Found %d occurrences (dry-run, no changes)", actual)
		}
		return mcp.NewToolResponse(mcp.NewTextContent(msg)), nil
	}); err != nil {
		return fmt.Errorf("failed to register search_and_replace: %w", err)
	}

	// Shield: workspace.secure_exec
	if err := srv.RegisterTool("workspace.secure_exec", "Execute a command securely with timeout", func(args SecuredExecArgs) (*mcp.ToolResponse, error) {
		onActivity()
		stdout, stderr, exitCode, err := ws.Execute(context.Background(), args.Command, args.Args, args.TimeoutSeconds)
		stdout = workspace.TruncateOutputString(stdout, 2000)
		stderr = workspace.TruncateOutputString(stderr, 2000)

		if err != nil {
			msg := fmt.Sprintf("Exit Code: %d\nSTDOUT:\n%s\nSTDERR:\n%s\nError: %s", exitCode, stdout, stderr, err.Error())
			return mcp.NewToolResponse(mcp.NewTextContent(msg)), nil
		}

		result := fmt.Sprintf("Exit Code: %d\nSTDOUT:\n%s\nSTDERR:\n%s", exitCode, stdout, stderr)
		return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
	}); err != nil {
		return fmt.Errorf("failed to register secure_exec: %w", err)
	}

	return nil
}

// 参数结构体（用于 Eyes 工具）
type InspectWorkspaceArgs struct {
	Path     string `json:"path" jsonschema:"description=Relative path to inspect (default root)"`
	MaxDepth int    `json:"maxDepth" jsonschema:"description=Recursion depth (default 2)"`
}

type ReadCodeFragmentArgs struct {
	Path      string `json:"path" jsonschema:"required,description=File path to read"`
	StartLine int    `json:"startLine" jsonschema:"required,description=Start line (1-indexed)"`
	EndLine   int    `json:"endLine" jsonschema:"required,description=End line (inclusive)"`
}

// 参数结构体（用于 Hands 工具）
type ApplyUnifiedDiffArgs struct {
	DiffText string `json:"diffText" jsonschema:"required,description=Unified diff content"`
	DryRun   bool   `json:"dryRun" jsonschema:"description=Preview only without applying"`
}

type SearchAndReplaceArgs struct {
	Path                string `json:"path" jsonschema:"required,description=File path to modify"`
	Old                 string `json:"old" jsonschema:"required,description=String to search for"`
	New                 string `json:"new" jsonschema:"required,description=Replacement string"`
	ExpectedOccurrences int    `json:"expectedOccurrences" jsonschema:"description=Expected number of occurrences (0 for dry-run)"`
}

// 参数结构体（用于 Shield 工具）
type SecuredExecArgs struct {
	Command        string   `json:"command" jsonschema:"required,description=Command to execute"`
	Args           []string `json:"args" jsonschema:"description=Command arguments"`
	TimeoutSeconds int64    `json:"timeoutSeconds" jsonschema:"description=Timeout in seconds (0 for default)"`
}
