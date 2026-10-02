//go:build windows

package luatool

import (
	"os/exec"
	"strconv"
)

// Windows 没有 POSIX 进程组，无需（也无法）在此设置；清理依赖 killTree + WaitDelay
func setupProcessGroup(cmd *exec.Cmd) {}

// 用 taskkill /T 杀掉子进程及其派生的孙辈（尽力而为，兜底靠 WaitDelay）
func killTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}
