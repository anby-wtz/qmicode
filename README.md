# QmiCode 轻量的通用Agent框架

## 1. 快速开始

TODO

## 2. 介绍

TODO

## 3. 项目概述

TODO

## 4. 核心概念

项目主要分为6个核心概念：

1. Agent：定义了模型上下文的系统提示词和工具（见agent包）
2. AgentRuntime：定义了模型的运行时环境，例如OpenAIRuntime（见agent包）
2. LLMSession：一次对话的上下文（见session包）
3. LLMTool：模型可用的工具，由Go的.so插件动态加载（见tool包）
4. ModelConfig: 模型的配置，包括BaseURL、APIKEY等（见modelconfig包）
5. Safe：安全策略，用于限制Agent的工具调用和运行环境（见safe包）

Agent定义的是模型的脚手架，LLMSession定义了模型的上下文，ModelConfig定义具体调用哪个LLM，三者相互独立。

Agent和LLMSession唯一不独立的地方是，初始LLMSession产生于Agent，因为Agent有系统提示词。但是我们规定，在已有对话内容时，切换Agent不会影响已有的对话内容，但会重设系统提示词。

## 5. 工具调用

我们使用.lua脚本定义工具。每个.lua文件对应一个工具。

目前，我们不支持一个脚本定义多个插件，因此，目前不支持直接把MCP工具列表直接复制到Function Call工具列表。

每个.lua脚本仅包含导出符号而无需在加载时执行代码。其符号如下：

```lua
-- 工具的元信息
name = "工具名称"
description = "工具描述"
parameters = {}  -- table类型，符合OpenAI兼容接口的Function Call参数定义

-- 工具的实现函数
-- 参数：args为table类型，符合OpenAI兼容接口的Function Call返回参数定义
-- 返回值：返回值1：string类型，向LLM展示的结果；返回值2：string类型，向人类展示的结果；返回值3：nil或错误信息
exec = function(args) 
    -- 工具实现逻辑
    return result, human_result, error_msg
end

-- 执行前向用户发送的提示信息
-- 参数：args为table类型，符合OpenAI兼容接口的Function Call返回参数定义
-- 返回值：string类型，向人类展示的结果
pre_msg = function(args)
    -- 工具执行前的提示信息逻辑
    return human_result
end
```

例如，典型的`bash`工具如下：

```lua
name = "Bash"
description = "Bash tool enable you to execute command line."
parameters = {
    type = "object",
    properties = {
        command = {
            type = "string",
            description = "Shell command to execute."
        },
        shell = {
            type = "string",
            description = "Which shell to use (e.g., bash, sh). Default: bash"
        },
        timeout = {
            type = "integer",
            description = "Timeout in seconds. Default: 600"
        }
    }
}

pre_msg = function(args)
    local command = args.command or ""
    local shell = args.shell or "bash"
    local timeout = args.timeout or 600
    
    if command == "" then
        return "command not provided"
    end
    
    return "(" .. shell .. ", " .. timeout .. "s)$" .. command
end

exec = function(args)
    local command = args.command or ""
    local shell = args.shell or "bash"
    local timeout = args.timeout or 600
    
    if command == "" then
        return "", "", "missing required parameter: command"
    end
    
    if timeout <= 0 then
        timeout = 600
    end
    
    -- 构建命令行参数
    local cmdArgs = {"-c", command}

    local output, stderr, timeouted, err = exec_with_timeout(timeout, shell, cmdArgs)
    
    if err ~= nil then
        return "", "", err
    end
    
    local toLLM = "stdout:\n" .. output .. "\nstderr:\n" .. stderr
    return toLLM, toLLM, nil
end
```

需要注意的是，我们使用gopher-lua库来执行lua脚本，因此，需要确保脚本中使用的函数在gopher-lua中可用。

### 5.1 qmicode提供的Lua API

#### 5.1.1 exec_with_timeout(timeout, command, args)

##### 函数原型

```lua
stdout, stderr, timeouted, error = exec_with_timeout(timeout, command, args)
```

##### 功能描述

执行一个外部进程并返回执行结果，支持超时控制。该函数会启动指定的命令行程序，并等待其执行完成或达到超时时间。

##### 参数说明

1. timeout (数字类型)
类型: number
含义: 超时时间（秒）
要求: 必须为正整数
说明: 如果进程在指定时间内未完成执行，则强制终止进程并返回超时标志

2. command (字符串类型)
类型: string
含义: 要执行的命令路径或命令名称
说明: 可以是完整路径，也可以是系统PATH中可找到的命令

3. args (表类型)
类型: table
含义: 命令行参数列表
要求: 必须是数组形式的表（table）
说明: 表中的每个元素都应为字符串类型，将作为命令的参数传入

##### 返回值说明

1. stdout (字符串类型)
含义: 进程的标准输出内容
说明: 如果进程成功执行，返回标准输出的内容；如果出错则返回空字符串

