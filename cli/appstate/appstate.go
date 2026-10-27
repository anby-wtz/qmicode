package appstate

import (
	"qmicode-go/app/config"
	clisafe "qmicode-go/app/safe"
	"qmicode-go/codelib/agent"
	coremc "qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"
)

// App 汇总 CLI 运行期需要读写的全部状态。
// 它只是把原先散落在 process_special_input 参数表与 cli 包级变量中的值收拢到一个结构体，
// 不提供任何方法、不持有行为，由 cli.Start 创建一次并按指针传递。
type App struct {
	Conf      config.Config       // 配置
	Rt        agent.AgentRuntime  // 运行时（无状态）
	Ag        agent.Agent         // 当前 Agent
	AgentName string              // 当前 Agent 名（用于显示与比较）
	Model     coremc.ModelConfig  // 当前模型配置
	ModelName string              // 当前模型名（用于显示与持久化）
	Sess      *session.LLMSession // 当前会话（就地修改）
	Safe      clisafe.SafeConfig  // 安全配置（原 sf 参数与包级 safeConfig 合并为此一处）

	// ExitRequested 由 /exit、/quit 置位，cli.Start 的主循环据此退出，
	// 避免在命令处理中直接 os.Exit 而跳过 defer 的资源释放。
	ExitRequested bool
}
