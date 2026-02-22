package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-go-mcp/internal/config"
)

func TestOSWorkspace_InspectWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir: tmpDir,
	}
	ws, _ := NewOSWorkspace(cfg)

	// 创建目录结构
	os.MkdirAll(filepath.Join(tmpDir, "src", "app"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("readme"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "app", "utils.go"), []byte("package app"), 0644)
	// 创建应忽略的目录
	os.MkdirAll(filepath.Join(tmpDir, ".git", "objects"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "node_modules", "pkg"), 0755)

	nodes, err := ws.InspectWorkspace(context.Background(), ".", 3)
	if err != nil {
		t.Fatalf("InspectWorkspace failed: %v", err)
	}

	// 检查是否包含预期文件（使用跨平台路径）
	paths := make(map[string]bool)
	for _, n := range nodes {
		paths[n.Path] = true
	}

	expected := []string{
		"README.md",
		filepath.FromSlash("src"),
		filepath.FromSlash("src/main.go"),
		filepath.FromSlash("src/app"),
		filepath.FromSlash("src/app/utils.go"),
	}
	for _, p := range expected {
		if !paths[p] {
			t.Errorf("missing path: %s", p)
		}
	}

	// 确认忽略目录不在结果中
	if paths[".git"] {
		t.Error(".git should be ignored")
	}
	if paths["node_modules"] {
		t.Error("node_modules should be ignored")
	}
}

func TestOSWorkspace_ReadCodeFragment(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir: tmpDir,
	}
	ws, _ := NewOSWorkspace(cfg)

	lines := strings.Split(strings.Repeat("line\n", 100), "\n")
	content := strings.Join(lines, "\n")
	os.WriteFile(filepath.Join(tmpDir, "multi.txt"), []byte(content), 0644)

	// 正常读取
	result, truncated, err := ws.ReadCodeFragment(context.Background(), "multi.txt", 1, 10)
	if err != nil {
		t.Fatalf("ReadCodeFragment failed: %v", err)
	}
	if truncated {
		t.Error("expected not truncated")
	}
	if len(result) != 10 {
		t.Errorf("expected 10 lines, got %d", len(result))
	}
	if result[0] != "line" {
		t.Errorf("unexpected content: %s", result[0])
	}

	// 超出范围应截断
	result, truncated, _ = ws.ReadCodeFragment(context.Background(), "multi.txt", 90, 200)
	if !truncated {
		t.Error("expected truncated")
	}

	// 无效行号
	_, _, err = ws.ReadCodeFragment(context.Background(), "multi.txt", 0, 5)
	if err == nil {
		t.Error("expected error for invalid startLine")
	}
}
