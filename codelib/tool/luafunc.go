package tool

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	converter "github.com/kerimovok/go-lua-converter"
	lua "github.com/yuin/gopher-lua"
)

// 每个 LState 当前工具调用所携带的 context（用于中断感知）。
// gopher-lua 对单个 State 是同步单线程调用的，因此这里以 *lua.LState 为键隔离各次调用，
// qmi_is_interrupted / exec_with_timeout 等注入函数通过 getCurrentCtx(L) 读取。
var (
	ctxMu       sync.RWMutex
	currentCtxs = map[*lua.LState]context.Context{}
)

func setCurrentCtx(L *lua.LState, ctx context.Context) {
	if ctx == nil {
		return
	}
	ctxMu.Lock()
	currentCtxs[L] = ctx
	ctxMu.Unlock()
}

func clearCurrentCtx(L *lua.LState) {
	ctxMu.Lock()
	delete(currentCtxs, L)
	ctxMu.Unlock()
}

// getCurrentCtx 返回当前调用携带的 context；若无则返回 context.Background()。
func getCurrentCtx(L *lua.LState) context.Context {
	ctxMu.RLock()
	if ctx, ok := currentCtxs[L]; ok && ctx != nil {
		ctxMu.RUnlock()
		return ctx
	}
	ctxMu.RUnlock()
	return context.Background()
}

// 返回值：标准输出、错误输出、是否超时、错误信息
// timeout: 单位秒，超时时间
// name: 进程名或路径
// args: 参数列表
func execWithTimeout(parent context.Context, timeout int, name string, args ...string) (string, string, bool, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 让子进程成为新进程组组长，超时后可杀整棵进程树（见 process_unix.go / process_windows.go）
	setupProcessGroup(cmd)
	// WaitDelay: 即使孙辈仍持有管道写端导致 EOF 迟迟不到，也强制 Run() 及时返回。
	// 用固定值(而非叠加 timeout)，因为 killTree 已同步杀进程树，这里只是最后泄洪口
	cmd.WaitDelay = 5 * time.Second

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		// 超时时补杀整棵进程树（context 只杀直接子进程，孙辈可能仍持有管道 FD）
		if cmd.Process != nil {
			killTree(cmd.Process.Pid)
		}
		return stdout.String(), stderr.String(), true, nil
	}
	return stdout.String(), stderr.String(), false, err
}

var errStopWalk = errors.New("stop walk")

// listFiles 递归列出 root 下的所有普通文件，返回相对路径（使用 '/' 分隔）。
// 跳过隐藏条目（文件名以 '.' 开头），limit<=0 时默认上限为 1000。
func listFiles(root string, limit int) ([]string, error) {
	if fi, err := os.Stat(root); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, os.ErrInvalid
	}
	if limit <= 0 {
		limit = 1000
	}

	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // 进入子目录失败则跳过该分支
		}
		name := d.Name()
		// 跳过隐藏文件/目录（路径中任一层以 '.' 开头）
		if strings.HasPrefix(name, ".") && path != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		if len(out) >= limit {
			return errStopWalk // 达到上限，提前结束
		}
		return nil
	})
	if err != nil && err != errStopWalk {
		return out, err
	}
	return out, nil
}

func LoadAPIFunctions(L *lua.LState) error {
	// 执行一个进程并返回结果（带超时）：exec_with_timeout(timeout, command, args) -> stdout, stderr, timeouted, error
	L.SetGlobal("exec_with_timeout", L.NewFunction(func(L *lua.LState) int {
		timeout := L.CheckNumber(1)
		command := L.CheckString(2)
		argsTable := L.CheckTable(3)
		args, ok := converter.TableToGo(L, argsTable).([]any)
		if !ok {
			L.Push(lua.LString(""))
			L.Push(lua.LString(""))
			L.Push(lua.LBool(false))
			L.Push(lua.LString("参数列表必须是数组"))
			return 4
		}
		var argsStr []string
		for _, arg := range args {
			if str, ok := arg.(string); ok {
				argsStr = append(argsStr, str)
				continue
			}
		}
		ctx := getCurrentCtx(L)
		stdout, stderr, timeouted, err := execWithTimeout(ctx, int(timeout), command, argsStr...)
		L.Push(lua.LString(stdout))
		L.Push(lua.LString(stderr))
		L.Push(lua.LBool(timeouted))
		if err != nil {
			L.Push(lua.LString(err.Error()))
			return 4
		}
		L.Push(lua.LNil)
		return 4
	}))

	// 中断感知：qmi_is_interrupted() -> bool
	// Lua 脚本在长循环中可主动调用以协作式响应用户中断（gopher-lua 无法从外部强制中止 VM）。
	L.SetGlobal("qmi_is_interrupted", L.NewFunction(func(L *lua.LState) int {
		if getCurrentCtx(L).Err() != nil {
			L.Push(lua.LBool(true))
		} else {
			L.Push(lua.LBool(false))
		}
		return 1
	}))

	// 递归列出目录下的文件（相对路径，'/' 分隔）：qmi_list_files(root, limit) -> files(array), error
	// - root: 要扫描的目录；不存在或不是目录时返回错误
	// - limit: 可选，最多返回多少个文件，<=0 时用默认上限(1000)
	// - 跳过隐藏文件/目录（任一层以 '.' 开头），只返回普通文件
	L.SetGlobal("qmi_list_files", L.NewFunction(func(L *lua.LState) int {
		root := L.CheckString(1)
		var limit int
		if L.Get(2) != lua.LNil {
			limit = int(L.CheckNumber(2))
		}
		files, err := listFiles(root, limit)
		if err != nil {
			L.Push(L.NewTable()) // 空表
			L.Push(lua.LString(err.Error()))
			return 2
		}
		resT := L.NewTable()
		for i, f := range files {
			resT.RawSetInt(i+1, lua.LString(f))
		}
		L.Push(resT)
		L.Push(lua.LNil)
		return 2
	}))

	// 获取系统信息
	L.SetGlobal("get_os_info", L.NewFunction(func(L *lua.LState) int {
		// 返回一个 table，包含 os, arch, 等
		info := L.NewTable()
		L.SetField(info, "os", lua.LString(runtime.GOOS))
		L.SetField(info, "arch", lua.LString(runtime.GOARCH))
		// 也可加入更多信息，如是否支持 bash（检查 PATH）
		L.Push(info)
		return 1
	}))
	return nil
}
