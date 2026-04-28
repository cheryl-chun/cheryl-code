# Cheryl Code

一个轻量级的 AI Coding Assistant，用于学习 LLM Agent、Tool Calling 和 TUI 开发。

## 项目结构

```
cheryl-code/
├── cmd/                    # 入口
│   └── main.go
├── internal/
│   ├── config/            # 配置管理（使用 confgen）
│   ├── llm/               # Agent 和 LLM 客户端
│   ├── tools/             # 工具系统
│   ├── messages/          # 消息管理
│   └── tui/               # 交互式界面（bubbletea）
└── configs/               # 配置文件
```

---

## Agent

### 工作原理

Agent 是一个**对话循环**，不断与 LLM 交互直到得到最终答案：

```
用户输入
  ↓
LLM 思考
  ↓
需要调用工具？
  ├─ 是 → 执行工具 → 返回结果 → 继续循环
  └─ 否 → 返回最终答案
```

### 代码实现（`internal/llm/agent.go`）

核心数据结构：
```go
type Agent struct {
    client   *Client              // LLM 客户端
    manager  *messages.MessageManager  // 对话历史管理
    registry *tools.ToolRegistry   // 工具注册表
}
```

循环逻辑：
```go
func (a *Agent) Run(ctx context.Context, prompt string) (string, error) {
    a.manager.AddUser(prompt)  // 1. 添加用户消息
    
    for {
        resp := a.client.Chat(ctx, a.manager.GetAll())  // 2. 调用 LLM
        
        if !a.client.HasToolCalls(resp) {
            return resp.Content  // 3. 没有工具调用，返回结果
        }
        
        // 4. 有工具调用，执行所有工具
        for _, toolCall := range resp.ToolCalls {
            result := a.executeTool(toolCall)
            a.manager.AddTool(result, toolCall.ID)
        }
        // 5. 继续循环，让 LLM 看到工具结果
    }
}
```

---

## Tool

### 设计思路

工具系统采用**接口 + 注册表**模式，类似插件系统。

### Tool 接口定义

```go
type Tool interface {
    Name() string         // 工具名称（如 "read_file"）
    Description() string  // 工具描述（给 LLM 看）
    Parameters() any      // JSON Schema 参数定义
    Execute(args map[string]any) (string, error)  // 执行逻辑
}
```

字段解释：
- `Name` + `Description` + `Parameters` → 给 LLM 看的"说明书"
- `Execute` → 实际执行逻辑
- 返回 `string` → LLM 只能理解文本

### 工具注册表

```go
type ToolRegistry struct {
    tools map[string]Tool  // name -> tool 映射
}

func (r *ToolRegistry) Register(tool Tool)
func (r *ToolRegistry) Get(name string) (Tool, bool)
func (r *ToolRegistry) ToOpenAITools() []openai.ChatCompletionToolUnionParam
```

`ToOpenAITools()`：把 Tool 接口转换成 OpenAI API 要求的格式。LLM 根据这个格式理解有哪些工具可用

### 实现一个工具的步骤

以 `read_tool.go` 为例：

1：定义结构体
```go
type ReadTool struct {
    rootPath string  // 工作目录
}
```

2：实现 Tool 接口
```go
func (t *ReadTool) Name() string {
    return "read_file"
}

func (t *ReadTool) Description() string {
    return "Read the content of a file"
}

func (t *ReadTool) Parameters() any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "path": map[string]any{
                "type": "string",
                "description": "File path to read",
            },
        },
        "required": []string{"path"},
    }
}

func (t *ReadTool) Execute(args map[string]any) (string, error) {
    path := args["path"].(string)
    content, err := os.ReadFile(path)
    return string(content), err
}
```

3：注册到 Registry
``` go
registry.Register(NewReadTool("/path/to/root"))
```


## TUI

