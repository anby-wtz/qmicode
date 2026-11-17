//go:build !windows

package gui

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func processTable() ([]processEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-A", "-o", "pid=", "-o", "ppid=").Output()
	if err != nil {
		return nil, fmt.Errorf("GUI 退出时读取进程树: %w", err)
	}
	var entries []processEntry
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		ppid, parentErr := strconv.Atoi(fields[1])
		if pidErr == nil && parentErr == nil {
			entries = append(entries, processEntry{pid: pid, ppid: ppid})
		}
	}
	return entries, nil
}
