-- Skill 工具：按需加载 agent 目录下 skills/<name>/SKILL.md 的领域指令。
-- Agent 平时只看到 skill 的名字与描述；任务匹配时调用本工具获取完整正文。
--
-- 目录约定（仅 Agent 级发现）：
--   <agentDir>/skills/<name>/SKILL.md      -- SKILL.md 本体
--   <agentDir>/skills/<name>/...           -- 附带资源（references/、scripts/ 等）
--
-- SKILL.md 格式：YAML frontmatter（仅识别顶层 name / description 两个标量字段，其余忽略），
-- frontmatter 之后的内容作为正文返回。

local function trim(s)
    return (tostring(s):gsub("^%s+", ""):gsub("%s+$", ""))
end

-- 从 SCRIPT_PATH(<agentDir>/tools/skill.lua) 推导 <agentDir>
local agent_dir = (SCRIPT_PATH:gsub("[/\\][^/\\]*$", "")):gsub("[/\\]tools$", "")
local skills_root = agent_dir .. "/skills"

-- 轻量 frontmatter 解析：只取顶层 key: value（标量），忽略列表项与缩进行。
-- 返回 meta(table), body(string)
local function parse_skill_md(text, path)
    if not text:sub(1, 3) == "---" then
        return nil, "missing frontmatter in " .. path
    end
    local first_nl = text:find("\n", 1, true)
    if not first_nl then
        return nil, "malformed frontmatter (no newline) in " .. path
    end
    local close_line = nil
    -- 逐行查找结束标记 "---"
    local lines = {}
    for line in (text .. "\n"):gmatch("(.-)\n") do
        table.insert(lines, line)
    end
    for i = 2, #lines do
        if trim(lines[i]) == "---" then
            close_line = i
            break
        end
    end
    if not close_line then
        return nil, "unterminated frontmatter in " .. path
    end

    local meta = {}
    for i = 2, close_line - 1 do
        local line = lines[i]
        -- 只认顶层 key: value；跳过空行、列表项(- )、缩进行(如 references: 下的列表)
        if not line:match("^%s") and not line:match("^-") then
            local k, v = line:match("^(%S+)%s*:%s*(.*)$")
            if k ~= nil then
                local unquoted = v:match('^"(.*)"$') or v:match("^'(.*)'$")
                meta[k] = trim(unquoted or v)
            end
        end
    end

    local body_lines = {}
    for i = close_line + 1, #lines do
        table.insert(body_lines, lines[i])
    end

    if not meta.name or meta.name == "" then
        return nil, "frontmatter missing 'name' in " .. path
    end
    if not meta.description or meta.description == "" then
        return nil, "frontmatter missing 'description' in " .. path
    end
    return meta, table.concat(body_lines, "\n")
end

-- 扫描 skills_root/<name>/SKILL.md（单层），返回 name -> {dir=..., md=path}，以及错误列表
local function discover_skills()
    local found = {}
    if type(qmi_list_files) ~= "function" then
        return found, {"qmi_list_files API is not available in this runtime"}
    end
    local files, err = qmi_list_files(skills_root, 1000)
    if err ~= nil then
        -- skills 目录不存在等：视为没有可用 skill（不报错）
        return found, {}
    end
    for _, f in ipairs(files) do
        -- 只认 <name>/SKILL.md，且 name 不含路径分隔符
        local name = f:match("^([^/]+)/SKILL%.md$")
        if name ~= nil and not found[name] then
            found[name] = {
                dir = skills_root .. "/" .. name,
                md = skills_root .. "/" .. name .. "/SKILL.md",
            }
        end
    end
    return found, {}
end

local function sorted_names(t)
    local names = {}
    for k in pairs(t) do table.insert(names, k) end
    table.sort(names)
    return names
end

tools = {
    {
        name = "Skill",
        description = table.concat({
            "Load a skill: on-demand domain-specific instructions and workflows.",
            "",
            "Call Skill(name='list') to see the available skills (name + description).",
            "When the current task matches one of them, call Skill with that name to load its full instructions into context.",
        }, "\n"),
        parameters = {
            type = "object",
            properties = {
                name = {
                    type = "string",
                    description = "The skill name to load. Use 'list' to list available skills."
                }
            },
            required = { "name" }
        },

        pre_msg = function(args)
            local name = args.name or ""
            if name == "list" then
                return "Listing available skills"
            end
            return "Loading skill: " .. name
        end,

        exec = function(args)
            local name = args.name or ""
            if name == "" then
                return "", "", "missing required parameter: name"
            end

            local found, _discErrs = discover_skills()

            -- 列出可用 skill
            if name == "list" then
                if next(found) == nil then
                    local toLLM = "No skills are currently available under " .. skills_root
                    return toLLM, toLLM, nil
                end
                local lines = {}
                for _, n in ipairs(sorted_names(found)) do
                    -- 逐个解析取 description（失败则跳过该条）
                    local f, ferr = io.open(found[n].md, "r")
                    if f then
                        local text = f:read("*a") or ""
                        f:close()
                        local meta = parse_skill_md(text, found[n].md)
                        local desc = (meta and meta.description) or "(no description)"
                        table.insert(lines, string.format("- %s: %s", n, desc))
                    end
                end
                local toLLM = "Available skills under " .. skills_root .. ":\n" .. table.concat(lines, "\n")
                return toLLM, toLLM, nil
            end

            -- 加载指定 skill
            local entry = found[name]
            if not entry then
                local names = sorted_names(found)
                local avail = (next(names) ~= nil) and table.concat(names, ", ") or "none"
                return "", "", string.format("Skill %q not found under %s. Available skills: %s", name, skills_root, avail)
            end

            local f, oerr = io.open(entry.md, "r")
            if not f then
                return "", "", "failed to open " .. entry.md .. ": " .. tostring(oerr)
            end
            local text = f:read("*a") or ""
            f:close()

            local meta, body = parse_skill_md(text, entry.md)
            if meta == nil then
                return "", "", body -- body 此处为错误信息
            end

            -- 附带文件清单（采样上限 20，跳过 SKILL.md 自身）
            local fileLines = {}
            local files, ferr = qmi_list_files(entry.dir, 100)
            if ferr == nil then
                for _, rel in ipairs(files) do
                    if rel ~= "SKILL.md" then
                        table.insert(fileLines, "- " .. entry.dir .. "/" .. rel)
                    end
                    if #fileLines >= 20 then break end
                end
            end

            local out = {}
            table.insert(out, string.format("SKILL.md of %s (%s):", meta.name, entry.md))
            table.insert(out, "")
            table.insert(out, trim(body))
            table.insert(out, "")
            table.insert(out, "Base directory: " .. entry.dir)
            table.insert(out, "Relative paths in the skill above are relative to this base directory.")
            if #fileLines > 0 then
                table.insert(out, "")
                table.insert(out, string.format("Files in this skill (sampled, up to 20):"))
                for _, l in ipairs(fileLines) do table.insert(out, l) end
            end

            local toLLM = table.concat(out, "\n")
            return toLLM, "Loaded skill: " .. name .. " (" .. entry.md .. ")", nil
        end,
    },
}
