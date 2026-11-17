package gui

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func processTable() ([]processEntry, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("GUI 退出时读取进程树: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var entries []processEntry
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		entries = append(entries, processEntry{pid: int(entry.ProcessID), ppid: int(entry.ParentProcessID)})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("GUI 退出时遍历进程树: %w", err)
	}
	return entries, nil
}
