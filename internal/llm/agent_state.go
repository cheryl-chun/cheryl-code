package llm

import (
	"fmt"
)

type AgentStatus string

const (
	AgentIdle            AgentStatus = "idle"             // idle
	AgentThinking        AgentStatus = "thinking"         // thinking
	AgentExecutingTools  AgentStatus = "executing_tools"  // execute
	AgentWaitingApproval AgentStatus = "waiting_approval" // waiting
	AgentDone            AgentStatus = "done"             // done
	AgentError           AgentStatus = "error"            // error
)

type AgentState struct {
	Status           AgentStatus
	ActiveToolCalls  map[string]*ToolCallState
	PendingApprovals []*ToolCallState // need user approve
	CompletedTools   []*ToolCallState // already finished
	TurnCount        int
	MaxTurns         int
	LastError        error
}

func NewAgentState(maxTurns int) *AgentState {
	return &AgentState{
		Status:           AgentIdle,
		ActiveToolCalls:  make(map[string]*ToolCallState),
		PendingApprovals: make([]*ToolCallState, 0),
		CompletedTools:   make([]*ToolCallState, 0),
		MaxTurns:         maxTurns,
	}
}

func (a *AgentState) AddToolCall(tc *ToolCallState) {
	a.ActiveToolCalls[tc.ID] = tc

	// if tool need to approve
	// add tool to pending queue
	if tc.NeedApproval && tc.Status() == ToolStatusApproved {
		a.PendingApprovals = append(a.PendingApprovals, tc)
		a.Status = AgentWaitingApproval
	}
}

func (a *AgentState) GetToolCall(id string) (*ToolCallState, bool) {
	tc, ok := a.ActiveToolCalls[id]
	return tc, ok
}

func (a *AgentState) UpdateToolCallStatus(id string, newStatus ToolCallStatus) error {
	tc, ok := a.ActiveToolCalls[id]
	if !ok {
		return errToolCallNotFound(id)
	}
	if err := tc.Transition(newStatus); err != nil {
		return err
	}

	if tc.IsCompleted() {
		a.CompletedTools = append(a.CompletedTools, tc)
		delete(a.ActiveToolCalls, id)
	}
	return nil
}

func (a *AgentState) ApproveToolCall(id string) error {
	tc, ok := a.ActiveToolCalls[id]
	if !ok {
		return errToolCallNotFound(id)
	}

	if err := tc.Transition(ToolStatusApproved); err != nil {
		return err
	}

	a.deletePendingTool(tc.ID)

	// all tool have approved
	if len(a.PendingApprovals) == 0 {
		a.Status = AgentExecutingTools
	}

	return nil
}

func (a *AgentState) RejectToolCall(id string) error {
	tc, ok := a.ActiveToolCalls[id]
	if !ok {
		return errToolCallNotFound(id)
	}

	if err := tc.Transition(ToolStatusRejected); err != nil {
		return err
	}

	a.deletePendingTool(tc.ID)

	if tc.IsCompleted() {
		a.CompletedTools = append(a.CompletedTools, tc)
		delete(a.ActiveToolCalls, id)
	}

	if len(a.PendingApprovals) == 0 {
		a.Status = AgentExecutingTools
	}

	return nil
}

func (a *AgentState) ApproveAll() error {
	for _, tc := range a.PendingApprovals {
		if err := tc.Transition(ToolStatusApproved); err != nil {
			return err
		}
	}

	a.PendingApprovals = []*ToolCallState{}

	a.Status = AgentExecutingTools

	return nil
}

func (s *AgentState) RejectAll() error {
	for _, tc := range s.PendingApprovals {
		if err := tc.Transition(ToolStatusRejected); err != nil {
			return err
		}

		if tc.IsCompleted() {
			s.CompletedTools = append(s.CompletedTools, tc)
			delete(s.ActiveToolCalls, tc.ID)
		}
	}

	s.PendingApprovals = []*ToolCallState{}
	s.Status = AgentExecutingTools

	return nil
}

func (a *AgentState) HasPendingApprovals() bool {
	return len(a.PendingApprovals) > 0
}

func (a *AgentState) Reset() {
	a.Status = AgentIdle
	a.ActiveToolCalls = make(map[string]*ToolCallState)
	a.PendingApprovals = []*ToolCallState{}
	a.CompletedTools = []*ToolCallState{}
	a.TurnCount = 0
	a.LastError = nil
}

func (a *AgentState) deletePendingTool(id string) {
	for i, tc := range a.PendingApprovals {
		if tc.ID == id {
			a.PendingApprovals = append(a.PendingApprovals[:i], a.PendingApprovals[i+1:]...)
			return
		}
	}
}

func errToolCallNotFound(id string) error {
	return fmt.Errorf("tool call %s is not found", id)
}
