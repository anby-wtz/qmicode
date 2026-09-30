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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"
	"qmicode-go/codelib/tool"
	"sort"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

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

type LLMChunkContentType int

const (
	ContentReasoning LLMChunkContentType = iota
	ContentAnswer
)

func LoadPathAgent(agentPath string, name string) (*PathAgent, error) {
	agent := PathAgent{
		Name:          name,
		SysPromptPath: filepath.Join(agentPath, "sysprompt"),
		ToolsPath:     filepath.Join(agentPath, "tools"),
		CptPromptPath: filepath.Join(agentPath, "cptprompt"),
	}

	entries, err := os.ReadDir(agent.SysPromptPath)
	if err != nil {
		return &PathAgent{}, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(agent.SysPromptPath, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return &PathAgent{}, err
			}
			agent.SysPrompt += string(content) + "\n"
		}
	}

	entries, err = os.ReadDir(agent.ToolsPath)
	if err != nil {
		return &PathAgent{}, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".lua") {
			path := filepath.Join(agent.ToolsPath, entry.Name())
			group, err := tool.LoadToolsFromScript(path)
			if err != nil {
				agent.Destroy()
				return &PathAgent{}, fmt.Errorf("failed to load tools %s: %v", path, err)
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
				return &PathAgent{}, fmt.Errorf("duplicate tool name %q in %s and %s", t.Name, prev, agent.Tools[i].Path)
			}
			seen[t.Name] = agent.Tools[i].Path
		}
	}

	entries, err = os.ReadDir(agent.CptPromptPath)
	if err != nil {
		return &PathAgent{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(agent.CptPromptPath, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return &PathAgent{}, err
			}
			agent.CptPrompt += string(content) + "\n"
		}
	}

	return &agent, nil

}

func (agent *PathAgent) GetInitLLMSession(name string) *session.LLMSession {
	res := session.LLMSession{}
	res.SetSessionName(name)
	res.AddMessage(openai.SystemMessage(agent.SysPrompt))
	return &res
}

func (a *PathAgent) GetPreMsg(name string, args map[string]any) (string, error) {
	// Lua tools
	for i := range a.Tools {
		for j := range a.Tools[i].Tools {
			t := &a.Tools[i].Tools[j]
			if t.Name == name {
				return t.CallPreMsgFunc(args)
			}
		}
	}
	// ExtraTools（动态添加的工具）
	for i := range a.ExtraTools {
		if a.ExtraTools[i].GetName() == name {
			return a.ExtraTools[i].CallPreMsgFunc(args)
		}
	}
	return "", fmt.Errorf("tool %s not found", name)
}

func (a *PathAgent) ExecuteTool(ctx context.Context, name string, args map[string]any) (string, string, error) {
	for i := range a.Tools {
		for j := range a.Tools[i].Tools {
			t := &a.Tools[i].Tools[j]
			if t.Name == name {
				return t.CallExecFunc(ctx, args)
			}
		}
	}
	for _, t := range a.ExtraTools {
		if t.GetName() == name {
			return t.CallExecFunc(ctx, args)
		}
	}
	return "", "", fmt.Errorf("tool %s not found", name)
}

func (a *PathAgent) Request(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) ([]openai.ChatCompletionMessageToolCall, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := openai.NewClient(
		option.WithBaseURL(config.BaseURL),
		option.WithAPIKey(config.ApiKey),
	)
	openaiTools := make([]openai.ChatCompletionToolParam, 0)
	for i := range a.Tools {
		for j := range a.Tools[i].Tools {
			t := &a.Tools[i].Tools[j]
			openaiTools = append(openaiTools, t.ToOpenAITool())
		}
	}
	if len(a.ExtraTools) > 0 {
		for _, t := range a.ExtraTools {
			openaiTools = append(openaiTools, t.ToOpenAITool())
		}
	}
	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    config.Model,
		Messages: sess.Messages,
		Tools:    openaiTools,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.Opt[bool]{Value: true},
		},
	})
	var acc openai.ChatCompletionAccumulator
	var partialContent string // 已产出的正文内容，用于中断时保留为 assistant message

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		for _, choice := range chunk.Choices {
			raw := choice.Delta.RawJSON()
			var rawMap map[string]string
			err := json.Unmarshal([]byte(raw), &rawMap)
			if err != nil {
				continue
			}
			reasoning, ok := rawMap["reasoning"]
			if ok && reasoning != "" {
				result(reasoning, ContentReasoning)
				continue
			}
			reasoning, ok = rawMap["reasoning_content"]
			if ok && reasoning != "" {
				result(reasoning, ContentReasoning)
				continue
			}

			if choice.Delta.Content != "" {
				partialContent += choice.Delta.Content
				result(choice.Delta.Content, ContentAnswer)
				continue
			}
		}
	}

	if ctx.Err() != nil {
		// 用户中断：保留已产出的部分输出为一条 assistant message（不保存未完成的 tool_calls，避免产生非法会话），然后返回 ErrInterrupted。
		if partialContent != "" {
			sess.AddMessage(openai.AssistantMessage(partialContent))
		}
		return nil, ErrInterrupted
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	msg := acc.Choices[0].Message
	sess.AddMessage(msg.ToParam())

	if len(msg.ToolCalls) > 0 {
		return msg.ToolCalls, nil
	}

	sess.Usage = int(acc.Usage.TotalTokens)

	return nil, nil

}

func (a *PathAgent) ExecuteTools(ctx context.Context, ts []openai.ChatCompletionMessageToolCall, sess *session.LLMSession, premsgfunc PreMsgFilterFuncCallback, aftermsgfunc AfterMsgFuncCallback) error {
	if ctx == nil {
		ctx = context.Background()
	}
	interrupted := false
	for _, tc := range ts {
		args := make(map[string]any)
		json.Unmarshal([]byte(tc.Function.Arguments), &args)

		// 已中断：当前及后续工具全部标记为 Aborted，保证 assistant.tool_calls 与 tool message 一一对应。
		if ctx.Err() != nil || interrupted {
			sess.AddMessage(
				openai.ToolMessage(
					"Aborted by user", tc.ID,
				),
			)
			interrupted = true
			continue
		}

		premsg, err := a.GetPreMsg(tc.Function.Name, args)
		if !premsgfunc(premsg, err) {
			sess.AddMessage(
				openai.ToolMessage(
					"Human Aborted", tc.ID,
				),
			)
			continue
		}

		res, human, err := a.ExecuteTool(ctx, tc.Function.Name, args)
		if err != nil {
			res = fmt.Sprintf("Error: %v", err)
		}
		aftermsgfunc(human, err)
		sess.AddMessage(
			openai.ToolMessage(
				res, tc.ID,
			),
		)

		if ctx.Err() != nil {
			interrupted = true // 工具执行期间被中断：剩余工具在下一轮循环标记为 Aborted
		}
	}
	if interrupted {
		return ErrInterrupted
	}
	return nil
}

func (a *PathAgent) Destroy() {
	for i := range a.Tools {
		a.Tools[i].Destroy()
	}
}

func (a *PathAgent) GetName() string {
	return a.Name
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

// 检查接口实现
var _ Agent = (*PathAgent)(nil)
