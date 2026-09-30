package appmcp

import (
	"context"
	"fmt"
	"qmicode-go/codelib/agent"
	"qmicode-go/codelib/tool"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go"
)

type McpTool struct {
	Name        string
	Description string
	Parameters  map[string]any

	McpName    string
	McpSession *mcp.ClientSession
}

var _ tool.Tool = (*McpTool)(nil)

func (t *McpTool) GetName() string {
	return t.Name
}

func (t *McpTool) GetDescription() string {
	return t.Description
}

func (t *McpTool) GetParameters() map[string]any {
	return t.Parameters
}

func (t *McpTool) ToOpenAITool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  t.Parameters,
		},
	}
}

func (t *McpTool) CallPreMsgFunc(args map[string]any) (string, error) {
	return fmt.Sprintf("Mcp %s Call Tool %s", t.McpName, t.Name), nil
}

// CallExecFunc 执行工具。ctx 被取消表示用户中断：Go 侧的阻塞调用（如子进程）应尽快终止，
func (t *McpTool) CallExecFunc(ctx context.Context, args map[string]any) (string, string, error) {
	result, err := t.McpSession.CallTool(
		ctx,
		&mcp.CallToolParams{
			Name:      t.Name,
			Arguments: args,
		},
	)
	if err != nil {
		return "", "", err
	}

	allText := ""

	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok {
			allText += text.Text
			allText += "\n"
		}
	}

	return allText, fmt.Sprintf("Successfully called mcp %s tool %s", t.McpName, t.Name), nil
}

func LoadToolToAgent(tools []*McpTool, ag agent.Agent) error {
	for _, item := range tools {
		ok := ag.AddTool(item)
		if !ok {
			return fmt.Errorf("add tool %s failed", item.Name)
		}
	}
	return nil
}
