package appmcp

import (
	"context"
	"encoding/json"
	"os/exec"
	"qmicode-go/codelib/agent"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ConnectLocalMcp(ctx context.Context, name string, cmd string, workdir string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name: name,
		},
		nil,
	)
	c := exec.Command(cmd)
	c.Dir = workdir
	session, err := client.Connect(
		ctx,
		&mcp.CommandTransport{
			Command: c,
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	return session, err
}

func ConnectRemoteMcp(ctx context.Context, name string, endpoint string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name: name,
		},
		nil,
	)

	session, err := client.Connect(
		ctx,
		&mcp.StreamableClientTransport{
			Endpoint: endpoint,
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	return session, err
}

func GetToolsFromMcpSession(ctx context.Context, name string, sess *mcp.ClientSession) ([]*McpTool, error) {
	res := make([]*McpTool, 0)
	for tool, err := range sess.Tools(
		ctx,
		&mcp.ListToolsParams{},
	) {
		if err != nil {
			return nil, err
		}

		data, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, err
		}

		var schema map[string]any

		err = json.Unmarshal(data, &schema)
		if err != nil {
			return nil, err
		}

		res = append(res, &McpTool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  schema,

			McpName:    name,
			McpSession: sess,
		})

	}

	return res, nil
}

type RemoteMcpConfig struct {
	Name     string
	Endpoint string
}

type LocalMcpConfig struct {
	Name    string
	Command string
	Workdir string
}

type McpType int

const (
	McpTypeRemote McpType = iota
	McpTypeLocal
)

func ConfigAllMcp(ctx context.Context, remote []*RemoteMcpConfig, local []*LocalMcpConfig, ag agent.Agent, connectCallBack func(tp McpType, name string)) error {
	for _, r := range remote {
		if connectCallBack != nil {
			connectCallBack(McpTypeRemote, r.Name)
		}
		sess, err := ConnectRemoteMcp(ctx, r.Name, r.Endpoint)
		if err != nil {
			return err
		}
		tools, err := GetToolsFromMcpSession(ctx, r.Name, sess)
		if err != nil {
			return err
		}
		err = LoadToolToAgent(tools, ag)
		if err != nil {
			return err
		}
	}

	for _, l := range local {
		if connectCallBack != nil {
			connectCallBack(McpTypeLocal, l.Name)
		}
		sess, err := ConnectLocalMcp(ctx, l.Name, l.Command, l.Workdir)
		if err != nil {
			return err
		}
		tools, err := GetToolsFromMcpSession(ctx, l.Name, sess)
		if err != nil {
			return err
		}
		err = LoadToolToAgent(tools, ag)
		if err != nil {
			return err
		}
	}

	return nil
}
