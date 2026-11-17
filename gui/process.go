package gui

import (
	"errors"
	"os"
)

type processEntry struct {
	pid  int
	ppid int
}

// killChildProcessTree is an exit-only GUI operation. It does not change the
// Runtime's cancellation or the Lua tool's existing process management.
// Only descendants of this GUI process are selected; the GUI, its parent, and
// unrelated processes are never selected for termination.
func killChildProcessTree() error {
	var result error
	for pass := 0; pass < 2; pass++ {
		entries, err := processTable()
		if err != nil {
			return errors.Join(result, err)
		}
		children := make(map[int][]int)
		for _, entry := range entries {
			children[entry.ppid] = append(children[entry.ppid], entry.pid)
		}
		root := os.Getpid()
		seen := map[int]bool{root: true}
		var descendants []int
		var visit func(int)
		visit = func(parent int) {
			for _, pid := range children[parent] {
				if pid <= 1 || seen[pid] {
					continue
				}
				seen[pid] = true
				visit(pid)
				descendants = append(descendants, pid)
			}
		}
		visit(root)
		// Children precede their parents, so a live parent is not killed before
		// we have selected and terminated its known descendants.
		for _, pid := range descendants {
			process, err := os.FindProcess(pid)
			if err != nil {
				continue // the process may already have exited
			}
			if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				result = errors.Join(result, err)
			}
			_ = process.Release()
		}
	}
	return result
}
