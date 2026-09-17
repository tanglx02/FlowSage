package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"cyberstrike-ai/internal/browser"
)

// runBrowser 分发 browser 子命令。
func runBrowser(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("缺少子命令，可用: list / open / status / cookie / close")
	}
	switch args[0] {
	case "list":
		return browserList(args[1:])
	case "open":
		return browserOpen(args[1:])
	case "status":
		return browserStatus(args[1:])
	case "cookie":
		return browserCookie(args[1:])
	case "close":
		return browserClose(args[1:])
	default:
		return fmt.Errorf("未知子命令: %s（可用: list / open / status / cookie / close）", args[0])
	}
}

// newFlagSet 创建统一风格的 FlagSet。
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: flowsage %s [参数]\n\n参数:\n", name)
		fs.PrintDefaults()
	}
	return fs
}

// splitPositional 取出第一个位置参数，其余交回给 flag 解析。
// Go 标准 flag 在遇到首个非 flag 参数后即停止解析，
// 因此需要先摘出位置参数，才能支持 `browser open <url> --wait-login` 这种写法。
func splitPositional(args []string) (string, []string) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return a, rest
		}
	}
	return "", args
}

// ---------------------------------------------------------------- list

func browserList(args []string) error {
	fs := newFlagSet("browser list")
	root := fs.String("root", "", "项目根目录（默认自动向上查找 go.mod）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rootDir := findRootDir(*root)
	list := browser.Detect(rootDir)

	fmt.Printf("项目根目录: %s\n\n", rootDir)
	if len(list) == 0 {
		fmt.Println("未找到可用的 Chromium 内核浏览器。")
		fmt.Println()
		fmt.Println("可选处理：")
		fmt.Println("  1) 安装 Google Chrome 或 Microsoft Edge（Windows 10+ 默认自带 Edge）")
		fmt.Printf("  2) 下载便携版解压到 %s\\%s\\ 目录\n", rootDir, browser.PortableRootName)
		fmt.Println("     分系统下载地址见 docs/dev/07-portable-browser.md")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "优先级\t来源\t路径")
	for i, b := range list {
		mark := ""
		if i == 0 {
			mark = "  ← 默认使用"
		}
		fmt.Fprintf(w, "%d\t%s\t%s%s\n", i+1, b.Kind, b.Path, mark)
	}
	return w.Flush()
}

// ---------------------------------------------------------------- open

func browserOpen(args []string) error {
	url, rest := splitPositional(args)

	fs := newFlagSet("browser open")
	root := fs.String("root", "", "项目根目录")
	workspace := fs.String("workspace", "", "工作区目录（默认 <项目根>/tmp/browser）")
	port := fs.Int("port", 0, "调试端口（默认自动分配空闲端口）")
	headless := fs.Bool("headless", false, "无界面模式")
	exe := fs.String("browser", "", "指定浏览器可执行文件路径")
	waitLogin := fs.Bool("wait-login", false, "等待手动登录完成后自动提取 Cookie")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if url == "" {
		return fmt.Errorf("缺少目标地址，用法: flowsage browser open <url> [--wait-login]")
	}

	rootDir := findRootDir(*root)
	ws := *workspace
	if ws == "" {
		ws = browser.DefaultWorkspace(rootDir)
	}

	inst, err := browser.Launch(browser.LaunchOptions{
		RootDir:   rootDir,
		Workspace: ws,
		URL:       url,
		DebugPort: *port,
		Headless:  *headless,
		ExePath:   *exe,
	})
	if err != nil {
		return err
	}

	fmt.Println("独立浏览器实例已启动")
	fmt.Printf("  浏览器    : %s (%s)\n", inst.ExePath, inst.Kind)
	fmt.Printf("  进程 PID  : %d\n", inst.PID)
	fmt.Printf("  调试端口  : %d\n", inst.DebugPort)
	fmt.Printf("  profile   : %s\n", inst.ProfileDir)
	fmt.Printf("  已打开    : %s\n", inst.TargetURL)
	fmt.Println()
	fmt.Println("该实例使用独立 profile：不影响你日常的浏览器，登录态会保留在 profile 中。")
	fmt.Println("浏览器已脱离本命令行进程，关闭终端也不会退出。")
	fmt.Println()

	if !*waitLogin {
		fmt.Println("登录完成后，执行以下命令提取 Cookie：")
		fmt.Printf("  flowsage browser cookie --url %s --format header\n", inst.TargetURL)
		return nil
	}

	fmt.Println("请在浏览器窗口中完成登录（含验证码），完成后回到这里按回车继续...")
	if _, err := readLine(); err != nil {
		return err
	}
	return extractAndReport(inst, inst.TargetURL, "text", "")
}

// ---------------------------------------------------------------- status

func browserStatus(args []string) error {
	fs := newFlagSet("browser status")
	root := fs.String("root", "", "项目根目录")
	workspace := fs.String("workspace", "", "工作区目录")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ws := *workspace
	if ws == "" {
		ws = browser.DefaultWorkspace(findRootDir(*root))
	}

	inst, err := browser.LoadState(ws)
	if err != nil {
		return err
	}

	fmt.Printf("浏览器    : %s (%s)\n", inst.ExePath, inst.Kind)
	fmt.Printf("进程 PID  : %d\n", inst.PID)
	fmt.Printf("调试端口  : %d\n", inst.DebugPort)
	fmt.Printf("profile   : %s\n", inst.ProfileDir)
	fmt.Printf("目标地址  : %s\n", inst.TargetURL)
	fmt.Printf("启动时间  : %s\n", inst.StartedAt.Format(time.DateTime))

	if inst.Alive() {
		fmt.Println("状态      : 运行中")
	} else {
		fmt.Println("状态      : 已关闭或不可达")
		fmt.Println()
		fmt.Println("提示：用 flowsage browser open <url> 重新启动；profile 中的登录态会保留。")
	}
	return nil
}

// ---------------------------------------------------------------- cookie

func browserCookie(args []string) error {
	fs := newFlagSet("browser cookie")
	root := fs.String("root", "", "项目根目录")
	workspace := fs.String("workspace", "", "工作区目录")
	url := fs.String("url", "", "只提取对该地址生效的 Cookie（默认用实例启动时的地址）")
	format := fs.String("format", "text", "输出格式: text | header | json")
	exportAlertctl := fs.String("export-alertctl", "", "同时写入 alertctl 的 config.toml 路径")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ws := *workspace
	if ws == "" {
		ws = browser.DefaultWorkspace(findRootDir(*root))
	}

	inst, err := browser.LoadState(ws)
	if err != nil {
		return err
	}
	target := *url
	if target == "" {
		target = inst.TargetURL
	}
	return extractAndReport(inst, target, *format, *exportAlertctl)
}

// extractAndReport 提取 Cookie 并按指定格式输出。
func extractAndReport(inst *browser.Instance, targetURL, format, exportAlertctl string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cookies, err := browser.ExtractCookies(ctx, inst, targetURL)
	if err != nil {
		return err
	}
	if len(cookies) == 0 {
		fmt.Println("未提取到任何 Cookie。")
		fmt.Println("常见原因：尚未在该实例中登录目标站点，或目标地址填错。")
		return nil
	}

	switch format {
	case "header":
		fmt.Println(browser.CookieHeader(cookies))
	case "json":
		data, err := json.MarshalIndent(cookies, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "text":
		fallthrough
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "域名\t名称\t过期")
		for _, c := range cookies {
			exp := "会话级"
			if c.Expires > 0 {
				exp = time.Unix(int64(c.Expires), 0).Format(time.DateTime)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", c.Domain, c.Name, exp)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf("共 %d 条。粘贴用的单行 Cookie：\n\n", len(cookies))
		fmt.Println(browser.CookieHeader(cookies))
	}

	if earliest := browser.EarliestExpiry(cookies); !earliest.IsZero() {
		fmt.Println()
		fmt.Printf("注意：最早过期时间 %s（%s 后失效）\n",
			earliest.Format(time.DateTime), time.Until(earliest).Round(time.Minute))
	}

	if exportAlertctl != "" {
		fmt.Println()
		return writeAlertctlCookie(exportAlertctl, browser.CookieHeader(cookies))
	}
	return nil
}

// ---------------------------------------------------------------- close

func browserClose(args []string) error {
	fs := newFlagSet("browser close")
	root := fs.String("root", "", "项目根目录")
	workspace := fs.String("workspace", "", "工作区目录")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ws := *workspace
	if ws == "" {
		ws = browser.DefaultWorkspace(findRootDir(*root))
	}

	inst, err := browser.LoadState(ws)
	if err != nil {
		return err
	}

	if err := inst.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "关闭进程失败（可能已退出）: %v\n", err)
	} else {
		fmt.Printf("已关闭浏览器实例（PID %d）\n", inst.PID)
	}
	if err := browser.RemoveState(ws); err != nil {
		return err
	}
	fmt.Println("profile 已保留，下次启动仍是同一会话。")
	return nil
}

// readLine 从标准输入读取一行。
func readLine() (string, error) {
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return sb.String(), nil
			}
			if buf[0] != '\r' {
				sb.WriteByte(buf[0])
			}
		}
		if err != nil {
			if sb.Len() > 0 {
				return sb.String(), nil
			}
			return "", err
		}
	}
}