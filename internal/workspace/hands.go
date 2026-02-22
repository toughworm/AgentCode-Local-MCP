package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// TODO(hands_file_overview):
//  本文件实现“精准修改模块（The Hands）”：
//  1. ApplyUnifiedDiff：接收 unified diff 文本，解析为 DiffPatch 结构，对每个文件执行安全校验 + 原子写入。
//  2. SearchAndReplace：对单个文件执行精确字符串替换，支持 expectedOccurrences 一致性校验和 dry-run。
//  3. parseUnifiedDiff / parseHunkHeader / applyPatchToContent 等辅助方法用于解析和应用补丁。
//  参考实现已经提供，但仍需按各 TODO 检查逻辑正确性、错误信息和性能是否满足当前设计。

// ApplyUnifiedDiff 应用 unified diff 补丁
// diffText 是标准 unified diff 格式的文本，dryRun 仅验证不写盘
// TODO(hands_apply_unified_diff_impl):
//  1. 先调用 parseUnifiedDiff(diffText)，将 diff 文本解析为 []DiffPatch。
//  2. 对于每个 patch：
//     - 使用 w.sanitizePath(patch.FilePath) 校验路径安全；
//     - 使用 w.isBlockedExtension 拦截敏感扩展名；
//     - 根据 patch.IsNewFile 判断是否允许新建文件。
//  3. 读取原始文件内容（若存在），调用 applyPatchToContent(original, patch) 得到 patchedContent。
//  4. 若 dryRun = true，仅将 patch.FilePath 追加到 appliedFiles，不写盘。
//  5. 若 dryRun = false：
//     - 在写入前，考虑调用 BackupAndRollback 提供备份能力；
//     - 使用临时文件 + 原子替换的方式写入 patchedContent；
//     - 任何一步出错都应恢复备份并返回详细错误。
//  6. 全程检查 ctx.Done()，支持取消。
func (w *OSWorkspace) ApplyUnifiedDiff(ctx context.Context, diffText string, dryRun bool) (appliedFiles []string, err error) {
	// 解析 diff
	patches, err := parseUnifiedDiff(diffText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	appliedFiles = make([]string, 0, len(patches))

	// 逐个文件处理
	for _, patch := range patches {
		select {
		case <-ctx.Done():
			return appliedFiles, ctx.Err()
		default:
		}

		// 安全检查：目标文件必须在工作区内且不在黑名单
		absPath, err := w.sanitizePath(patch.FilePath)
		if err != nil {
			return appliedFiles, fmt.Errorf("invalid file path %q in diff: %w", patch.FilePath, err)
		}
		if w.isBlockedExtension(absPath) {
			return appliedFiles, fmt.Errorf("blocked extension for file %s", absPath)
		}

		// 检查文件是否存在（除非是新增文件）
		_, statErr := os.Stat(absPath)
		if patch.IsNewFile {
			// 新增文件：允许目标文件不存在；其他 stat 错误需要上抛
			if statErr != nil && !os.IsNotExist(statErr) {
				return appliedFiles, fmt.Errorf("failed to stat target file %s: %w", absPath, statErr)
			}
		} else {
			// 修改已有文件：目标文件必须存在
			if statErr != nil {
				if os.IsNotExist(statErr) {
					return appliedFiles, fmt.Errorf("file %s does not exist (but diff indicates modification)", absPath)
				}
				return appliedFiles, fmt.Errorf("failed to stat target file %s: %w", absPath, statErr)
			}
		}

		// 读取原始内容
		var original []byte
		if statErr == nil {
			original, err = os.ReadFile(absPath)
			if err != nil {
				return appliedFiles, fmt.Errorf("failed to read original file %s: %w", absPath, err)
			}
		}

		// 应用补丁（简化实现：基于行号定位和替换）
		patchedContent, err := applyPatchToContent(original, patch)
		if err != nil {
			return appliedFiles, fmt.Errorf("failed to apply patch to %s: %w", absPath, err)
		}

		// dryRun 模式下只做验证不写盘
		if dryRun {
			appliedFiles = append(appliedFiles, patch.FilePath)
			continue
		}

		// 原子写入：复制到临时文件后 rename（备份原文件）
		backupPath := absPath + ".bak"
		if statErr == nil {
			if err := os.Rename(absPath, backupPath); err != nil {
				return appliedFiles, fmt.Errorf("failed to backup original file: %w", err)
			}
			defer func() {
				if err != nil {
					// 失败时恢复备份
					os.Rename(backupPath, absPath)
				}
			}()
		}

		// 写入新内容
		if err := os.WriteFile(absPath, patchedContent, 0644); err != nil {
			return appliedFiles, fmt.Errorf("failed to write patched file: %w", err)
		}

		// 写入成功，删除备份
		if statErr == nil {
			os.Remove(backupPath)
		}

		appliedFiles = append(appliedFiles, patch.FilePath)
	}

	return appliedFiles, nil
}

// SearchAndReplace 在指定文件中进行精确字符串替换
// expectedOccurrences 期望替换的次数，实际次数不符时返回错误
// TODO(hands_search_and_replace_impl):
//  1. 检查 old 非空，expectedOccurrences >= 0，否则返回参数错误。
//  2. 使用 w.sanitizePath(path) 和 w.isBlockedExtension 校验路径与扩展名。
//  3. 读取文件内容为字符串，统计 strings.Count(content, old) 为 actualOccurrences。
//  4. 若 expectedOccurrences > 0 且 actualOccurrences != expectedOccurrences：
//     - 返回错误，提醒 Agent 先检查上下文，避免误替换。
//  5. 若 expectedOccurrences == 0：
//     - 代表 dry-run，只返回 actualOccurrences，不写入文件。
//  6. 执行 strings.ReplaceAll(content, old, new)，并采用 tmp + rename 的原子写入方式写回文件。
func (w *OSWorkspace) SearchAndReplace(ctx context.Context, path, old, new string, expectedOccurrences int) (actualOccurrences int, err error) {
	// 参数检查
	if old == "" {
		return 0, fmt.Errorf("search string cannot be empty")
	}
	if expectedOccurrences < 0 {
		return 0, fmt.Errorf("expectedOccurrences cannot be negative")
	}

	// 安全检查
	absPath, err := w.sanitizePath(path)
	if err != nil {
		return 0, fmt.Errorf("path security check failed: %w", err)
	}
	if w.isBlockedExtension(absPath) {
		return 0, fmt.Errorf("file extension is blocked")
	}

	// 读取文件内容
	data, err := os.ReadFile(absPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	// 计数并替换
	actualOccurrences = strings.Count(content, old)
	if expectedOccurrences > 0 && actualOccurrences != expectedOccurrences {
		return actualOccurrences, fmt.Errorf("occurrence count mismatch: expected %d, found %d", expectedOccurrences, actualOccurrences)
	}

	// 如果 expectedOccurrences == 0，视为只查询不写入
	if expectedOccurrences == 0 {
		return actualOccurrences, nil
	}

	// 执行替换
	newContent := strings.ReplaceAll(content, old, new)

	// 原子写入
	tmpPath := absPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(newContent), 0644); err != nil {
		return actualOccurrences, fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		os.Remove(tmpPath)
		return actualOccurrences, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return actualOccurrences, nil
}

// --- unified diff 解析辅助结构 ---

type DiffPatch struct {
	FilePath     string `json:"file_path"`     // 目标文件路径
	IsNewFile    bool   `json:"is_new_file"`   // 是否为新增文件
	Hunks        []Hunk `json:"hunks"`         // 补丁块列表
	OriginalMode string `json:"original_mode"` // 原始文件模式（可选）
	NewMode      string `json:"new_mode"`      // 新文件模式（可选）
}

type Hunk struct {
	OldStart int    `json:"old_start"` // 原始文件起始行
	OldCount int    `json:"old_count"` // 原始文件行数
	NewStart int    `json:"new_start"` // 新文件起始行
	NewCount int    `json:"new_count"` // 新文件行数
	Lines    []Line `json:"lines"`     // 行内容
}

type Line struct {
	Type string `json:"type"` // "+" 添加, "-" 删除, " " 上下文
	Text string `json:"text"` // 行内容（不含前导符号）
}

// parseUnifiedDiff 解析 unified diff 格式文本
// 简化实现，支持标准 --- a/file / +++ b/file 格式
// TODO(hands_parse_unified_diff_impl):
//  1. 将 diff 文本按行分割，遍历时识别：
//     - 文件头：以 "--- " 和 "+++ " 开头的行；
//     - hunk 头：以 "@@ " 开头的行；
//     - hunk 内容：以 '+', '-', ' ' 开头的行。
//  2. 每遇到新的文件头，结束上一个 DiffPatch，将收集到的 Hunks 正常化（normalizeHunks）。
//  3. 通过 parseHunkHeader 解析每个 hunk 的 old/new 行号与行数。
//  4. 遍历结束后，补充最后一个 patch 和 hunk，并调用 isNewFilePatch 判断是否为新文件补丁。
func parseUnifiedDiff(diffText string) ([]DiffPatch, error) {
	var patches []DiffPatch
	lines := strings.Split(diffText, "\n")

	var currentPatch *DiffPatch
	var currentHunk *Hunk
	inHunk := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// 检测文件头：--- a/path / +++ b/path
		if strings.HasPrefix(line, "--- ") && !strings.HasPrefix(line, "--- /dev/null") {
			// 结束上一个 patch
			if currentPatch != nil && currentHunk != nil {
				currentPatch.Hunks = append(currentPatch.Hunks, *currentHunk)
				currentPatch.Hunks = normalizeHunks(currentPatch.Hunks)
				patches = append(patches, *currentPatch)
			}

			// 提取文件路径（移除 a/ 或 b/ 前缀）
			origPath := strings.TrimPrefix(line[4:], "a/")
			origPath = strings.TrimPrefix(origPath, "b/")
			currentPatch = &DiffPatch{
				FilePath: origPath,
			}
			currentHunk = nil
			inHunk = false
			continue
		}

		if strings.HasPrefix(line, "+++ ") && currentPatch != nil {
			// 提取文件路径（同上）
			newPath := strings.TrimPrefix(line[4:], "b/")
			newPath = strings.TrimPrefix(newPath, "a/")
			// 通常新旧路径相同
			if newPath != "/dev/null" {
				currentPatch.FilePath = newPath
			}
			continue
		}

		// 检测 hunk 头：@@ -oldStart,oldCount +newStart,newCount @@
		if strings.HasPrefix(line, "@@ ") && currentPatch != nil {
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, fmt.Errorf("invalid hunk header at line %d: %w", i+1, err)
			}
			// 保存上一个 hunk
			if currentHunk != nil {
				currentPatch.Hunks = append(currentPatch.Hunks, *currentHunk)
			}
			currentHunk = &hunk
			inHunk = true
			continue
		}

		// 处理行内容
		if inHunk && currentHunk != nil && (len(line) > 0) {
			lineType := line[0]
			lineContent := line[1:]
			if lineType == '+' || lineType == '-' || lineType == ' ' {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type: string(lineType),
					Text: lineContent,
				})
			}
		}
	}

	// 最后一个 hunk 和 patch
	if currentPatch != nil && currentHunk != nil {
		currentPatch.Hunks = append(currentPatch.Hunks, *currentHunk)
		currentPatch.Hunks = normalizeHunks(currentPatch.Hunks)
		patches = append(patches, *currentPatch)
	}

	// 检测是否为新增文件
	for i := range patches {
		patches[i].IsNewFile = isNewFilePatch(patches[i])
	}

	return patches, nil
}

