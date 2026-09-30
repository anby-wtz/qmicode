-- 获取系统信息（由 Go 的 get_os_info 提供）
local sys = get_os_info()
local os_name = sys.os          -- "windows", "linux", "darwin" 等
local arch = sys.arch           -- "amd64", "arm64" 等

-- 根据操作系统选择默认 shell
local shell_default = (os_name == "windows") and "cmd" or "bash"

-- 工具定义（保持原工具名 "Bash"，实际可执行任何 shell）
tools = {
    {
        name = "Bash",
        description = string.format(
            "Execute a shell command. System: %s (%s). Default shell: %s.",
            os_name, arch, shell_default
        ),
        parameters = {
            type = "object",
            properties = {
                command = {
                    type = "string",
                    description = "Shell command to execute."
                },
                shell = {
                    type = "string",
                    description = "Which shell to use (e.g., bash, sh, cmd, powershell). Default: " .. shell_default
                },
                timeout = {
                    type = "integer",
                    description = "Timeout in seconds. Default: 600"
                }
            }
        },
        pre_msg = function(args)
            local command = args.command or ""
            local shell = args.shell or shell_default
            local timeout = args.timeout or 600
            if command == "" then
                return "command not provided"
            end
            return "(" .. shell .. ", " .. timeout .. "s)$ " .. command
        end,
        exec = function(args)
            local command = args.command or ""
            local shell = args.shell or shell_default
            local timeout = args.timeout or 600

            if command == "" then
                return "", "", "missing required parameter: command"
            end
            if timeout <= 0 then timeout = 600 end

            -- 根据 shell 类型构造参数列表（传递给 exec_with_timeout 的第三个参数必须是表）
            local cmdArgs
            if shell == "cmd" then
                cmdArgs = {"/c", command}
            elseif shell == "powershell" then
                cmdArgs = {"-Command", command}
            else
                -- bash, sh, zsh 等均使用 -c
                cmdArgs = {"-c", command}
            end

            -- 调用 Go 注册的 exec_with_timeout(timeout, shell, args_table)
            local output, stderr, timeouted, err = exec_with_timeout(timeout, shell, cmdArgs)

            if err ~= nil then
                return "", "", err
            end

            -- 超时: 保留部分输出并显式告知LLM结果不完整。
            -- 注意不能走 error 通道(返回第3个值非nil)，否则上层 ExecuteTools 会用 "Error: ..." 覆盖 res，丢失已拿到的输出。
            if timeouted then
                local banner = string.format(
                    "[TIMEOUT] command was killed after %ds. Output below is PARTIAL and the process tree has been terminated; do not assume it completed.",
                    timeout)
                if output ~= "" then
                    output = banner .. "\n" .. output
                else
                    output = banner
                end
            end

            local toLLM = "stdout:\n" .. output .. "\nstderr:\n" .. stderr
            return toLLM, toLLM, nil
        end,
    },
}