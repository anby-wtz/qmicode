// Package controller provides the small, UI-independent conversation loop used by
// the desktop client. A Controller must only be used by one worker at a time.
package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appagent "qmicode-go/app/agent"
	"qmicode-go/app/appmcp"
	"qmicode-go/app/config"
	appmodel "qmicode-go/app/modelconfig"
	appsafe "qmicode-go/app/safe"
	appsession "qmicode-go/app/session"
	"qmicode-go/codelib/agent"
	"qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Info is a snapshot; the UI never reads the mutable session itself.
type Info struct {
	AgentName   string
	ModelName   string
	ModelID     string
	SessionName string
	SessionPath string
	Usage       int
	NeedConfirm bool
	HasModel    bool
	ToolCount   int
}

type HistoryItem struct {
	Role      string
	Text      string
	Reasoning string
	// AttachmentCount and ToolCallNames are structured so the frontend renders
	// them in its own language instead of parsing localized text.
	AttachmentCount int
	ToolCallNames   []string
}

// ToolRequest includes the original arguments, not just the tool's pre_msg.
type ToolRequest struct {
	ID        string
	Name      string
	Arguments string
	Message   string
}

// Callbacks run on the conversation worker. Confirm must also unblock when its
// context is cancelled. A missing Confirm callback always denies execution.
type Callbacks struct {
	ResponseStart func()
	Chunk         agent.ChunkResultFuncCallback
	ResponseEnd   func()
	Confirm       func(context.Context, ToolRequest) bool
	ToolResult    func(ToolRequest, string, error)
}

type Controller struct {
	conf      config.Config
	ag        agent.Agent
	runtime   agent.AgentRuntime
	model     modelconfig.ModelConfig
	modelName string
	sess      *session.LLMSession
	safe      appsafe.SafeConfig
	mcp       []*mcp.ClientSession
}

