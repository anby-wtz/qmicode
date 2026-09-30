-- 文件操作工具组：ReadFile / WriteFile / EditFile
-- 同文件内的工具可以共享局部辅助函数。

local function short(s)
    s = tostring(s):gsub("\n", "\\n")
    if #s > 60 then
        return s:sub(1, 57) .. "..."
    end
    return s
end


-- Count literal (non-overlapping) occurrences of sub in s, returning the line number of each match.
local function find_all(s, sub)
    local lines = {}
    local pos = 1
    while true do
        local start_i, end_i = s:find(sub, pos, true)
        if not start_i then
            break
        end
        -- Compute the line number of this match (count '\n' before it).
        local line = 1
        for i = 1, start_i - 1 do
            if s:byte(i) == 10 then
                line = line + 1
            end
        end
        table.insert(lines, line)
        pos = end_i + 1
    end
    return lines
end


tools = {
    {
        name = "ReadFile",
        description = "Read the content of a file and return its contents.",
        parameters = {
            type = "object",
            properties = {
                path = {
                    type = "string",
                    description = "The absolute or relative path to the file to read."
                },
                offset = {
                    type = "integer",
                    description = "The number of lines to skip from the beginning. Default: 0"
                },
                limit = {
                    type = "integer",
                    description = "The maximum number of lines to read. Default: 2000"
                }
            },
            required = { "path" }
        },

        pre_msg = function(args)
            local path = args.path or ""
            local offset = args.offset or 0
            local limit = args.limit or 2000
            return "Reading file: " .. path .. " (offset: " .. offset .. ", limit: " .. limit .. ")"
        end,

        exec = function(args)
            local path = args.path or ""
            local offset = args.offset or 0
            local limit = args.limit or 2000

            -- 参数校验
            if path == "" then
                return "", "", "missing required parameter: path"
            end
            if offset < 0 then
                return "", "", "invalid offset: must be non-negative"
            end
            if limit <= 0 then
                return "", "", "invalid limit: must be positive"
            end

            local file, err = io.open(path, "r")
            if not file then
                return "", "", "failed to open file: " .. err
            end

            -- 跳过 offset 行，若提前到达文件尾则报错
            local skipped = 0
            while skipped < offset do
                local line = file:read("*l")
                if not line then
                    file:close()
                    return "", "", "offset " .. offset .. " exceeds file line count (actual: " .. skipped .. ")"
                end
                skipped = skipped + 1
            end

            -- 读取最多 limit 行
            local lines = {}
            local count = 0
            while count < limit do
                local line = file:read("*l")
                if not line then
                    break   -- 正常文件结束
                end
                table.insert(lines, line)
                count = count + 1
            end
            file:close()

            local content = table.concat(lines, "\n")
            local toLLM = "File content (offset: " .. offset .. ", limit: " .. limit .. "):\n" .. content
            return toLLM, "Read file " .. path .. " successfully.", nil
        end,
    },

    {
        name = "WriteFile",
        description = "Write content to a file. Creates the file if it doesn't exist, or overwrites if it does.",
        parameters = {
            type = "object",
            properties = {
                path = {
                    type = "string",
                    description = "The absolute or relative path to the file to write."
                },
                content = {
                    type = "string",
                    description = "The content to write to the file."
                }
            },
            required = { "path", "content" }
        },

        pre_msg = function(args)
            local path = args.path or ""
            return "Writing file: " .. path
        end,

        exec = function(args)
            local path = args.path or ""
            local content = args.content

            -- Parameter validation
            if path == "" then
                return "", "", "missing required parameter: path"
            end
            if content == nil then
                return "", "", "missing required parameter: content"
            end

            local file, err = io.open(path, "w")
            if not file then
                return "", "", "failed to open file for writing: " .. tostring(err)
            end

            local ok, werr = pcall(function()
                file:write(content)
            end)
            if not ok then
                file:close()
                return "", "", "failed to write content: " .. tostring(werr)
            end

            -- Ensure data is flushed and file is properly closed
            local close_ok, close_err = pcall(function()
                file:close()
            end)
            if not close_ok then
                return "", "", "failed to close file: " .. tostring(close_err)
            end

            local toLLM = "File written successfully: " .. path
            return toLLM, "Wrote file: " .. path, nil
        end,
    },

    {
        name = "EditFile",
        description = "Edit a file by replacing an exact string match. The old_string must appear exactly once in the file; if it matches zero or multiple times, an error is returned.",
        parameters = {
            type = "object",
            properties = {
                path = {
                    type = "string",
                    description = "The absolute or relative path to the file to edit."
                },
                old_string = {
                    type = "string",
                    description = "The exact text to find. Must match exactly once in the file (literal matching, not a pattern)."
                },
                new_string = {
                    type = "string",
                    description = "The text to replace old_string with. Optional; an empty string deletes the matched text."
                }
            },
            required = { "path", "old_string" }
        },

        pre_msg = function(args)
            local path = args.path or ""
            local old_s = short(args.old_string or "")
            local new_s = short(args.new_string ~= nil and args.new_string or "")
            return "Editing file: " .. path .. "\n- " .. old_s .. "\n+ " .. new_s
        end,

        exec = function(args)
            local path = args.path or ""
            local old_string = args.old_string
            local new_string = args.new_string or ""

            -- Parameter validation
            if path == "" then
                return "", "", "missing required parameter: path"
            end
            if old_string == nil or old_string == "" then
                return "", "", "invalid old_string: must be a non-empty string"
            end

            local file, err = io.open(path, "r")
            if not file then
                return "", "", "failed to open file for reading: " .. tostring(err)
            end
            local content = file:read("*a") or ""
            file:close()

            -- Strict uniqueness check: old_string must match exactly once.
            local matches = find_all(content, old_string)
            local n = #matches

            if n == 0 then
                return "", "", "old_string not found in file (0 matches). Provide a larger and more unique context."
            end
            if n > 1 then
                local lines_str = table.concat(matches, ", ")
                return "", "", string.format(
                    "old_string matched %d times (at lines: %s). It must match exactly once; add surrounding context to make it unique.",
                    n, lines_str)
            end

            -- Exactly one match: splice literally so new_string is never interpreted as a pattern.
            local start_i, end_i = content:find(old_string, 1, true)
            local new_content = content:sub(1, start_i - 1) .. new_string .. content:sub(end_i + 1)

            local out, oerr = io.open(path, "w")
            if not out then
                return "", "", "failed to open file for writing: " .. tostring(oerr)
            end
            local ok, werr = pcall(function()
                out:write(new_content)
            end)
            if not ok then
                out:close()
                return "", "", "failed to write content: " .. tostring(werr)
            end
            local close_ok, cerr = pcall(function()
                out:close()
            end)
            if not close_ok then
                return "", "", "failed to close file: " .. tostring(cerr)
            end

            local toLLM = "File edited successfully: " .. path
            return toLLM, "Edited file: " .. path, nil
        end,
    },
}
