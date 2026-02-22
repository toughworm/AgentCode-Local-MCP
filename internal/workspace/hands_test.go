package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-go-mcp/internal/config"
)

func TestParseUnifiedDiff(t *testing.T) {
	diff := `--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,4 @@
 line1
-line2
+new line2
 line3
`
	patches, err := parseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("parseUnifiedDiff failed: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].FilePath != "test.txt" {
		t.Errorf("filePath = %s, want test.txt", patches[0].FilePath)
	}
	if len(patches[0].Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(patches[0].Hunks))
	}
	hunk := patches[0].Hunks[0]
	if hunk.OldStart != 1 || hunk.OldCount != 3 || hunk.NewStart != 1 || hunk.NewCount != 4 {
		t.Errorf("hunk header mismatch: %+v", hunk)
	}
}

func TestApplyPatchToContent(t *testing.T) {
	original := []byte("line1\nline2\nline3\n")
	diff := `--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 line1
-line2
+new line2
 line3
`
	patches, _ := parseUnifiedDiff(diff)
	patched, err := applyPatchToContent(original, patches[0])
	if err != nil {
		t.Fatalf("applyPatchToContent failed: %v", err)
	}
	expected := "line1\nnew line2\nline3\n"
	if string(patched) != expected {
		t.Errorf("patched = %q, want %q", string(patched), expected)
	}
}

func TestOSWorkspace_ApplyUnifiedDiff(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir: tmpDir,
	}
	ws, _ := NewOSWorkspace(cfg)

	// 创建目标文件
	target := filepath.Join(tmpDir, "target.txt")
	os.WriteFile(target, []byte("line1\nline2\nline3\n"), 0644)

	diff := `--- a/target.txt
+++ b/target.txt
@@ -1,3 +1,3 @@
 line1
-line2
+new line2
 line3
`
	files, err := ws.ApplyUnifiedDiff(context.Background(), diff, false)
	if err != nil {
		t.Fatalf("ApplyUnifiedDiff failed: %v", err)
	}
	if len(files) != 1 || files[0] != "target.txt" {
		t.Errorf("applied files = %v", files)
	}

	content, _ := os.ReadFile(target)
	expected := "line1\nnew line2\nline3\n"
	if string(content) != expected {
		t.Errorf("final content = %q, want %q", string(content), expected)
	}
}

func TestOSWorkspace_SearchAndReplace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		RootDir: tmpDir,
	}
	ws, _ := NewOSWorkspace(cfg)

	file := filepath.Join(tmpDir, "swap.txt")
	os.WriteFile(file, []byte("foo bar foo baz foo"), 0644)

	// dry-run: 仅统计
	count, err := ws.SearchAndReplace(context.Background(), "swap.txt", "foo", "qux", 0)
	if err != nil {
		t.Fatalf("SearchAndReplace dry-run failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 occurrences, got %d", count)
	}

	// 实际替换
	count, err = ws.SearchAndReplace(context.Background(), "swap.txt", "foo", "qux", 3)
	if err != nil {
		t.Fatalf("SearchAndReplace replace failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "qux") {
		t.Error("replacement not applied")
	}

	// 次数不匹配应失败
	_, err = ws.SearchAndReplace(context.Background(), "swap.txt", "qux", "foo", 2)
	if err == nil {
		t.Error("expected mismatch error")
	}
}
