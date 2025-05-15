package jpf

import (
	"errors"
	"iter"
)

type Agent[T, U any] interface {
	// Returns the function that this agent can use to create a new action
	Action() RetryFunction[T, U]
	// Given the current state and an action, returns a new state, whether the action was terminal, and an error
	Handle(T, U) (T, bool, error)
}

type AgentStep[T, U any] struct {
	State  T
	Action U
	Error  error
}

func RunAgent[T, U any](model Model, agent Agent[T, U], initialState T, retriesPerAction int) iter.Seq[AgentStep[T, U]] {
	state := initialState
	actionFunc := agent.Action()
	return func(yield func(AgentStep[T, U]) bool) {
		for {
			nextAction, _, err := RunWithRetries(model, actionFunc, retriesPerAction, state)
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
