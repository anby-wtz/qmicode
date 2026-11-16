package appstate

import (
	"qmicode-go/app/config"
	clisafe "qmicode-go/app/safe"
	"qmicode-go/codelib/agent"
	coremc "qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"
)

// Mode 是本进程使用的运行时模式，启动方式决定，启动后不变。
type Mode int

const (
	ModeOpenAI Mode = iota
	ModeChatGPT
)

type App struct {
	Mode      Mode                // 运行时模式
	Conf      config.Config       // 配置
	Rt        agent.AgentRuntime  // 运行时（无状态）
	Ag        agent.Agent         // 当前 Agent
	AgentName string              // 当前 Agent 名（用于显示与比较）
	Model     coremc.ModelConfig  // 当前模型配置
	ModelName string              // 当前模型名（用于显示与持久化）
	Sess      *session.LLMSession // 当前会话（就地修改）
	Safe      clisafe.SafeConfig  // 安全配置

	// ExitRequested 由 /exit、/quit 置位，cli.Start 的主循环据此退出，
	ExitRequested bool

	AttachImages []string // 附加的图片路径列表（用于多模态）
}
