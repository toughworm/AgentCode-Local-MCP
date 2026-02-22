package e2e

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEndToEnd_MCPProcess 端到端测试：构建二进制并通过 MCP 协议交互
func TestEndToEnd_MCPProcess(t *testing.T) {
	t.Helper()

	// 构建 MCP 服务器到临时目录，避免依赖固定路径
	tmpDir := t.TempDir()
	binName := "opencode-mcp-e2e-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/opencode-mcp")
	// 从 e2e 子目录回到项目根目录
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, string(out))
	}
	defer os.Remove(binPath)

	// 启动 MCP 进程
	cmd := exec.Command(binPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe failed: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe failed: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer cmd.Process.Kill()

	// 辅助发送请求并读取响应
	sendRequest := func(req map[string]interface{}) (map[string]interface{}, error) {
		data, _ := json.Marshal(req)
		if _, err := stdin.Write(append(data, '\n')); err != nil {
			return nil, err
		}

		reader := bufio.NewReader(stdout)
		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      interface{}     `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.NewDecoder(reader).Decode(&resp); err != nil {
			return nil, err
		}
		if resp.Error != nil && len(resp.Error) > 0 {
			return nil, nil
		}
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// 1. 测试 initialize
	initResp, err := sendRequest(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	if _, ok := initResp["protocolVersion"]; !ok {
		t.Fatalf("initialize response missing protocolVersion")
	}

	// 2. 测试 tools/list
	listResp, err := sendRequest(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	tools, _ := listResp["tools"].([]interface{})
	if len(tools) == 0 {
		t.Fatal("no tools listed")
	}

	// 3. 测试 workspace.health
	healthResp, err := sendRequest(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "workspace.health",
			"arguments": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("workspace.health failed: %v", err)
	}
	contentArr, ok := healthResp["content"].([]interface{})
	if !ok || len(contentArr) == 0 {
		t.Fatalf("unexpected health response: %+v", healthResp)
	}
	textObj, ok := contentArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content element: %+v", contentArr[0])
	}
	text, _ := textObj["text"].(string)
	if !strings.Contains(text, `"version"`) {
		t.Fatalf("health response missing version: %s", text)
	}

	t.Log("✅ end-to-end test passed")
}

