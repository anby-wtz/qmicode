# QmiCode 桌面 GUI

## 修改边界

GUI 源码位于 `gui/`，复用项目现有的 Agent、模型、会话、安全配置和工具实现。根目录 `main.go` 的 `gui` 子命令启动桌面界面。

GUI 不重新实现或修正原有的取消、进程管理和保存语义。正常的“停止”使用已有 Runtime 的 context 取消接口；只有 GUI 自己退出时，才由 `gui/process*.go` 清理本 GUI 的子进程树。

## 启动

GUI 与项目共用根模块，通过根目录 `main.go` 的 `gui` 子命令启动，不需要构建标签。

在仓库根目录执行：

```bash
CGO_ENABLED=1 go build -o qmicode .
./qmicode gui
```

也可以直接执行：

```bash
go run . gui
```

**启动时的当前目录就是工具操作的工作目录。** 如需操作其他项目，从目标项目目录运行构建后的 `qmicode`，并传入 `gui` 子命令。

GUI 启动后会加载默认 Agent 和模型；没有默认模型时，仍可在界面中添加模型配置。若 Agent 尚未安装，请先在项目目录运行 `qmicode install`，再点击“重试加载”。

构建需要 Go 1.27、CGO、C 编译器，以及 Fyne 所需的 OpenGL 和窗口系统开发依赖。窗口上方是对话区，Agent、模型、会话、安全和外观操作统一放在底部。底部外观设置可从系统已安装字体列表中选择字体、调整字号，并设置浅色/深色/跟随系统主题。设置保存在配置目录的 `config_gui.yaml`（默认 `~/.config/qmicode/config_gui.yaml`），默认外观为浅色。

## 功能

- **Agent**：查看并切换配置目录中已安装的 Agent；切换时保存当前会话，并加载新 Agent 的工具与 MCP 配置。
- **模型**：切换已保存的 OpenAI 兼容模型配置，或在界面中添加配置并设为默认。
- **会话**：加载历史会话、创建新会话、手动保存；加载前自动保存当前会话，消息历史会在界面中重绘。
- **安全**：可开启或关闭每次工具执行前的确认；当前工具运行在真实系统环境中，**没有沙箱**。关闭确认后，模型可能自动执行工具。
- **外观**：从系统可用字体列表中选择字体、调整字号（10–32），切换浅色/深色/跟随系统；设置保存于 `config_gui.yaml` 并在启动时恢复。
- **语言**：界面支持简体中文与英文，可在“外观”设置中切换；切换后按钮、状态、对话历史和待确认的工具调用会立即按新语言重绘。设置保存于 `config_gui.yaml`，默认简体中文。
- **对话**：多行输入、流式输出、可折叠 reasoning、消息复制、自动滚动；Ctrl+Enter 发送，Ctrl+S 保存。
- **停止与退出**：停止会取消当前请求；退出时等待任务收尾、保存会话并清理 GUI 子进程。退出保存失败时会询问是否仍要退出。

快捷键：Enter 换行，Ctrl+Enter 发送，Ctrl+S 保存。

存档默认位于 `~/.config/qmicode/sessions/`；GUI 的会话选择器会列出该目录中的会话，已有 CLI 也可以加载这些文件。

工具操作实际系统，不提供沙箱。加载可信 MCP 配置时会沿用已有实现启动相应的本地服务。

## 目录

```text
gui/
├── gui.go                  窗口、任务状态、保存和退出流程
├── appearance.go           字体主题、设置界面及 config_gui.yaml 持久化
├── conversation.go         流式显示与工具确认
├── i18n.go                 中英文文案表、错误与状态文案本地化
├── controller/controller.go GUI 内部对话控制逻辑
├── process.go              GUI 退出时清理自身子进程树
├── process_unix.go         Unix 进程列表读取
├── process_windows.go      Windows 进程列表读取
└── README.md
```

界面文案集中在 `i18n.go` 的 `englishText`，以中文原文为 key；下层（`controller`、字体与设置加载）返回的中文错误和状态由 `messageRules` 按整句匹配翻译，两处新增文案时都需要同步补表。`i18n_test.go` 会检查译文表是否重复、`%` 占位符是否与中文一致、每条译文是否有调用点，以及 controller 可能产生的文案是否都能翻译。

当前 GUI 使用项目的 OpenAI 兼容 Runtime；ChatGPT OAuth 模式、图片附件、Markdown 渲染、压缩和撤销操作尚未集成。