// New loads the CLI-compatible Agent, model, safety and MCP configuration. The
// GUI remains available without a default model so one can be created in-app.
func New(ctx context.Context, conf config.Config, status func(string)) (_ *Controller, err error) {
	c := &Controller{conf: conf, runtime: agent.NewAgentRuntime()}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	if err = conf.Init(); err != nil {
		return nil, fmt.Errorf("初始化配置目录: %w", err)
	}
	c.safe = appsafe.SafeConfig{Environment: appsafe.SafeEnvironmentRealSystem, NeedConfirm: true}
	if safeErr := c.safe.LoadFromDefault(conf); safeErr != nil && status != nil {
		status("安全配置读取失败，已默认启用工具调用确认")
	}
	c.modelName, err = appmodel.LoadDefaultModel(&c.model, conf)
	if err != nil || strings.TrimSpace(c.model.Model) == "" {
		c.modelName = ""
		c.model = modelconfig.ModelConfig{}
		if status != nil {
			status("尚未配置有效模型，可在 GUI 中添加或切换模型")
		}
		err = nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if status != nil {
		status("正在加载 Agent：" + conf.GeneralAgentName)
	}
	c.ag, err = appagent.LoadPathAgent(conf.GetAgentPath(conf.GeneralAgentName), conf.GeneralAgentName)
	if err != nil {
		return nil, fmt.Errorf("加载 Agent: %w；尚未安装时请在项目目录执行 qmicode install", err)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	mcpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c.mcp, err = connectMCP(mcpCtx, c.conf, c.ag.GetName(), c.ag, status)
	if err != nil {
		return nil, err
	}
	c.resetSession()
	return c, nil
}

func connectMCP(ctx context.Context, conf config.Config, agentName string, ag agent.Agent, status func(string)) ([]*mcp.ClientSession, error) {
	path := conf.GetAgentMcpConfigFilePath(agentName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("检查 MCP 配置: %w", err)
	}
	list, err := appmcp.GetMcpListConfigFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 MCP 配置: %w", err)
	}
	var sessions []*mcp.ClientSession
	closeOnError := func(cause error) ([]*mcp.ClientSession, error) {
		return nil, errors.Join(cause, closeMCPSessions(sessions))
	}
	attach := func(name string, connect func() (*mcp.ClientSession, error)) error {
		if status != nil {
			status("正在连接 MCP：" + name)
		}
		sess, err := connect()
		if err != nil {
			return fmt.Errorf("连接 MCP %s: %w", name, err)
		}
		// Own the session immediately, including when listing/registering fails.
		sessions = append(sessions, sess)
		tools, err := appmcp.GetToolsFromMcpSession(ctx, name, sess)
		if err != nil {
			return fmt.Errorf("获取 MCP %s 工具: %w", name, err)
		}
		if err := appmcp.LoadToolToAgent(tools, ag); err != nil {
			return fmt.Errorf("注册 MCP %s 工具: %w", name, err)
		}
		return nil
	}
	for _, remote := range list.Remote {
		if err := attach(remote.Name, func() (*mcp.ClientSession, error) {
			return appmcp.ConnectRemoteMcp(ctx, remote.Name, remote.Endpoint)
		}); err != nil {
			return closeOnError(err)
		}
	}
	for _, local := range list.Local {
		if err := attach(local.Name, func() (*mcp.ClientSession, error) {
			return appmcp.ConnectLocalMcp(ctx, local.Name, local.Cmd, local.Args, conf.GetAgentPath(agentName))
		}); err != nil {
			return closeOnError(err)
		}
	}
	return sessions, nil
}

func closeMCPSessions(sessions []*mcp.ClientSession) error {
	var err error
	for _, sess := range sessions {
		err = errors.Join(err, sess.Close())
	}
	return err
}

func (c *Controller) Info() Info {
	info := Info{
		AgentName: c.ag.GetName(), ModelName: c.modelName, ModelID: c.model.Model,
		SessionName: c.sess.Name, SessionPath: c.conf.GetSessionPath(c.sess.Name),
		Usage: c.sess.Usage, NeedConfirm: c.safe.NeedConfirm,
		HasModel: strings.TrimSpace(c.model.Model) != "",
	}
	info.ToolCount = len(c.ag.ListTools())
	return info
}

func (c *Controller) ListAgents() ([]string, error) {
	names, err := c.conf.ListAgents()
	if err != nil {
		return nil, err
	}
	agents := make([]string, 0, len(names))
	for _, name := range names {
		info, err := os.Stat(c.conf.GetAgentPath(name))
		if err != nil {
			continue
		}
		if info.IsDir() {
			agents = append(agents, name)
		}
	}
	sort.Strings(agents)
	return agents, nil
}

func (c *Controller) SwitchAgent(ctx context.Context, name string, status func(string)) error {
	if name == c.ag.GetName() {
		return nil
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("Agent 名称不能为空")
	}
	if err := c.Save(); err != nil {
		return err
	}
	if status != nil {
		status("正在加载 Agent：" + name)
	}
	nextAgent, err := appagent.LoadPathAgent(c.conf.GetAgentPath(name), name)
	if err != nil {
		if nextAgent != nil {
			nextAgent.Destroy()
		}
		return fmt.Errorf("加载 Agent %q：%w", name, err)
	}
	if err := ctx.Err(); err != nil {
		nextAgent.Destroy()
		return err
	}
	mcpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	nextMCP, err := connectMCP(mcpCtx, c.conf, name, nextAgent, status)
	if err != nil {
		nextAgent.Destroy()
		return err
	}

	oldAgent, oldMCP := c.ag, c.mcp
	c.ag, c.mcp = nextAgent, nextMCP
	c.sess.SetSystemMessage(nextAgent.GetSysPrompt())
	oldCloseErr := closeMCPSessions(oldMCP)
	oldAgent.Destroy()
	if oldCloseErr != nil {
		return fmt.Errorf("Agent 已切换，但关闭旧 MCP 连接失败：%w", oldCloseErr)
	}
	return nil
}

func (c *Controller) ListModels() ([]string, error) {
	return appmodel.ListModel(c.conf)
}

func (c *Controller) SwitchModel(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("模型名称不能为空")
	}
	models, err := c.ListModels()
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range models {
		if candidate == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("未找到模型配置 %q", name)
	}
	var next modelconfig.ModelConfig
	if err := appmodel.Load(&next, c.conf, name); err != nil {
		return fmt.Errorf("加载模型配置 %q：%w", name, err)
	}
	if strings.TrimSpace(next.Model) == "" {
		return fmt.Errorf("模型配置 %q 的 model 为空", name)
	}
	if err := appmodel.SetModelToDefault(c.conf, name); err != nil {
		return fmt.Errorf("保存默认模型失败：%w", err)
	}
	c.model = next
	c.modelName = name
	return nil
}

func (c *Controller) CreateModel(name, baseURL, apiKey, modelID string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) || strings.HasSuffix(name, ".yaml") {
		return errors.New("模型配置名称无效")
	}
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(modelID) == "" {
		return errors.New("Base URL 和模型 ID 不能为空")
	}
	models, err := c.ListModels()
	if err != nil {
		return err
	}
	for _, existing := range models {
		if existing == name {
			return fmt.Errorf("模型配置 %q 已存在", name)
		}
	}
	next := modelconfig.ModelConfig{
		BaseURL: strings.TrimSpace(baseURL),
		ApiKey:  apiKey,
		Model:   strings.TrimSpace(modelID),
	}
	if err := appmodel.Save(&next, c.conf, name); err != nil {
		return fmt.Errorf("保存模型配置：%w", err)
	}
	if err := appmodel.SetModelToDefault(c.conf, name); err != nil {
		return fmt.Errorf("模型已保存，但设为默认失败：%w", err)
	}
	c.model = next
	c.modelName = name
	return nil
}

func (c *Controller) ListSessions() ([]string, error) {
	return appsession.ListSessions(c.conf)
}

func (c *Controller) LoadSession(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("会话名称不能为空")
	}
	sessions, err := c.ListSessions()
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range sessions {
		if candidate == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("未找到会话 %q", name)
	}
	if err := c.Save(); err != nil {
		return err
	}
	loaded := &session.LLMSession{}
	if err := appsession.LoadSession(c.conf, name, loaded); err != nil {
		return fmt.Errorf("加载会话 %q：%w", name, err)
	}
	loaded.SetSystemMessage(c.ag.GetSysPrompt())
	c.sess = loaded
	return nil
}

