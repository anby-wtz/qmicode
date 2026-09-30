package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"qmicode-go/app/config"
	appsess "qmicode-go/app/session"
	"qmicode-go/codelib/agent"
	coremc "qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"
)

// runAgentLoop 执行一轮完整的 Agent 循环（LLM 请求 + 工具调用）。
// 在独立 goroutine 中运行；监听 SIGINT，收到信号即取消 context，从而中断：
//   - LLM 流式响应（openai-go 中止 HTTP 读取）
//   - Lua 工具的子进程（exec_with_timeout 使用同一 ctx）
//
// 返回 agent.ErrInterrupted 表示被用户 Ctrl-C 中断；其余 error 为真实错误。
func runAgentLoop(ag agent.Agent, model *coremc.ModelConfig, sess *session.LLMSession, conf config.Config) (err error) {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	var once sync.Once
	interrupted := func() {
		once.Do(func() {
			cancel()
			fmt.Printf("\n%s[Interrupted] cancelling...%s", Yellow, Reset)
		})
	}

	done := make(chan error, 1)
	go func() {
		defer cancel() // 无论成功失败，确保资源释放

		for {
			tcs, err := ag.Request(ctx, *model, sess, resCallback)
			if ctx.Err() != nil {
				err = agent.ErrInterrupted // Request 中断时已把部分输出保留进会话
			}
			if err != nil || tcs == nil {
				done <- err
				return
			}

			err = ag.ExecuteTools(ctx, tcs, sess, premsgCallback, aftermsgCallback)
			if ctx.Err() != nil && !errors.Is(err, agent.ErrInterrupted) {
				err = agent.ErrInterrupted
			}

			appsess.SaveSession(conf, sess)

			if err != nil {
				done <- err
				return
			}
			// 工具全部执行完毕，继续下一轮 LLM 请求
		}
	}()

	select {
	case res := <-done:
		return res
	case s := <-sigCh:
		interrupted()
		fmt.Printf(" (signal: %v)", s)
		res := <-done
		return res
	}
}
