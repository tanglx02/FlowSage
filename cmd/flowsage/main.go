// Command flowsage 是 FlowSage 的命令行入口。
//
// 与 cmd/server（上游 Web 控制台）并存：本命令提供浏览器托管、会话接管等
// 面向作业的 CLI 能力，后续阶段会加入流量观测与页面操控。
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const usage = `FlowSage — 由浏览器驱动的作业 Agent

用法:
  flowsage <命令> [参数]

浏览器命令:
  browser list                 列出本机探测到的可用浏览器
  browser open <url>           启动独立浏览器实例并打开目标地址
  browser status               查看当前实例状态
  browser cookie               从运行中的实例提取 Cookie
  browser close                关闭当前实例

通用参数:
  --root <目录>                指定项目根目录（默认自动向上查找 go.mod）
  -h, --help                   显示帮助

示例:
  flowsage browser open https://example.com --wait-login
  flowsage browser cookie --url https://example.com --format header
  flowsage browser cookie --export-alertctl D:\project\python\态势感知告警处理\二区\config.toml

更多说明见 docs/dev/06-design.md
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(2)
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	case "browser":
		if err := runBrowser(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", args[0])
		fmt.Print(usage)
		os.Exit(2)
	}
}

// findRootDir 定位项目根目录，用于查找便携版浏览器。
//
// 顺序：显式指定的 override → 从当前目录向上查找 go.mod → 可执行文件所在目录。
// 最后一项保证在便携包中（无 go.mod）也能正确定位同级 browser/ 目录。
func findRootDir(override string) string {
	if override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			return abs
		}
		return override
	}

	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}