func (c *Controller) History() []HistoryItem {
	items := make([]HistoryItem, 0)
	for _, message := range c.sess.GetMessageList() {
		if message == nil {
			continue
		}
		item := HistoryItem{Role: message.Role}
		if message.Type == session.LLMMessageTypeCompact {
			item.Role = "compact"
		}
		if message.Content != nil {
			if message.Content.Text != nil {
				item.Text = *message.Content.Text
			}
			if message.Content.Reasoning != nil {
				item.Reasoning = *message.Content.Reasoning
			}
			item.AttachmentCount = len(message.Content.ContentParts)
			for _, call := range message.Content.ToolCalls {
				item.ToolCallNames = append(item.ToolCallNames, call.Funtion.Name)
			}
		}
		if item.Text != "" || item.Reasoning != "" || item.AttachmentCount > 0 || len(item.ToolCallNames) > 0 {
			items = append(items, item)
		}
	}
	return items
}

func (c *Controller) SetNeedConfirm(needConfirm bool) error {
	next := c.safe
	next.NeedConfirm = needConfirm
	if err := next.SaveToDefault(c.conf); err != nil {
		return fmt.Errorf("保存安全配置失败：%w", err)
	}
	c.safe = next
	return nil
}

func (c *Controller) resetSession() {
	// No colons or separators: the filename also works on Windows.
	name := "gui-" + time.Now().Format("2006-01-02_15-04-05.000000000")
	c.sess = c.ag.GetInitLLMSession(name)
}

func (c *Controller) NewSession() error {
	if err := c.Save(); err != nil {
		return err
	}
	c.resetSession()
	return nil
}

// SaveError lets frontends distinguish a failed request from unsaved data,
// including when both errors are joined after an interrupted request.
type SaveError struct {
	Path string
	Err  error
}

func (e *SaveError) Error() string {
	return fmt.Sprintf("保存会话失败（%s）: %v", e.Path, e.Err)
}

func (e *SaveError) Unwrap() error { return e.Err }

func (c *Controller) Save() error {
	if err := appsession.SaveSession(c.conf, c.sess); err != nil {
		return &SaveError{Path: c.conf.GetSessionPath(c.sess.Name), Err: err}
	}
	return nil
}

// Run saves the user's message before requesting the model, checkpoints after
// complete tool batches, and saves again on success, failure or cancellation.
func (c *Controller) Run(ctx context.Context, prompt string, cb Callbacks) (err error) {
	if strings.TrimSpace(c.model.Model) == "" {
		return errors.New("尚未配置模型，请先添加或切换模型")
	}
	if strings.TrimSpace(prompt) == "" {
		return errors.New("消息不能为空")
	}
	if cb.Chunk == nil {
		cb.Chunk = func(string, agent.LLMChunkContentType) {}
	}
	c.sess.AddMessage(session.NewUserMessage(&prompt, nil))
	defer func() { err = errors.Join(err, c.Save()) }()
	if err = c.Save(); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return agent.ErrInterrupted
		}
		if cb.ResponseStart != nil {
			cb.ResponseStart()
		}
		calls, requestErr := c.runtime.Request(ctx, c.ag, c.model, c.sess, cb.Chunk)
		if cb.ResponseEnd != nil {
			cb.ResponseEnd()
		}
		if requestErr != nil {
			return requestErr
		}
		if len(calls) == 0 {
			if ctx.Err() != nil {
				return agent.ErrInterrupted
			}
			return nil
		}

		var toolsErr error
		for _, call := range calls {
			request := ToolRequest{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments}
			// Execute one call at a time to associate callbacks with its metadata.
			// Even after cancellation, execute the remaining (now aborted) entries
			// so every tool_call still has a corresponding tool result in history.
			execErr := c.runtime.ExecuteTools(ctx, c.ag, []agent.ToolCall{call}, c.sess,
				func(message string, preErr error) bool {
					request.Message = message
					if preErr != nil {
						if cb.ToolResult != nil {
							cb.ToolResult(request, "", preErr)
						}
						return false
					}
					if ctx.Err() != nil {
						return false
					}
					if !c.safe.NeedConfirm {
						return true
					}
					return cb.Confirm != nil && cb.Confirm(ctx, request) && ctx.Err() == nil
				},
				func(result string, toolErr error) {
					if cb.ToolResult != nil {
						cb.ToolResult(request, result, toolErr)
					}
				})
			toolsErr = errors.Join(toolsErr, execErr)
		}
		if ctx.Err() != nil {
			return agent.ErrInterrupted
		}
		if toolsErr != nil {
			return toolsErr
		}
		if err := c.Save(); err != nil {
			return err
		}
	}
}

// Close is called only after the worker has stopped using the Agent.
func (c *Controller) Close() error {
	var err error
	for _, sess := range c.mcp {
		err = errors.Join(err, sess.Close())
	}
	c.mcp = nil
	if c.ag != nil {
		c.ag.Destroy()
		c.ag = nil
	}
	return err
}
