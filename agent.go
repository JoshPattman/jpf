package jpf

import (
	"errors"
	"iter"
)

// Action[state] defines an action that an agent has produced.
// An action is capable of updating the agent's state.
type Action[T any] interface {
	// DoAction updates the agent state, returning the new state, if it was a terminal action
	DoAction(state T) (T, bool, error)
}

// Agent[state] defines some agentic behaviour.
// It does not contain any state or models itself, only configuration.
// Abstractly, an agent is somthing that can pick a next action from the given state.
type Agent[T any] interface {
	// Function is the functin that is called to generate the next action of the agent.
	Function[T, Action[T]]
}

// AgentStep[state, action] defines a step the agent has taken this iteration.
type AgentStep[T any] struct {
	// State is the newest state of the agent (after taking the action).
	State T
	// Action is the most recent action to be taken.
	Action Action[T]
	// Error is only populated when somthing unrecoverable happened (stopping iteration).
	Error error
}

// RunAgent will run the agent indefinitely with the model, starting with the inital state.
func RunAgent[T any](model Model, agent Agent[T], initialState T) iter.Seq[AgentStep[T]] {
	state := initialState
	return func(yield func(AgentStep[T]) bool) {
		for {
			nextAction, _, err := RunOneShot(model, agent, state)
			if err != nil {
				yield(AgentStep[T]{*new(T), nil, errors.Join(errors.New("failed to get next action"), err)})
				return
			}
			nextState, terminal, err := nextAction.DoAction(state)
			if err != nil {
				yield(AgentStep[T]{*new(T), nil, errors.Join(errors.New("failed to apply next action"), err)})
				return
			}
			if !yield(AgentStep[T]{nextState, nextAction, nil}) {
				return
			}
			if terminal {
				return
			}
			state = nextState
		}
	}
}