// parseHunkHeader 解析 @@ -start,count +start,count @@
// TODO(hands_parse_hunk_header_impl):
//  1. 去掉开头和结尾的 "@@"，并使用 strings.TrimSpace 清理空格。
//  2. 按空格或 "+" 分割为旧行信息和新行信息两个部分。
//  3. 对 "start,count" 格式分别解析：
//     - 若包含逗号，拆分为 start 和 count；
//     - 否则将 count 视为 1。
func parseHunkHeader(line string) (Hunk, error) {
	// 移除 @@ 包裹
	content := strings.TrimSpace(line[2 : len(line)-2])
	parts := strings.Split(content, " +")
	if len(parts) != 2 {
		return Hunk{}, fmt.Errorf("invalid format")
	}

	var hunk Hunk
	// 解析旧部分：-start,count
	oldPart := strings.TrimPrefix(parts[0], "-")
	if strings.Contains(oldPart, ",") {
		p := strings.Split(oldPart, ",")
		if len(p) == 2 {
			fmt.Sscanf(p[0], "%d", &hunk.OldStart)
			fmt.Sscanf(p[1], "%d", &hunk.OldCount)
		}
	} else {
		fmt.Sscanf(oldPart, "%d", &hunk.OldStart)
		hunk.OldCount = 1
	}

	// 解析新部分：+start,count
	newPart := parts[1]
	if strings.Contains(newPart, ",") {
		p := strings.Split(newPart, ",")
		if len(p) == 2 {
			fmt.Sscanf(p[0], "%d", &hunk.NewStart)
			fmt.Sscanf(p[1], "%d", &hunk.NewCount)
		}
	} else {
		fmt.Sscanf(newPart, "%d", &hunk.NewStart)
		hunk.NewCount = 1
	}

	return hunk, nil
}

