# QmiCode 架构

QmiCode分为如下几个正交的、互相独立的概念：

- Agent（代理）：定义系统提示词、压缩提示词和工具。另外，默认安装的技能系统以Agent为单位；
- AgentRuntime（Agent运行时）：定义Agent的API请求
- Session（对话）：定义大模型的对话。但系统提示词以Agent为准；
- ModelConfig（模型配置）：模型的配置。包括BaseURL、API KEY等。