2. stderr (字符串类型)
含义: 进程的标准错误输出内容
说明: 如果进程有错误输出，返回错误信息；如果没有错误则返回空字符串

3. timeouted (布尔类型)
含义: 是否发生超时
返回值:
true: 进程执行超时被强制终止
false: 进程正常执行完成

4. error (字符串类型)
含义: 执行过程中发生的错误信息
说明:
如果执行成功，返回 nil
如果发生错误，返回具体的错误描述信息

#### 5.1.2 qmi_list_files(root, limit)

##### 函数原型

```lua
files, error = qmi_list_files(root, limit)
```

##### 功能描述

递归列出目录 `root` 下的所有普通文件，返回相对路径数组（以 `/` 分隔）。跨平台一致，无需依赖 shell。

##### 参数说明

1. root (字符串类型)：要扫描的目录；不存在或不是目录时返回错误
2. limit (数字类型，可选)：最多返回的文件数；<=0 时使用默认上限(1000)

##### 返回值说明

1. files (表类型)：相对路径数组（数组形式）
2. error (字符串类型/nil)：出错时返回错误描述，成功时为 nil

注意：跳过隐藏文件与目录（任一路径层以 `.` 开头），只返回普通文件。

#### 5.1.3 qmi_is_interrupted()

##### 函数原型

```lua
interrupted = qmi_is_interrupted()
```

##### 功能描述

检查当前工具调用是否已被用户中断（如 CLI 中的 Ctrl-C）。该函数基于本次调用的 context，在 Go 侧执行阻塞调用（`exec_with_timeout`）时同样生效——一旦中断，子进程树会被强制终止。

##### 返回值说明

1. interrupted (布尔类型)
含义: 是否已被用户中断
true: 已收到中断请求，脚本应尽快清理并返回
false: 未被中断

##### 使用约定

gopher-lua 无法从外部强制中止 Lua VM 的纯计算型长循环，因此**Lua 工具脚本应在长时间循环中主动检查该函数并提前退出**：

```lua
exec = function(args)
    local results = {}
    for i = 1, 1000 do
        if qmi_is_interrupted() then
            return "interrupted after " .. (i - 1) .. " iterations", "", nil
        end
        -- ...处理第 i 项
    end
    return table.concat(results, "\n"), "", nil
end
```

### 5.2 提供的Lua变量注入

```lua
SCRIPT_PATH = "/path/to/script.lua" -- 脚本路径
```

### 5.3 Skill（按需加载的领域指令）

Skill 用于把"某类任务的完整操作指引"从系统提示词中剥离出来，按需注入上下文：Agent 平时只看到 skill 的名字与描述，任务匹配时通过 `Skill` 工具加载其完整正文。

目录约定（仅 Agent 级发现，与 sysprompt/tools/cptprompt 平级）：

```
<agentDir>/skills/<name>/SKILL.md      # SKILL.md 本体（frontmatter + 正文）
<agentDir>/skills/<name>/...           # 附带资源（references/、scripts/ 等任意文件）
```

`SKILL.md` 必须以 YAML frontmatter 开头，至少包含两个标量字段：

- `name`：skill 名（必填；不对 name 做格式校验，但建议与目录名一致）
- `description`：一句话说明何时该用这个 skill（必填，用于让模型判断是否加载）

其余 frontmatter 字段（如 license、references 列表等）会被忽略。frontmatter 之后的全部内容作为正文返回给 LLM。

使用方式：

- `Skill(name='list')`：列出当前 agent 下所有可用 skill（名称 + 描述）
- `Skill(name='<name>')`：加载指定 skill，输出格式为 `SKILL.md of <name> (<path>):` + 正文 + Base directory + 附带文件清单（采样上限20个）。正文中的相对路径均以该 skill 目录为基准。

示例见仓库内 `general-agent/skills/git-release/`。

### 5.4 中断机制（Ctrl-C）

CLI 中按 Ctrl-C 会取消当前正在进行的请求，具体行为：

1. **LLM 流式响应过程中**：立即停止接收。已产出的部分输出会保留为一条 assistant message 写入会话历史；未完成的 tool_calls 不会被记录（保证会话合法）。
2. **工具执行过程中**：`exec_with_timeout` 启动的子进程树会被强制终止，当前工具的结果按错误处理写回会话；同一轮中尚未开始执行的工具会被标记为 `Aborted by user`。
3. **/compact 过程中**：立即停止，会话状态保持不变。

对 codelib 的调用方（如嵌入 QmiCode 的程序），中断语义由 `context.Context` 表达：取消传给 `Request` / `ExecuteTools` / `Compact` 的 ctx 即可；被用户中断时这些方法返回 `agent.ErrInterrupted`，可用 `errors.Is(err, agent.ErrInterrupted)` 识别。
