package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

const validScript = `
local function upper(s) return string.upper(tostring(s)) end

tools = {
    {
        name = "Echo",
        description = "echo back the input",
        parameters = {
            type = "object",
            properties = { text = { type = "string" } },
            required = { "text" },
        },
        pre_msg = function(args) return "echo: " .. tostring(args.text) end,
        exec = function(args)
            local s = args.text or ""
            return upper(s), "human saw: " .. s, nil
        end,
    },
    {
        name = "Run",
        description = "run a command via the injected API",
        parameters = { type = "object", properties = {} },
        pre_msg = function(args) return "running" end,
        exec = function(args)
            local out, errout, timeouted, err = exec_with_timeout(5, "echo", {"hello"})
            if err ~= nil then return "", "", err end
            return out .. errout .. tostring(timeouted), "ran echo", nil
        end,
    },
    {
        name = "Fail",
        description = "always fails",
        parameters = { type = "object", properties = {} },
        pre_msg = function(args) return "failing" end,
        exec = function(args) return "", "", "boom: expected error" end,
    },
}
`

func TestLoadToolsFromScript(t *testing.T) {
	path := writeScript(t, validScript)
	group, err := LoadToolsFromScript(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer group.Destroy()

	if len(group.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(group.Tools))
	}

	echo := &group.Tools[0]
	run := &group.Tools[1]
	failTool := &group.Tools[2]

	if echo.Name != "Echo" || echo.Description != "echo back the input" {
		t.Errorf("unexpected meta: %+v", echo)
	}
	props, ok := echo.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("parameters.properties is not a map: %#v", echo.Parameters)
	}
	if _, ok := props["text"]; !ok {
		t.Errorf("missing property text: %#v", props)
	}

	pre, err := echo.CallPreMsgFunc(map[string]any{"text": "hi"})
	if err != nil || pre != "echo: hi" {
		t.Errorf("pre_msg = %q, err = %v", pre, err)
	}

	res, human, err := echo.CallExecFunc(context.Background(), map[string]any{"text": "abc"})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if res != "ABC" || human != "human saw: abc" {
		t.Errorf("exec = (%q, %q)", res, human)
	}

	// 验证注入的 Go API 可用（同组第二个工具，共享 LState）
	res, _, err = run.CallExecFunc(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("run exec: %v", err)
	}
	if !strings.HasPrefix(res, "hello") || strings.Contains(res, "true") {
		t.Errorf("unexpected run result: %q", res)
	}

	// 错误路径：exec 返回第3个值非 nil 时应转为 Go error
	if _, _, err = failTool.CallExecFunc(context.Background(), map[string]any{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected exec error containing %q, got %v", "boom", err)
	}

	group.Destroy() // 验证 Destroy 幂等
	group.Destroy()
}

func TestLoadToolsFromScriptErrors(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		wantSub string
	}{
		{"no tools global", `name = "X"`, "'tools' must be a table"},
		{"empty tools", `tools = {}`, "'tools' is empty"},
		{"entry not table", `tools = { "x" }`, "tools[1] must be a table"},
		{"missing name", `tools = { { description = "d", parameters = { type="object" }, pre_msg=function() end, exec=function() end } }`, ".name must be a non-empty string"},
		{"empty desc", `tools = { { name = "n", parameters = { type="object" }, pre_msg=function() end, exec=function() end } }`, ".description must be a non-empty string"},
		{"params not table", `tools = { { name = "n", description = "d", parameters = 1, pre_msg=function() end, exec=function() end } }`, ".parameters must be a table"},
		{"pre_msg not func", `tools = { { name = "n", description = "d", parameters = { type="object" }, pre_msg = 1, exec=function() end } }`, ".pre_msg must be a function"},
		{"exec not func", `tools = { { name = "n", description = "d", parameters = { type="object" }, pre_msg=function() end, exec = nil } }`, ".exec must be a function"},
		{
			"duplicate names in file",
			`tools = {
				{ name = "dup", description = "a", parameters = { type="object" }, pre_msg=function() end, exec=function() end },
				{ name = "dup", description = "b", parameters = { type="object" }, pre_msg=function() end, exec=function() end },
			}`,
			"duplicate tool name \"dup\"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeScript(t, tc.script)
			group, err := LoadToolsFromScript(path)
			if err == nil {
				group.Destroy()
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want contains %q", err, tc.wantSub)
			}
		})
	}
}

func TestLoadToolsFromScriptBadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.lua")
	if _, err := LoadToolsFromScript(path); err == nil {
		t.Fatal("expected error for missing file")
	}
}
