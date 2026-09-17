package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// cookieLineRe 匹配 alertctl config.toml 中的 cookie 赋值行。
var cookieLineRe = regexp.MustCompile(`^\s*cookie\s*=`)

// writeAlertctlCookie 把单行 Cookie 写入 alertctl 的 config.toml（首个落地场景的适配器）。
//
// 这是替代"人工从浏览器复制 Cookie 再粘进配置文件"的最后一公里，
// 因此做最小侵入的原地修改：
//  1. 先备份为 <path>.bak，失败可回滚
//  2. 已有 cookie 行 → 整行替换
//  3. 没有 → 插入到 [site] 段头之后
//
// 保留原文件的 BOM：现场常用记事本另存，UTF-8 会带 BOM，
// alertctl 读取时会自行剥离，这里不应擅自去掉以免文件形态变化。
func writeAlertctlCookie(path, cookie string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}

	const bom = "\ufeff"
	hasBOM := strings.HasPrefix(string(raw), bom)
	content := strings.TrimPrefix(string(raw), bom)

	// TOML 基本字符串：%q 会处理反斜杠与双引号，Cookie 值通常不含这些字符。
	newLine := fmt.Sprintf("cookie = %q", cookie)

	lines := strings.Split(content, "\n")
	replacedAt := -1
	siteInsertAt := -1

	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if trimmed == "[site]" {
				siteInsertAt = i + 1
			}
			continue
		}
		if cookieLineRe.MatchString(ln) {
			replacedAt = i
			break
		}
	}

	switch {
	case replacedAt >= 0:
		lines[replacedAt] = newLine
	case siteInsertAt >= 0:
		lines = append(lines[:siteInsertAt],
			append([]string{newLine}, lines[siteInsertAt:]...)...)
		replacedAt = siteInsertAt
	default:
		return fmt.Errorf("在 %s 中既没有 cookie 行、也没有 [site] 段，请确认这是 alertctl 的配置文件", path)
	}

	if err := os.WriteFile(path+".bak", raw, 0o600); err != nil {
		return fmt.Errorf("写入备份失败，已中止（原文件未改动）: %w", err)
	}

	out := strings.Join(lines, "\n")
	if hasBOM {
		out = bom + out
	}
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return fmt.Errorf("写入 %s 失败，可用 %s.bak 恢复: %w", path, path, err)
	}

	preview := cookie
	if len(preview) > 48 {
		preview = preview[:48] + "..."
	}
	fmt.Printf("已写入 alertctl 配置\n")
	fmt.Printf("  文件      : %s\n", path)
	fmt.Printf("  备份      : %s.bak\n", path)
	fmt.Printf("  所在行    : 第 %d 行\n", replacedAt+1)
	fmt.Printf("  Cookie    : %s\n", preview)
	return nil
}