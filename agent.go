package jpf

import (
	"errors"
	"iter"
)

// Agent[state, action] defines some agentic behaviour.
// It does not contain any state or models itself, only configuration.
// Abstractly, an agent is somthing that can pick a next action from the given state, then apply that action to its state.
type Agent[T, U any] interface {
	// Function is the functin that is called to generate the next action of the agent.
	Function[T, U]
	// Handle integrates the given action into the state.
	// It returns a new state, a boolean that is tru if that action was terminal, and a terminal error (if any).
	Handle(T, U) (T, bool, error)
}

// AgentStep[state, action] defines a step the agent has taken this iteration.
type AgentStep[T, U any] struct {
	// State is the newest state of the agent (after taking the action).
	State T
	// Action is the most recent action to be taken.
	Action U
	// Error is only populated when somthing unrecoverable happened (stopping iteration).
	Error error
}

// RunAgent[state, action] will run the agent indefinitely with the model, starting with the inital state.
func RunAgent[T, U any](model Model, agent Agent[T, U], initialState T) iter.Seq[AgentStep[T, U]] {
	state := initialState
	return func(yield func(AgentStep[T, U]) bool) {
		for {
			nextAction, _, err := RunOneShot(model, agent, state)
			if err != nil {
				yield(AgentStep[T, U]{*new(T), *new(U), errors.Join(errors.New("failed to get next action"), err)})
				return
			}
			nextState, terminal, err := agent.Handle(state, nextAction)
			if err != nil {
				yield(AgentStep[T, U]{*new(T), *new(U), errors.Join(errors.New("failed to apply next action"), err)})
				return
			}
			if !yield(AgentStep[T, U]{nextState, nextAction, nil}) {
				return
			}
			if terminal {
				return
			}
			state = nextState
		}
	}
}
