# Cheryl Code

一个对标 **Claude Code** 的 AI Coding Assistant，用于学习 LLM Agent、Tool Calling 和 TUI 开发。

## ✨ 核心特性

- ✅ **流式响应**：实时显示 LLM 输出，打字机效果
- ✅ **工具调用可视化**：实时展示工具执行状态（Pending → Running → Success/Error）
- ✅ **智能审批机制**：危险操作（写文件、执行命令）需要用户确认
- ✅ **状态机管理**：使用状态模式 + 策略模式管理工具生命周期
- ✅ **优雅的 TUI**：基于 Bubbletea 的交互式终端界面
- ✅ **可中断执行**：Ctrl+C 停止 Agent，拒绝工具后智能停止
- ✅ **多行输入**：Enter 发送，Shift+Enter 换行

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


阻塞模式：

```go
result, err := agent.Run(ctx, "读取 config.yaml")
fmt.Println(result)  // 等待完成后一次性显示
```

流式模式，实时显示 LLM 输出：

```go
eventCh, err := agent.RunStream(ctx, "读取 config.yaml")

for event := range eventCh {
    switch event.Type {
    case llm.StreamContent:
        fmt.Print(event.Content)  // 打字机效果
    case llm.StreamToolCall:
        fmt.Printf("\n🔧 %s\n", event.ToolName)
    case llm.StreamDone:
        fmt.Println("\n完成")
    }
}
```

### 代码实现（`internal/llm/agent.go`）

**核心数据结构：**
```go
type Agent struct {
    client   *Client              // LLM 客户端
    manager  *messages.MessageManager  // 对话历史管理
    registry *tools.ToolRegistry   // 工具注册表
}
```

**一次性模式：**
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

### 流式输出关键点

#### SSE

OpenAI SDK 使用 `ssestream.Stream` 处理流式响应：

```go
stream := client.Chat.Completions.NewStreaming(ctx, params)

for stream.Next() {
    chunk := stream.Current()  // 获取当前 chunk
    delta := chunk.Choices[0].Delta
    
    if delta.Content != "" {
        fmt.Print(delta.Content)  // 逐字输出
    }
}

if err := stream.Err(); err != nil {
    // 处理错误
}
```

#### ToolCallBuilder

流式 API 中，工具调用可能分多个 chunk 返回：

```
chunk1: {Index: 0, ID: "call_xxx"}
chunk2: {Index: 0, Function.Name: "read_file"}
chunk3: {Index: 0, Function.Arguments: "{\"path"}
chunk4: {Index: 0, Function.Arguments: "\":\"file.txt\"}"}
```

使用 Builder 模式拼接完整的工具调用：

```go
type ToolCallBuilder struct {
    ID        string
    Name      string
    Arguments string  // 逐步拼接
}

func (b *ToolCallBuilder) Append(tc openai.ChatCompletionChunkChoiceDeltaToolCall) {
    if tc.ID != "" {
        b.ID = tc.ID
    }
    if tc.Function.Name != "" {
        b.Name += tc.Function.Name
    }
    if tc.Function.Arguments != "" {
        b.Arguments += tc.Function.Arguments  // 拼接 JSON 片段
    }
}

func (b *ToolCallBuilder) IsComplete() bool {
    return b.ID != "" && b.Name != "" && b.Arguments != ""
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
    RequiresApproval(args map[string]any) bool    // 是否需要用户审批
}
```

字段解释：
- `Name` + `Description` + `Parameters` → 给 LLM 看的"说明书"
- `Execute` → 实际执行逻辑
- `RequiresApproval` → 工具自己声明是否需要审批（解耦设计）
- 返回 `string` → LLM 只能理解文本

### 已实现的工具

| 工具 | 描述 | 需要审批 |
|------|------|----------|
| `read_file` | 读取文件内容 | ❌ 否 |
| `write_file` | 写入文件 | ✅ 是 |
| `bash` | 执行 Shell 命令 | ✅ 是（可根据命令动态判断）|

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


---

## 工具调用状态机

采用 **状态模式 + 策略模式** 管理工具调用的生命周期。

### 状态转换图

```
proposed → pending_approval → approved → running → success
                           ↘ rejected             ↘ error
```

### 核心组件

**ToolCallStatusState 接口（状态模式）**
```go
type ToolCallStatusState interface {
    Status() ToolCallStatus
    TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error)
    AllowedTransitions() []ToolCallStatus
    IsTerminal() bool
    Icon() string  // UI 图标
    Color() string // UI 颜色
}
```

**StateBehavior 接口（策略模式）**
```go
type StateBehavior interface {
    OnEnter(ctx *ToolCallState) error  // 进入状态时的钩子
    OnExit(ctx *ToolCallState) error   // 离开状态时的钩子
    Tick(ctx *ToolCallState) error     // 定期调用（用于进度报告）
}
```

**StateGraph（管理转换规则）**
```go
type StateGraph struct {
    edges     map[ToolCallStatus][]ToolCallStatus  // 有向图
    behaviors map[ToolCallStatus]StateBehavior     // 状态行为
}
```