[Bubbletea](https://github.com/charmbracelet/bubbletea) 是基于 Elm Architecture 的 TUI 框架。

### Elm Architecture 三要素

```
┌─────────────────────────────┐
│  Model（状态）               │
│  - 对话历史                  │
│  - 等待状态                  │
│  - 终端尺寸                  │
└──────────┬──────────────────┘
           │
           ↓
┌─────────────────────────────┐
│  Update（事件处理）          │
│  - 按键事件                  │
│  - LLM 响应                  │
│  - 窗口大小变化              │
└──────────┬──────────────────┘
           │
           ↓
┌─────────────────────────────┐
│  View（UI 渲染）             │
│  - 对话历史区                │
│  - 输入框                    │
│  - 状态栏                    │
└─────────────────────────────┘
```

### Model 定义

```go
type Model struct {
    // UI 组件
    viewport viewport.Model  // 对话历史显示区（可滚动）
    textarea textarea.Model  // 用户输入框
    spinner  spinner.Model   // 等待动画
    
    // 数据
    messages []Message       // 对话历史
    agent    *llm.Agent     // Agent 实例
    
    // 状态
    width   int
    height  int
    waiting bool  // 是否在等待 LLM 响应
    ready   bool  // 是否已初始化
}
```

### Update 处理逻辑

所有交互都是通过消息（Msg）驱动的

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    
    case tea.KeyMsg:  // 按键事件
        if msg.Type == tea.KeyEnter {
            return m.sendMessage()  // 发送消息
        }
        if msg.String() == "ctrl+c" {
            return m, tea.Quit  // 退出
        }
    
    case agentResponseMsg:  // 自定义：Agent 响应
        m.waiting = false
        m.addMessage("assistant", msg.response)
        return m, nil
    
    case tea.WindowSizeMsg:  // 窗口大小变化
        m.width = msg.Width
        m.height = msg.Height
        m.resizeComponents()
    }
    
    // 转发给子组件
    m.textarea, cmd = m.textarea.Update(msg)
    m.viewport, cmd = m.viewport.Update(msg)
    
    return m, tea.Batch(cmds...)
}
```

### 异步操作：Cmd 和 Msg

**问题：** 如何在 Bubbletea 中调用 Agent（耗时操作）？

**答案：** 通过 `Cmd` 和自定义 `Msg`

```go
// 1. 定义自定义消息类型
type agentResponseMsg struct {
    response string
    err      error
}

// 2. 创建 Cmd（异步执行）
func (m Model) callAgent(prompt string) tea.Cmd {
    return func() tea.Msg {
        // 这里在后台线程执行
        response, err := m.agent.Run(ctx, prompt)
        
        // 返回消息，触发 Update
        return agentResponseMsg{
            response: response,
            err:      err,
        }
    }
}

// 3. 在 Update 中处理响应
case agentResponseMsg:
    m.waiting = false
    m.addMessage("assistant", msg.response)
```

工作流程：
```
用户按 Enter
  ↓
Update 返回 callAgent Cmd
  ↓
Bubbletea 在后台执行 callAgent
  ↓
执行完成，返回 agentResponseMsg
  ↓
Update 收到 agentResponseMsg
  ↓
更新界面显示结果
```

### UI 组件（来自 bubbles）

| 组件 | 用途 | 关键方法 |
|------|------|----------|
| `viewport` | 可滚动内容区 | `SetContent()`, `GotoBottom()` |
| `textarea` | 多行输入框 | `Focus()`, `Value()`, `Reset()` |
| `spinner` | 等待动画 | `Tick`, `View()` |

---

## 使用方式

### 配置

```bash
# 复制配置文件
cp configs/config.yaml.example configs/config.yaml

# 编辑配置
vim configs/config.yaml
```

```yaml
llm:
  baseUrl: https://api.openai.com/v1
  apiKey: your-api-key-here
  model: gpt-4o
```

### 运行

```bash
# 构建
make build

# 启动交互式界面
./bin/cheryl-code

# 或直接运行
go run ./cmd
```

### 快捷键

- `Enter` - 发送消息
- `Ctrl+Enter` / `Shift+Enter` - 换行
- `Ctrl+C` - 退出
- `↑` / `↓` - 滚动对话历史

---


## 下一步计划

- [ ] 添加流式响应（SSE）
- [ ] 工具调用可视化（显示正在执行的工具）
- [ ] 对话历史持久化
- [ ] 支持更多工具（git、http 等）
- [ ] 添加配置界面
- [ ] 性能优化（限制循环次数）

---

## 参考资料

- [Bubbletea 官方教程](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [OpenAI Tool Calling](https://platform.openai.com/docs/guides/function-calling)
- [Bubbles 组件库](https://github.com/charmbracelet/bubbles)
- [Lipgloss 样式库](https://github.com/charmbracelet/lipgloss)