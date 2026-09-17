// Package browser 提供浏览器探测、独立实例管理与 CDP 会话能力。
//
// 设计要点（见 docs/dev/06-design.md §5.1）：
//   - 浏览器按 Chrome → Edge → 项目内便携版 的顺序探测，兼容 Windows / Linux / macOS
//   - 实例使用独立的 user-data-dir，不影响用户日常浏览器，且登录态可跨重启保留
//   - 实例以脱离父进程的方式启动，CLI 退出后浏览器继续存活，后续命令可重新附着
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Kind 标识浏览器来源，用于日志与降级提示。
type Kind string

const (
	KindChrome   Kind = "chrome"   // 系统安装的 Google Chrome
	KindEdge     Kind = "edge"     // 系统安装的 Microsoft Edge（Windows 10+ 默认存在）
	KindPortable Kind = "portable" // 项目 browser/ 目录下的便携版
	KindOther    Kind = "other"    // 其他 Chromium 内核浏览器（如 Linux 的 chromium）
)

// Browser 是一个可被 CDP 驱动的浏览器可执行文件。
type Browser struct {
	Kind Kind   `json:"kind"`
	Path string `json:"path"`
}

// String 便于日志与命令行输出。
func (b Browser) String() string {
	return fmt.Sprintf("%s (%s)", b.Path, b.Kind)
}

// PortableRootName 是便携版浏览器在项目根目录下的约定目录名。
const PortableRootName = "browser"

// Detect 返回按优先级排序的可用浏览器列表。
//
// rootDir 为项目根目录，用于查找便携版；传空则跳过便携版探测。
// 找不到任何浏览器时返回空列表与 nil error —— 由调用方决定如何提示。
func Detect(rootDir string) []Browser {
	var found []Browser
	add := func(k Kind, path string) {
		if path == "" {
			return
		}
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			return
		}
		for _, f := range found {
			if samePath(f.Path, path) {
				return
			}
		}
		found = append(found, Browser{Kind: k, Path: path})
	}

	switch runtime.GOOS {
	case "windows":
		for _, p := range []string{
			filepath.Join(os.Getenv("ProgramFiles"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
		} {
			add(KindChrome, p)
		}
		for _, p := range []string{
			filepath.Join(os.Getenv("ProgramFiles(x86)"), `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(os.Getenv("ProgramFiles"), `Microsoft\Edge\Application\msedge.exe`),
		} {
			add(KindEdge, p)
		}
	case "darwin":
		for _, p := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		} {
			add(KindChrome, p)
		}
		add(KindEdge, "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge")
	default: // linux 及其他类 Unix
		for _, name := range []string{"google-chrome", "google-chrome-stable"} {
			if p, err := exec.LookPath(name); err == nil {
				add(KindChrome, p)
			}
		}
		for _, name := range []string{"microsoft-edge", "microsoft-edge-stable"} {
			if p, err := exec.LookPath(name); err == nil {
				add(KindEdge, p)
			}
		}
		for _, name := range []string{"chromium", "chromium-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				add(KindOther, p)
			}
		}
	}

	for _, p := range detectPortable(rootDir) {
		add(KindPortable, p)
	}
	return found
}

// detectPortable 在 <rootDir>/browser/ 下查找便携版可执行文件。
// 兼容三种解压布局：chrome-win/、chrome-win64/、chrome-linux64/、chrome-mac/。
func detectPortable(rootDir string) []string {
	if rootDir == "" {
		return nil
	}
	base := filepath.Join(rootDir, PortableRootName)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}

	var names []string
	switch runtime.GOOS {
	case "windows":
		names = []string{"chrome.exe"}
	case "darwin":
		names = []string{
			filepath.Join("Chromium.app", "Contents", "MacOS", "Chromium"),
			filepath.Join("Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
		}
	default:
		names = []string{"chrome"}
	}

	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, n := range names {
			p := filepath.Join(base, e.Name(), n)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// samePath 比较两个路径是否指向同一文件（Windows 下大小写不敏感）。
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// Pick 按优先级返回首个可用浏览器；无可用时返回错误并给出指引。
func Pick(rootDir string) (Browser, error) {
	list := Detect(rootDir)
	if len(list) == 0 {
		return Browser{}, fmt.Errorf(
			"未找到可用的 Chromium 内核浏览器。\n"+
				"可选处理：\n"+
				"  1) 安装 Google Chrome 或 Microsoft Edge（Windows 10+ 默认自带 Edge）\n"+
				"  2) 下载便携版解压到项目 %s/ 目录，下载地址见 docs/dev/07-portable-browser.md",
			PortableRootName)
	}
	return list[0], nil
}