### 可扩展性

- ✅ 添加新状态：实现 `ToolCallStatusState` 接口
- ✅ 添加新行为：实现 `StateBehavior` 接口
- ✅ 修改转换规则：调用 `StateGraph.AddEdge()`
- ✅ 可视化状态图：`StateGraph.ToGraphviz()` 导出 DOT 格式

---

## TUI

[Bubbletea](https://github.com/charmbracelet/bubbletea) 是基于 Elm Architecture 的 TUI 框架。

### 工具调用可视化

每个工具调用都会实时展示状态：

```
┌─────────────────────────────────────┐
│ ⚙️ write_file [running]             │  ← 黄色边框
│                                     │
│ Args:                               │
│ {                                   │
│   "path": "test.txt",              │
│   "content": "Hello"               │
│ }                                   │
│                                     │
│ ⏱  Duration: 42ms                   │
└─────────────────────────────────────┘
```

### 审批选择器

需要审批的工具会弹出选择器：

```
┌─────────────────────────────────────┐
│      📝 需要创建文件: test.txt       │
│                                     │
│   ✓ Approve        ← 选中（绿色背景）│
│   ✗ Reject                          │
│   ✓✓ Approve All                    │
│   ✗✗ Reject All                     │
└─────────────────────────────────────┘
```

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

**普通模式：**
- `Enter` - 发送消息
- `Shift+Enter` - 换行
- `Ctrl+C` - 退出（Processing 时停止 Agent）
- `↑` / `↓` - 滚动对话历史

**审批模式：**
- `↑` / `↓` - 选择选项
- `Enter` - 确认选择
- `Ctrl+C` - 退出

---


## 已完成功能 ✅

- [x] 基础 Agent（阻塞和流式模式）
- [x] 工具系统（read_file, write_file, bash）
- [x] 工具接口自声明审批需求（解耦设计）
- [x] 流式响应（SSE）+ 打字机效果
- [x] 工具调用状态机（状态模式 + 策略模式）
- [x] 工具调用可视化（实时状态展示）
- [x] 工具审批机制（上下选择器）
- [x] 美观的 TUI 界面（用户消息边框、工具卡片、分割线）
- [x] Ctrl+C 停止 Agent
- [x] Enter 发送，Shift+Enter 换行
- [x] 拒绝工具后智能停止（告知 LLM）

---

## 🚀 下一步计划（对标 Claude Code）

### Phase 1: 基础增强 🔨
- [ ] **对话历史持久化**：每个项目一个会话文件
- [ ] **会话管理**：列表、切换、删除历史会话
- [ ] **更多工具**：
  - [ ] Glob（文件搜索）
  - [ ] Grep（内容搜索）
  - [ ] List（列出目录）
  - [ ] Git 操作（status, diff, commit, push）
- [ ] **错误重试**：工具执行失败时自动重试或询问用户

### Phase 2: 上下文感知 🧠
- [ ] **项目上下文**：自动读取 .gitignore、README、项目结构
- [ ] **智能文件感知**：记住最近编辑的文件
- [ ] **Git 状态集成**：显示当前分支、未提交的更改
- [ ] **配置管理**：支持项目级配置（.cheryl-code/config.yaml）

### Phase 3: 高级编辑 ✏️
- [ ] **智能 Edit 工具**：基于 diff 的代码编辑
- [ ] **多文件编辑**：一次修改多个文件
- [ ] **代码重构**：重命名、提取函数等
- [ ] **预览功能**：编辑前显示 diff 预览

### Phase 4: 用户体验 🎨
- [ ] **主题系统**：支持自定义颜色主题
- [ ] **快捷命令**：`/help`、`/clear`、`/history` 等
- [ ] **自动补全**：工具名称、文件路径自动补全
- [ ] **进度指示**：长时间操作显示进度条

### Phase 5: 高级特性 🚀
- [ ] **多模型支持**：切换不同的 LLM（GPT-4, Claude, 本地模型）
- [ ] **插件系统**：用户自定义工具
- [ ] **Web 界面**：可选的 Web UI（使用 WebSocket）
- [ ] **协作模式**：多用户共享会话

---

## 🎯 当前迭代重点

**优先级 P0**（下一步立即实现）：
1. **Glob 工具**：`glob("**/*.go")` 搜索文件
2. **Grep 工具**：`grep("pattern", "path")` 搜索内容
3. **List 工具**：`list("dir")` 列出目录
4. **对话历史持久化**：保存和加载会话

**优先级 P1**（近期计划）：
- Git 集成（status, diff, commit）
- 智能 Edit 工具（基于 diff）
- 项目上下文感知

**优先级 P2**（中期规划）：
- 主题系统
- 快捷命令
- 多模型支持

---

## 参考资料

- [Bubbletea 官方教程](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [OpenAI Tool Calling](https://platform.openai.com/docs/guides/function-calling)
- [Bubbles 组件库](https://github.com/charmbracelet/bubbles)
- [Lipgloss 样式库](https://github.com/charmbracelet/lipgloss)