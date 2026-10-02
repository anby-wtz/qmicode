package luatool

import (
	"context"
	"fmt"

	tool "qmicode-go/codelib/tool"

	"github.com/openai/openai-go"
	luar "github.com/oscarhealth/gopher-luar"
	lua "github.com/yuin/gopher-lua"

	converter "github.com/kerimovok/go-lua-converter"
)

const (
	LuaToolsVarName  = "tools"
	LuaNameVarName   = "name"
	LuaDescVarName   = "description"
	LuaParamsVarName = "parameters"

	LuaPreMsgFuncName = "pre_msg"
	LuaExecFuncName   = "exec"
)

// LuaToolGroup 表示一个 Lua 脚本文件加载出的工具组。
// 同一文件中的多个工具共享同一个 LState，生命周期由 LuaToolGroup 统一管理：
// Destroy 时只关闭一次 State。
type LuaToolGroup struct {
	Path  string // 来源 .lua 文件路径（用于错误信息）
	State *lua.LState

	Tools []LuaLLMTool
}

// LuaLLMTool 表示 tools 列表中的一个工具定义。
// pre_msg / exec 不单独持有函数引用，而是保留原始条目表 entry，调用时再从中查找；
// state 仅作为访问指针（gopher-lua 的 LValue 无法反查所属 State），生命周期归 ToolGroup 管。
type LuaLLMTool struct {
	Name        string
	Description string
	Parameters  map[string]any

	state *lua.LState // 所属组的共享状态，仅供调用使用
	entry *lua.LTable // tools 列表中的原始条目表（含 pre_msg / exec）
}

// 检查是否正确实现接口
var _ tool.Tool = (*LuaLLMTool)(nil)

func (t *LuaLLMTool) GetName() string {
	return t.Name
}

func (t *LuaLLMTool) GetDescription() string {
	return t.Description
}

func (t *LuaLLMTool) GetParameters() map[string]any {
	return t.Parameters
}

func (t *LuaLLMTool) getFunc(name string) (*lua.LFunction, error) {
	fn := t.entry.RawGetString(name)
	lfn, ok := fn.(*lua.LFunction)
	if !ok {
		return nil, fmt.Errorf("tool %s: %s is not a function", t.Name, name)
	}
	return lfn, nil
}

func (t *LuaLLMTool) CallPreMsgFunc(args map[string]any) (string, error) {
	fn, err := t.getFunc(LuaPreMsgFuncName)
	if err != nil {
		return "", err
	}
	L := t.state
	err = L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1, // 1 个返回值
		Protect: true,
	}, luar.New(L, args))
	if err != nil {
		return "", err
	}
	ret := L.Get(-1).String()
	L.Pop(1)
	return ret, nil
}

// 返回值：返回值1：工具执行结果（for LLM），返回值2：工具执行结果（for 人类），返回值3：错误对象
func (t *LuaLLMTool) CallExecFunc(ctx context.Context, args map[string]any) (string, string, error) {
	fn, err := t.getFunc(LuaExecFuncName)
	if err != nil {
		return "", "", err
	}
	L := t.state

	// 注入当前调用的 ctx，供 qmi_is_interrupted() / exec_with_timeout 感知中断。
	// 同一 LState 一次只会有一个工具调用在执行（gopher-lua 单线程同步），故 per-State 覆盖是安全的。
	setCurrentCtx(L, ctx)
	defer clearCurrentCtx(L)

	err = L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    3, // 3 个返回值
		Protect: true,
	}, luar.New(L, args, luar.ReflectOptions{Immutable: true}))
	if err != nil {
		return "", "", err
	}
	// 对于Lua脚本，如果返回值3不是nil，则Go创建一个错误
	ret1 := L.Get(-3).String()
	ret2 := L.Get(-2).String()
	ret3 := L.Get(-1)
	L.Pop(3)
	if ret3.Type() == lua.LTNil {
		return ret1, ret2, nil
	}
	return ret1, ret2, fmt.Errorf("%s", ret3.String())
}

// LoadToolsFromScript 加载一个 Lua 脚本文件，返回其中的工具组。
// 脚本必须定义全局 tools 表（数组），每个元素是一个工具定义表：
//
//	tools = {
//	    { name=..., description=..., parameters=..., pre_msg=function(args)..., exec=function(args)... },
//	    ...
//	}
func LoadToolsFromScript(path string) (*LuaToolGroup, error) {
	L := lua.NewState()

	fail := func(err error) (*LuaToolGroup, error) {
		L.Close()
		return nil, err
	}

	// 先注入脚本路径
	L.SetGlobal("SCRIPT_PATH", lua.LString(path))

	if err := LoadAPIFunctions(L); err != nil {
		return fail(err)
	}

	if err := L.DoFile(path); err != nil {
		return fail(fmt.Errorf("execute %s: %w", path, err))
	}

	toolsVal := L.GetGlobal(LuaToolsVarName)
	toolsTable, ok := toolsVal.(*lua.LTable)
	if !ok {
		return fail(fmt.Errorf("%s: global 'tools' must be a table (array of tool definitions)", path))
	}

	n := toolsTable.MaxN()
	if n == 0 {
		return fail(fmt.Errorf("%s: 'tools' is empty", path))
	}

	group := &LuaToolGroup{Path: path, State: L}
	names := make(map[string]bool)

	for i := 1; i <= n; i++ {
		entryVal := toolsTable.RawGetInt(i)
		entry, ok := entryVal.(*lua.LTable)
		if !ok {
			return fail(fmt.Errorf("%s: tools[%d] must be a table", path, i))
		}

		nameVal := entry.RawGetString(LuaNameVarName)
		name, ok := nameVal.(lua.LString)
		if !ok || string(name) == "" {
			return fail(fmt.Errorf("%s: tools[%d].name must be a non-empty string", path, i))
		}

		descVal := entry.RawGetString(LuaDescVarName)
		desc, ok := descVal.(lua.LString)
		if !ok || string(desc) == "" {
			return fail(fmt.Errorf("%s: tools[%d].description must be a non-empty string", path, i))
		}

		paramsVal := entry.RawGetString(LuaParamsVarName)
		paramsTable, ok := paramsVal.(*lua.LTable)
		if !ok {
			return fail(fmt.Errorf("%s: tools[%d].parameters must be a table", path, i))
		}
		paraGo := converter.TableToGo(L, paramsTable)
		para, ok := paraGo.(map[string]any)
		if !ok {
			return fail(fmt.Errorf("%s: tools[%d].parameters is not a map", path, i))
		}

		for _, fnName := range []string{LuaPreMsgFuncName, LuaExecFuncName} {
			fnVal := entry.RawGetString(fnName)
			if _, ok := fnVal.(*lua.LFunction); !ok {
				return fail(fmt.Errorf("%s: tools[%d].%s must be a function", path, i, fnName))
			}
		}

		if names[string(name)] {
			return fail(fmt.Errorf("%s: duplicate tool name %q in the same file", path, string(name)))
		}
		names[string(name)] = true

		group.Tools = append(group.Tools, LuaLLMTool{
			Name:        string(name),
			Description: string(desc),
			Parameters:  para,
			state:       L,
			entry:       entry,
		})
	}

	return group, nil
}

func (t *LuaLLMTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  t.Parameters,
		},
	}
}

// Destroy 关闭本组共享的 LState。同一文件的所有工具随之失效，只能调用一次。
func (g *LuaToolGroup) Destroy() {
	if g.State != nil {
		g.State.Close()
		g.State = nil
	}
}
