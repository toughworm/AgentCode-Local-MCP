package workspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"opencode-go-mcp/internal/config"
)

func TestOSWorkspace_SanitizePath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir: tmpDir,
	}
	ws, err := NewOSWorkspace(cfg)
	if err != nil {
		t.Fatalf("NewOSWorkspace failed: %v", err)
	}

	// 测试合法相对路径
	relPath := "subdir/file.txt"
	absPath, err := ws.(*OSWorkspace).sanitizePath(relPath)
	if err != nil {
		t.Errorf("sanitizePath(%q) unexpected error: %v", relPath, err)
	} else {
		expected := filepath.Join(tmpDir, relPath)
		if absPath != expected {
			t.Errorf("sanitizePath(%q) = %q, want %q", relPath, absPath, expected)
		}
	}

	// 测试绝对路径（在 root 内）
	absInside := filepath.Join(tmpDir, "file.txt")
	absPath, err = ws.(*OSWorkspace).sanitizePath(absInside)
	if err != nil {
		t.Errorf("sanitizePath(abs inside) error: %v", err)
	} else if absPath != absInside {
		t.Errorf("sanitizePath(abs inside) = %q, want %q", absPath, absInside)
	}

	// 测试逃出 root 的路径
	escapePath := filepath.Join(tmpDir, "..", "etc", "passwd")
	_, err = ws.(*OSWorkspace).sanitizePath(escapePath)
	if err == nil {
		t.Error("sanitizePath(escape) should have failed")
	}
}

func TestOSWorkspace_ReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir:       tmpDir,
		MaxFileBytes:  1024,
		BlockedExtensions: []string{".exe"},
	}
	ws, _ := NewOSWorkspace(cfg)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	content, err := ws.ReadFile(context.Background(), "test.txt", 0)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("content = %q, want %q", string(content), "hello world")
	}

	// 测试大文件截断
	bigContent := strings.Repeat("x", 2000)
	os.WriteFile(filepath.Join(tmpDir, "big.txt"), []byte(bigContent), 0644)
	content, _ = ws.ReadFile(context.Background(), "big.txt", 100)
	if len(content) > 100 {
		t.Errorf("expected truncation to 100 bytes, got %d", len(content))
	}

	// 测试黑名单扩展名
	os.WriteFile(filepath.Join(tmpDir, "test.exe"), []byte("binary"), 0644)
	_, err = ws.ReadFile(context.Background(), "test.exe", 0)
	if err == nil {
		t.Error("expected blocked extension error")
	}
}

func TestOSWorkspace_WriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir:       tmpDir,
		BlockedExtensions: []string{".lock"},
	}
	ws, _ := NewOSWorkspace(cfg)

	// 测试创建新文件
	err := ws.WriteFile(context.Background(), "newfile.txt", []byte("content"), true)
	if err != nil {
		t.Fatalf("WriteFile create failed: %v", err)
	}
	created := filepath.Join(tmpDir, "newfile.txt")
	if _, err := os.Stat(created); err != nil {
		t.Error("file not created")
	}

	// 测试覆写现有文件
	err = ws.WriteFile(context.Background(), "newfile.txt", []byte("new content"), true)
	if err != nil {
		t.Fatalf("WriteFile overwrite failed: %v", err)
	}
	data, _ := os.ReadFile(created)
	if string(data) != "new content" {
		t.Error("file not overwritten")
	}

	// 测试 allowCreate=false 时文件不存在则报错
	err = ws.WriteFile(context.Background(), "missing.txt", []byte("data"), false)
	if err == nil {
		t.Error("expected error for non-existent file with allowCreate=false")
	}

	// 测试黑名单扩展名
	err = ws.WriteFile(context.Background(), "blocked.lock", []byte("bad"), true)
	if err == nil {
		t.Error("expected blocked extension error")
	}
}

func TestOSWorkspace_Execute(t *testing.T) {
	tmpDir := t.TempDir()

	// 根据操作系统选择合适的命令
	var cmdName string
	var cmdArgs []string
	if runtime.GOOS == "windows" {
		cmdName = "cmd"
		cmdArgs = []string{"/C", "echo", "hello"}
	} else {
		cmdName = "echo"
		cmdArgs = []string{"hello"}
	}

	cfg := &config.Config{
		RootDir:             tmpDir,
		AllowedBuildCommands: []string{cmdName},
		BuildTimeout:        10,
	}
	ws, _ := NewOSWorkspace(cfg)

	// 测试简单命令
	stdout, stderr, exitCode, err := ws.Execute(context.Background(), cmdName, cmdArgs, 0)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exitCode 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout = %q, expected to contain 'hello'", stdout)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}

	// 测试非法命令（不在白名单中）
	_, _, _, err = ws.Execute(context.Background(), "forbidden-command", []string{}, 0)
	if err == nil {
		t.Error("expected command not allowed error")
	}

	// 测试超时（通过取消上下文模拟）
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	_, _, _, err = ws.Execute(ctx, cmdName, cmdArgs, 0)
	if err == nil {
		t.Error("expected canceled context error")
	}
}

func TestOSWorkspace_PhysicalFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{RootDir: tmpDir}
	ws, _ := NewOSWorkspace(cfg)

	// 创建文件
	testFile := filepath.Join(tmpDir, "sizetest.txt")
	os.WriteFile(testFile, []byte("1234567890"), 0644)

	size, err := ws.PhysicalFileSize("sizetest.txt")
	if err != nil {
		t.Fatalf("PhysicalFileSize failed: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}
}
