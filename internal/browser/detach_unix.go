//go:build !windows

package browser

import (
	"os"
	"syscall"
)

// detachSysProcAttrCandidates 返回按优先级排列的启动属性。
// Unix 上用 Setsid 让浏览器自成会话，父进程退出或终端关闭都不影响它，
// 因此只有一种方式，不存在降级分支。
func detachSysProcAttrCandidates() []*syscall.SysProcAttr {
	return []*syscall.SysProcAttr{{Setsid: true}}
}

// terminateProcessTree 终止浏览器进程。
// 以 Setsid 启动后浏览器自成进程组，可直接向该进程发送信号。
func terminateProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}