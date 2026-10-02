/*
Copyright: 2026 QmiTek. All rights reserved.
Date: 2026-06-06
Author: anby.wtz
*/

/*
Each agent has its template and configuration, which stored and defined in a specific directory.
The directory structure follows the pattern:
/agents/{agent_name}
├── sysprompt  // the system prompt for the agent. It includes several markdown files with number to sort (eg. 01-Identity.md, 02-Role.md, etc.)
├── tools      // tool definitions and configurations. It includes several .lua script files.
├── cptprompt  // system prompt when compact. It includes several markdown files like sysprompt.
*/
/*
The tools directory lua script files that can be dynamically loaded to extend the agent's capabilities.
Each .lua file defines a global 'tools' table (an array), and each element is one tool definition:

	tools = {
	    { name=..., description=..., parameters=..., pre_msg=function(args)..., exec=function(args)... },
	    ...
	}

The API definition is in the tool package.
In tools directory, files without the .lua extension are ignored.
*/

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	core "qmicode-go/codelib/agent"
	"qmicode-go/codelib/session"
	"qmicode-go/codelib/tool"
	"sort"
	"strings"

	"github.com/openai/openai-go"
)

// PathAgent 是 core.Agent（定义层）的一个实现：提示词加载 + 工具定义/注册。
type PathAgent struct {
	Name          string
	SysPromptPath string
	ToolsPath     string
	CptPromptPath string

	SysPrompt  string
	CptPrompt  string
	Tools      []tool.LuaToolGroup
	ExtraTools []tool.Tool
}

func LoadPathAgent(agentPath string, name string) (*PathAgent, error) {
	agent := PathAgent{
		Name:          name,
		SysPromptPath: filepath.Join(agentPath, "sysprompt"),
		ToolsPath:     filepath.Join(agentPath, "tools"),
		CptPromptPath: filepath.Join(agentPath, "cptprompt"),
	}

	entries, err := os.ReadDir(agent.SysPromptPath)
	if err != nil {
		return &agent, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(agent.SysPromptPath, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return &agent, err
			}
			agent.SysPrompt += string(content) + "\n"
		}
	}

	entries, err = os.ReadDir(agent.ToolsPath)
	if err != nil {
		return &agent, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".lua") {
			path := filepath.Join(agent.ToolsPath, entry.Name())
			group, err := tool.LoadToolsFromScript(path)
			if err != nil {
				agent.Destroy()
				return &agent, fmt.Errorf("failed to load tools %s: %v", path, err)
			}
			agent.Tools = append(agent.Tools, *group)
		}
	}

	// 跨文件重名检测：同一工具名只能出现在一个文件中
	seen := make(map[string]string) // name -> 首次出现的文件路径
	for i := range agent.Tools {
		for j := range agent.Tools[i].Tools {
			t := &agent.Tools[i].Tools[j]
			if prev, dup := seen[t.Name]; dup {
				agent.Destroy()
				return &agent, fmt.Errorf("duplicate tool name %q in %s and %s", t.Name, prev, agent.Tools[i].Path)
			}
			seen[t.Name] = agent.Tools[i].Path
		}
	}

	entries, err = os.ReadDir(agent.CptPromptPath)
	if err != nil {
		return &agent, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(agent.CptPromptPath, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return &agent, err
			}
			agent.CptPrompt += string(content) + "\n"
		}
	}

	return &agent, nil
}

func (a *PathAgent) GetName() string {
	return a.Name
}

func (a *PathAgent) GetSysPrompt() string {
	return a.SysPrompt
}

func (a *PathAgent) GetCompactPrompt() string {
	return a.CptPrompt
}

// ListTools 返回当前全部可用工具的统一视图（静态 Lua 工具 + 动态注册的工具）。
func (a *PathAgent) ListTools() []tool.Tool {
	res := make([]tool.Tool, 0)
	for i := range a.Tools {
		for j := range a.Tools[i].Tools {
			t := &a.Tools[i].Tools[j]
			res = append(res, t)
		}
	}
	for _, t := range a.ExtraTools {
		res = append(res, t)
	}
	return res
}

// FindTool 按名称查找工具（静态 Lua 工具优先，其次动态注册的工具）。
func (a *PathAgent) FindTool(name string) (tool.Tool, bool) {
	for i := range a.Tools {
		for j := range a.Tools[i].Tools {
			t := &a.Tools[i].Tools[j]
			if t.Name == name {
				return t, true
			}
		}
	}
	for _, t := range a.ExtraTools {
		if t.GetName() == name {
			return t, true
		}
	}
	return nil, false
}

func (a *PathAgent) AddTool(tool tool.Tool) bool {
	// 1. 检查是否已经存在
	for i := range a.Tools {
		for j := range a.Tools[i].Tools {
			if a.Tools[i].Tools[j].Name == tool.GetName() {
				return false // 已经存在，不添加
			}
		}
	}
	for i := range a.ExtraTools {
		if a.ExtraTools[i].GetName() == tool.GetName() {
			return false // 已经存在，不添加
		}
	}
	a.ExtraTools = append(a.ExtraTools, tool)
	return true // 添加成功
}

func (a *PathAgent) RemoveTool(name string) bool {
	// 只有ExtraTools中的工具可以被移除
	for i, t := range a.ExtraTools {
		if t.GetName() == name {
			a.ExtraTools = append(a.ExtraTools[:i], a.ExtraTools[i+1:]...)
			return true
		}
	}
	return false // 未找到要移除的工具
}

func (a *PathAgent) GetInitLLMSession(name string) *session.LLMSession {
	res := session.LLMSession{}
	res.SetSessionName(name)
	res.AddMessage(openai.SystemMessage(a.SysPrompt))
	return &res
}

// Destroy 释放加载的资源（Lua 工具组的 LState），只能调用一次。
func (a *PathAgent) Destroy() {
	for i := range a.Tools {
		a.Tools[i].Destroy()
	}
}

// 检查接口实现
var _ core.Agent = (*PathAgent)(nil)
