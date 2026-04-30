package llm

import (
	"encoding/json"
	"fmt"
	"time"
)

type ToolCallStatus string

const (
	ToolStatusProposed        ToolCallStatus = "proposed"
	ToolStatusPendingApproval ToolCallStatus = "pending_approval"
	ToolStatusApproved        ToolCallStatus = "approved"
	ToolStatusRejected        ToolCallStatus = "rejected"
	ToolStatusRunning         ToolCallStatus = "running"
	ToolStatusSuccess         ToolCallStatus = "success"
	ToolStatusError           ToolCallStatus = "error"
)


type ToolCallStatusState interface {
	Status() ToolCallStatus

	// convert to a new status
	TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error)

	// which status is allowed to be transfered
	AllowedTransitions() []ToolCallStatus

	IsTerminal() bool

	Icon() string
	Color() string
}


type StateBehavior interface {
	OnEnter(ctx *ToolCallState) error
	OnExit(ctx *ToolCallState) error
	Tick(ctx *ToolCallState) error
}

type DefaultBehavior struct{}

func (b *DefaultBehavior) OnEnter(ctx *ToolCallState) error  { return nil }
func (b *DefaultBehavior) OnExit(ctx *ToolCallState) error   { return nil }
func (b *DefaultBehavior) Tick(ctx *ToolCallState) error     { return nil }


type StateGraph struct {
	edges     map[ToolCallStatus][]ToolCallStatus
	behaviors map[ToolCallStatus]StateBehavior
}

var toolCallStateMachine *StateGraph

func init() {
	toolCallStateMachine = NewStateGraph().
		AddEdge(ToolStatusProposed, ToolStatusPendingApproval).
		AddEdge(ToolStatusProposed, ToolStatusRunning).
		AddEdge(ToolStatusPendingApproval, ToolStatusApproved).
		AddEdge(ToolStatusPendingApproval, ToolStatusRejected).
		AddEdge(ToolStatusApproved, ToolStatusRunning).
		AddEdge(ToolStatusRunning, ToolStatusSuccess).
		AddEdge(ToolStatusRunning, ToolStatusError)
}

func NewStateGraph() *StateGraph {
	return &StateGraph{
		edges:     make(map[ToolCallStatus][]ToolCallStatus),
		behaviors: make(map[ToolCallStatus]StateBehavior),
	}
}

func (g *StateGraph) AddEdge(from, to ToolCallStatus) *StateGraph {
	g.edges[from] = append(g.edges[from], to)
	return g
}

func (g *StateGraph) SetBehavior(status ToolCallStatus, behavior StateBehavior) *StateGraph {
	g.behaviors[status] = behavior
	return g
}

func (g *StateGraph) CanTransition(from, to ToolCallStatus) bool {
	targets, exists := g.edges[from]
	if !exists {
		return false
	}

	for _, target := range targets {
		if target == to {
			return true
		}
	}
	return false
}

func (g *StateGraph) GetBehavior(status ToolCallStatus) StateBehavior {
	if behavior, exists := g.behaviors[status]; exists {
		return behavior
	}
	return &DefaultBehavior{}
}

// 导出为 Graphviz DOT 格式（可视化）
func (g *StateGraph) ToGraphviz() string {
	result := "digraph ToolCallStateMachine {\n"
	result += "  rankdir=LR;\n"
	result += "  node [shape=box, style=rounded];\n\n"

	result += "  success [style=\"rounded,filled\", fillcolor=lightgreen];\n"
	result += "  error [style=\"rounded,filled\", fillcolor=lightcoral];\n"
	result += "  rejected [style=\"rounded,filled\", fillcolor=lightgray];\n\n"

	for from, targets := range g.edges {
		for _, to := range targets {
			result += fmt.Sprintf("  %s -> %s;\n", from, to)
		}
	}

	result += "}\n"
	return result
}


// Proposed 
type ProposedState struct{}

func (s *ProposedState) Status() ToolCallStatus {
	return ToolStatusProposed
}

func (s *ProposedState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	if !toolCallStateMachine.CanTransition(s.Status(), newStatus) {
		return nil, fmt.Errorf("invalid transition: %s -> %s", s.Status(), newStatus)
	}

	switch newStatus {
	case ToolStatusPendingApproval:
		return &PendingApprovalState{}, nil
	case ToolStatusRunning:
		return &RunningState{}, nil
	default:
		return nil, fmt.Errorf("unexpected transition: %s -> %s", s.Status(), newStatus)
	}
}

func (s *ProposedState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{ToolStatusPendingApproval, ToolStatusRunning}
}

func (s *ProposedState) IsTerminal() bool {
	return false
}

func (s *ProposedState) Icon() string {
	return "📋"
}

func (s *ProposedState) Color() string {
	return "gray"
}

// PendingApproval 状态
type PendingApprovalState struct{}

func (s *PendingApprovalState) Status() ToolCallStatus {
	return ToolStatusPendingApproval
}

func (s *PendingApprovalState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	if !toolCallStateMachine.CanTransition(s.Status(), newStatus) {
		return nil, fmt.Errorf("invalid transition: %s -> %s", s.Status(), newStatus)
	}

	switch newStatus {
	case ToolStatusApproved:
		return &ApprovedState{}, nil
	case ToolStatusRejected:
		return &RejectedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected transition: %s -> %s", s.Status(), newStatus)
	}
}

func (s *PendingApprovalState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{ToolStatusApproved, ToolStatusRejected}
}

func (s *PendingApprovalState) IsTerminal() bool {
	return false
}

