//go:build !windows

package luatool

import (
	"os/exec"
	"syscall"
)

// 让子进程成为新进程组组长，便于超时后杀掉整棵进程树
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// 杀掉以 pid 为组长的整个进程组（含 bash 派生的所有孙辈）
func killTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
