package jpf

import (
	"errors"
	"iter"
)

// Action defines an action that an agent has produced.
// An action may do anything, but usually it updates the agent's state.
type Action interface {
	// DoAction runs the action, updates the agent state.
	DoAction() error
}

// Agent defines a stateful agent, capable of generating next actions with no inputs.
type Agent interface {
	// BuildInputMessages builds the input messages to be sent to the LLM, given the agents current state.
	BuildInputMessages() ([]Message, error)
	// ParseResponseText converts the raw text output of an agent step into the next action.
	ParseResponseText(string) (Action, error)
}

var _ Function[Agent, Action] = &agentFunction{}

// agentFunction wraps an agent so we can call a single step as if it were a function.
type agentFunction struct {
	Agent
}

func (f *agentFunction) BuildInputMessages(a Agent) ([]Message, error) {
	return a.BuildInputMessages()
}

// AgentStep defines a step the agent has taken this iteration.
type AgentStep struct {
	// Action is the most recent action to be taken.
	Action Action
	// Error is only populated when somthing unrecoverable happened (stopping iteration).
	Error error
}

// RunAgent will run the agent indefinitely with the model, starting with the inital state.
func RunAgent[T any](model Model, agent Agent) iter.Seq[AgentStep] {
	agentfn := &agentFunction{agent}
	return func(yield func(AgentStep) bool) {
		for {
			nextAction, _, err := RunOneShot(model, agentfn, agent)
			if err != nil {
				yield(AgentStep{nil, errors.Join(errors.New("failed to get next action"), err)})
				return
			}
			err = nextAction.DoAction()
			if err != nil {
				yield(AgentStep{nil, errors.Join(errors.New("failed to apply next action"), err)})
				return
			}
			if !yield(AgentStep{nextAction, nil}) {
				return
			}
		}
	}
}