func (s *PendingApprovalState) Icon() string {
	return "⚠️"
}

func (s *PendingApprovalState) Color() string {
	return "yellow"
}

// Approved
type ApprovedState struct{}

func (s *ApprovedState) Status() ToolCallStatus {
	return ToolStatusApproved
}

func (s *ApprovedState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	if !toolCallStateMachine.CanTransition(s.Status(), newStatus) {
		return nil, fmt.Errorf("invalid transition: %s -> %s", s.Status(), newStatus)
	}

	switch newStatus {
	case ToolStatusRunning:
		return &RunningState{}, nil
	default:
		return nil, fmt.Errorf("unexpected transition: %s -> %s", s.Status(), newStatus)
	}
}

func (s *ApprovedState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{ToolStatusRunning}
}

func (s *ApprovedState) IsTerminal() bool {
	return false
}

func (s *ApprovedState) Icon() string {
	return "✓"
}

func (s *ApprovedState) Color() string {
	return "green"
}

// Running
type RunningState struct{}

func (s *RunningState) Status() ToolCallStatus {
	return ToolStatusRunning
}

func (s *RunningState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	if !toolCallStateMachine.CanTransition(s.Status(), newStatus) {
		return nil, fmt.Errorf("invalid transition: %s -> %s", s.Status(), newStatus)
	}

	switch newStatus {
	case ToolStatusSuccess:
		return &SuccessState{}, nil
	case ToolStatusError:
		return &ErrorState{}, nil
	default:
		return nil, fmt.Errorf("unexpected transition: %s -> %s", s.Status(), newStatus)
	}
}

func (s *RunningState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{ToolStatusSuccess, ToolStatusError}
}

func (s *RunningState) IsTerminal() bool {
	return false
}

func (s *RunningState) Icon() string {
	return "⚙️"
}

func (s *RunningState) Color() string {
	return "cyan"
}

// Success
type SuccessState struct{}

func (s *SuccessState) Status() ToolCallStatus {
	return ToolStatusSuccess
}

func (s *SuccessState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	return nil, fmt.Errorf("cannot transition from terminal state: %s", s.Status())
}

func (s *SuccessState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{}
}

func (s *SuccessState) IsTerminal() bool {
	return true
}

func (s *SuccessState) Icon() string {
	return "✅"
}

func (s *SuccessState) Color() string {
	return "green"
}

// Error
type ErrorState struct{}

func (s *ErrorState) Status() ToolCallStatus {
	return ToolStatusError
}

func (s *ErrorState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	return nil, fmt.Errorf("cannot transition from terminal state: %s", s.Status())
}

func (s *ErrorState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{}
}

func (s *ErrorState) IsTerminal() bool {
	return true
}

func (s *ErrorState) Icon() string {
	return "❌"
}

func (s *ErrorState) Color() string {
	return "red"
}

// Rejected
type RejectedState struct{}

func (s *RejectedState) Status() ToolCallStatus {
	return ToolStatusRejected
}

func (s *RejectedState) TransitionTo(newStatus ToolCallStatus) (ToolCallStatusState, error) {
	return nil, fmt.Errorf("cannot transition from terminal state: %s", s.Status())
}

func (s *RejectedState) AllowedTransitions() []ToolCallStatus {
	return []ToolCallStatus{}
}

func (s *RejectedState) IsTerminal() bool {
	return true
}

func (s *RejectedState) Icon() string {
	return "✗"
}

func (s *RejectedState) Color() string {
	return "red"
}

type ToolCallState struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"args"`
	ArgsRaw  string                 `json:"args_raw"` // JSON 字符串
	Result   string                 `json:"result"`
	Error    error                  `json:"error,omitempty"`

	state ToolCallStatusState `json:"-"`

	NeedApproval bool      `json:"need_approval"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
}

func NewToolCallState(id, name string, args map[string]interface{}) *ToolCallState {
	argsJSON, _ := json.Marshal(args)

	return &ToolCallState{
		ID:        id,
		Name:      name,
		Args:      args,
		ArgsRaw:   string(argsJSON),
		state:     &ProposedState{}, // 初始状态
		CreatedAt: time.Now(),
	}
}

func (t *ToolCallState) Status() ToolCallStatus {
	return t.state.Status()
}

func (t *ToolCallState) State() ToolCallStatusState {
	return t.state
}

func (t *ToolCallState) Transition(newStatus ToolCallStatus) error {
	// execute old status's OnExit hook
	oldBehavior := toolCallStateMachine.GetBehavior(t.state.Status())
	if err := oldBehavior.OnExit(t); err != nil {
		return fmt.Errorf("OnExit failed: %w", err)
	}

	// transfer status
	newState, err := t.state.TransitionTo(newStatus)
	if err != nil {
		return err
	}

	t.state = newState

	// execute new status's OnEnter hook
	newBehavior := toolCallStateMachine.GetBehavior(newStatus)
	if err := newBehavior.OnEnter(t); err != nil {
		return fmt.Errorf("OnEnter failed: %w", err)
	}

	if t.state.IsTerminal() {
		t.CompletedAt = time.Now()
	}

	return nil
}

func (t *ToolCallState) IsCompleted() bool {
	return t.state.IsTerminal()
}

func (t *ToolCallState) Duration() time.Duration {
	if t.CompletedAt.IsZero() {
		return time.Since(t.CreatedAt)
	}
	return t.CompletedAt.Sub(t.CreatedAt)
}

func (t *ToolCallState) AllowedTransitions() []ToolCallStatus {
	return t.state.AllowedTransitions()
}