// isNewFilePatch 判断是否为新增文件（新旧内容都源自 /dev/null 或旧文件为空）
// TODO(hands_is_new_file_patch_impl):
//  1. 当前简化策略：如果所有 hunk 中没有删除行（'-'），则视为新文件补丁。
//  2. 如后续需要更精确，可结合 diff 头信息（/dev/null）进行判断。
func isNewFilePatch(patch DiffPatch) bool {
	// 如果所有 hunk 的行都是添加（+），没有删除（-）和上下文（ ），视为新文件
	for _, hunk := range patch.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == "-" {
				return false
			}
		}
	}
	return true
}

// normalizeHunks 将多个 hunk 处理为统一的顺序（确保 OldStart 升序）
// TODO(hands_normalize_hunks_impl):
//  1. 使用 sort.Slice 按 OldStart 升序排序，以保证应用补丁时的顺序正确。
func normalizeHunks(hunks []Hunk) []Hunk {
	// 按 OldStart 排序
	sort.Slice(hunks, func(i, j int) bool {
		return hunks[i].OldStart < hunks[j].OldStart
	})
	return hunks
}

// applyPatchToContent 将 parsed patch 应用到原始内容
// TODO(hands_apply_patch_to_content_impl):
//  1. 将 original 按行分割为切片 originalLines，并保留换行符。
//  2. 遍历每个 Hunk：
//     - 将 [oldLineNum, hunk.OldStart-1] 范围的原始行复制到结果。
//     - 在 Hunk 内部：
//     - '-' 行跳过相应原始行；
//     - '+' 行直接追加新行；
//     - ' ' 行复制对应原始行。
//  3. 所有 Hunk 应用完之后，将剩余原始行追加到结果中，最终 Join 成字符串。
//  4. 注意保持最后一个换行符的语义与原始文件一致。
func applyPatchToContent(original []byte, patch DiffPatch) ([]byte, error) {
	originalLines := strings.Split(string(original), "\n")
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	// 将行索引构建为切片（1-indexed）
	for i := range originalLines {
		originalLines[i] = originalLines[i] + "\n"
	}

	var result []string
	oldLineNum := 1

	for _, hunk := range patch.Hunks {
		// 处理 hunk 开始前的上下文：复制 [oldLineNum, hunk.OldStart-1] 的原始行
		for oldLineNum < hunk.OldStart {
			if oldLineNum-1 < len(originalLines) {
				result = append(result, originalLines[oldLineNum-1])
			}
			oldLineNum++
		}

		// 处理 hunk 内部
		for _, line := range hunk.Lines {
			if line.Type == "-" {
				// 删除行：跳过原始文件的这一行
				if oldLineNum-1 < len(originalLines) && strings.TrimSuffix(originalLines[oldLineNum-1], "\n") == line.Text {
					oldLineNum++
				}
			} else if line.Type == "+" {
				// 添加行：直接追加到结果
				result = append(result, line.Text+"\n")
			} else if line.Type == " " {
				// 上下文行：复制原始文件
				if oldLineNum-1 < len(originalLines) {
					result = append(result, originalLines[oldLineNum-1])
					oldLineNum++
				} else {
					// 如果原始文件行数不足，添加空行
					result = append(result, line.Text+"\n")
				}
			}
		}
	}

	// 追加剩余部分
	for oldLineNum-1 < len(originalLines) {
		result = append(result, originalLines[oldLineNum-1])
		oldLineNum++
	}

	final := strings.Join(result, "")
	// 确保最后没有多余的换行符（与原始文件行为一致）
	if len(original) > 0 && !bytes.HasSuffix(original, []byte{'\n'}) {
		final = strings.TrimSuffix(final, "\n")
	}

	return []byte(final), nil
}
