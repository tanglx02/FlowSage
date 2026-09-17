package browser

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// Instance 描述一个已启动的独立浏览器实例。
type Instance struct {
	Kind       Kind      `json:"kind"`
	ExePath    string    `json:"exe_path"`
	PID        int       `json:"pid"`
	DebugPort  int       `json:"debug_port"`
	ProfileDir string    `json:"profile_dir"`
	TargetURL  string    `json:"target_url"`
	StartedAt  time.Time `json:"started_at"`
}

// LaunchOptions 控制实例的启动方式。
type LaunchOptions struct {
	RootDir   string   // 项目根目录，用于便携版探测
	Workspace string   // 工作区目录，存放 profile 与状态文件
	URL       string   // 启动后打开的地址
	DebugPort int      // 0 表示自动挑选空闲端口
	Headless  bool     // 无界面模式（仅用于自动化验证）
	ExtraArgs []string // 追加给浏览器的额外参数
	Browser   *Browser // 指定浏览器；nil 表示按优先级自动探测
	ExePath   string   // 直接指定浏览器可执行文件路径（优先级高于自动探测）
}

// DefaultWorkspace 返回默认工作区：<rootDir>/tmp/browser。
// 选择 tmp/ 是因为上游 .gitignore 已忽略该目录，profile 与状态文件不会误入版本库。
func DefaultWorkspace(rootDir string) string {
	return filepath.Join(rootDir, "tmp", "browser")
}

// StatePath 返回实例状态文件路径。
func StatePath(workspace string) string {
	return filepath.Join(workspace, "state.json")
}

// Launch 启动一个独立实例并立即返回。
//
// 浏览器以脱离父进程的方式启动：CLI 退出后浏览器继续存活，
// 后续用 LoadState + Attach 重新接管，用户不必重开窗口、重登一次。
func Launch(opts LaunchOptions) (*Instance, error) {
	b, err := resolveBrowser(opts)
	if err != nil {
		return nil, err
	}

	port := opts.DebugPort
	if port == 0 {
		port, err = freePort()
		if err != nil {
			return nil, fmt.Errorf("分配调试端口失败: %w", err)
		}
	}

	profileDir := filepath.Join(opts.Workspace, "profile")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建 profile 目录失败: %w", err)
	}

	args := []string{
		// 独立 profile：既不影响用户日常浏览器，也让登录态可跨重启保留。
		"--user-data-dir=" + profileDir,
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-popup-blocking",
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}
	args = append(args, opts.ExtraArgs...)
	if opts.URL != "" {
		args = append(args, opts.URL)
	}

	pid, err := startDetached(b.Path, args)
	if err != nil {
		return nil, err
	}

	inst := &Instance{
		Kind:       b.Kind,
		ExePath:    b.Path,
		PID:        pid,
		DebugPort:  port,
		ProfileDir: profileDir,
		TargetURL:  opts.URL,
		StartedAt:  time.Now(),
	}

	if err := inst.waitReady(25 * time.Second); err != nil {
		_ = inst.Close()
		return nil, err
	}
	if err := SaveState(opts.Workspace, inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// startDetached 以脱离父进程的方式启动浏览器，并立即释放句柄。
//
// 按平台提供的候选属性依次尝试：Windows 上先试"能从 Job 脱离"的强隔离方式，
// 该 Job 不允许脱离时创建会失败，再退回普通脱离方式。
func startDetached(exePath string, args []string) (int, error) {
	var lastErr error
	for _, attr := range detachSysProcAttrCandidates() {
		cmd := exec.Command(exePath, args...)
		cmd.SysProcAttr = attr
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		pid := cmd.Process.Pid
		_ = cmd.Process.Release()
		return pid, nil
	}
	return 0, fmt.Errorf("启动浏览器失败: %w", lastErr)
}

// waitReady 轮询调试端口，直到浏览器可被接管或超时。
func (i *Instance) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := i.DebuggerURL(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("等待浏览器调试端口就绪超时（%s）：%v\n"+
		"若浏览器已弹出但无法接管，通常是企业策略禁用了远程调试，可改用其他浏览器来源", timeout, lastErr)
}

// DebuggerURL 返回浏览器级 WebSocket 调试地址。
func (i *Instance) DebuggerURL() (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", i.DebugPort)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("调试端口返回 %s", resp.Status)
	}
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	if v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("调试端口未返回 webSocketDebuggerUrl")
	}
	return v.WebSocketDebuggerURL, nil
}

// Alive 探测实例是否仍在运行。
func (i *Instance) Alive() bool {
	_, err := i.DebuggerURL()
	return err == nil
}

// Close 终止实例及其子进程。
func (i *Instance) Close() error {
	return terminateProcessTree(i.PID)
}

// SaveState 持久化实例状态，供后续命令重新接管。
func SaveState(workspace string, inst *Instance) error {
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StatePath(workspace), data, 0o600)
}

// LoadState 读取上次启动的实例状态。文件不存在时返回明确错误。
func LoadState(workspace string) (*Instance, error) {
	data, err := os.ReadFile(StatePath(workspace))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("没有已启动的浏览器实例（未找到 %s）", StatePath(workspace))
		}
		return nil, err
	}
	var inst Instance
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, fmt.Errorf("状态文件损坏: %w", err)
	}
	return &inst, nil
}

// RemoveState 清除状态文件。
func RemoveState(workspace string) error {
	err := os.Remove(StatePath(workspace))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// resolveBrowser 按优先级或用户指定选择浏览器。
func resolveBrowser(opts LaunchOptions) (Browser, error) {
	if opts.Browser != nil {
		return *opts.Browser, nil
	}
	if opts.ExePath != "" {
		return Browser{Kind: KindOther, Path: opts.ExePath}, nil
	}
	return Pick(opts.RootDir)
}

// freePort 向系统申请一个空闲端口。
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}