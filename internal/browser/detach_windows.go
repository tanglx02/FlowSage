//go:build windows

package browser

import (
	"os/exec"
	"strconv"
	"syscall"
)

// Windows 进程创建标志（syscall 未导出的常量）。
const (
	detachedProcess          = 0x00000008 // 不继承父进程控制台
	createBreakawayFromJob   = 0x01000000 // 允许脱离父进程所在的 Job Object
)

// detachSysProcAttrCandidates 返回按优先级排列的启动属性：
//
//  1. 强隔离：脱离当前 Job Object + 独立进程组 + 无控制台。
//     父进程若运行在"关闭即杀子进程"的 Job 中（终端、IDE、CI 常见），
//     只有带上 CREATE_BREAKAWAY_FROM_JOB 才能让浏览器活下来。
//  2. 降级：Job 不允许 breakaway 时，带该标志创建会失败，退回普通脱离方式。
//
// 调用方按顺序尝试，直到 Start 成功。
func detachSysProcAttrCandidates() []*syscall.SysProcAttr {
	base := uint32(syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess)
	return []*syscall.SysProcAttr{
		{CreationFlags: base | createBreakawayFromJob},
		{CreationFlags: base},
	}
}

// terminateProcessTree 连同子进程一起终止。
// 浏览器会派生渲染进程，只杀父进程会留下残留子进程，因此用 taskkill /T。
func terminateProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}