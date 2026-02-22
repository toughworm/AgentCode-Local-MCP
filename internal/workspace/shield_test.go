package workspace

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"opencode-go-mcp/internal/config"
)

func TestTruncateOutputString(t *testing.T) {
	s := strings.Repeat("abcdefghij", 500) // 5000 chars
	result := TruncateOutputString(s, 100)
	if len(result) > 100 {
		t.Errorf("truncated output too long: %d", len(result))
	}
	if !strings.Contains(result, "[TRUNCATED]") {
		t.Error("missing truncation marker")
	}

	// 短文本应原样返回
	short := "hello"
	if TruncateOutputString(short, 100) != short {
		t.Error("short string modified")
	}
}

func TestOSWorkspace_SecureExec(t *testing.T) {
	tmpDir := t.TempDir()

	// 根据操作系统选择合适的命令
	var cmdName string
	var baseArgs []string
	if runtime.GOOS == "windows" {
		cmdName = "cmd"
		baseArgs = []string{"/C", "echo"}
	} else {
		cmdName = "echo"
		baseArgs = []string{}
	}

	cfg := &config.Config{
		RootDir:             tmpDir,
		BuildTimeout:        5,
		AllowedBuildCommands: []string{cmdName},
	}
	ws, _ := NewOSWorkspace(cfg)

	// 安全命令应执行
	args := append(append([]string{}, baseArgs...), "test")
	stdout, _, exitCode, err := ws.SecureExec(context.Background(), cmdName, args, 0)
	if err != nil {
		t.Fatalf("SecureExec failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout, "test") {
		t.Errorf("stdout missing output: %s", stdout)
	}

	// 输出应被截断（如果很长）
	long := strings.Repeat("x", 5000)
	longArgs := append(append([]string{}, baseArgs...), long)
	stdout, _, _, _ = ws.SecureExec(context.Background(), cmdName, longArgs, 0)
	if len(stdout) > 2000 {
		t.Errorf("output not truncated: %d", len(stdout))
	}
}
